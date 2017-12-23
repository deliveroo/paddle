// Copyright © 2017 RooFoods LTD
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pipeline

import (
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"log"
	"time"
)

type runCmdFlagsStruct struct {
	StepName string
}

const defaultPollInterval = 5 * time.Second
const defaultTimeout = 120 * time.Second

var runCmdFlags *runCmdFlagsStruct
var clientset *kubernetes.Clientset

var runCmd = &cobra.Command{
	Use:   "run [pipeline_yaml]",
	Short: "Run a pipeline or a pipeline step",
	Args:  cobra.ExactArgs(1),
	Long: `Store data into S3 under a versioned path, and update HEAD.

Example:

$ paddle pipeline run test_pipeline.yaml
`,
	Run: func(cmd *cobra.Command, args []string) {
		runPipeline(args[0], runCmdFlags)
	},
}

func init() {
	runCmdFlags = &runCmdFlagsStruct{}
	runCmd.Flags().StringVarP(&runCmdFlags.StepName, "step", "s", "", "Single step to execute")

	config, err := getKubernetesConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

func runPipeline(path string, flags *runCmdFlagsStruct) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	pipeline := parsePipeline(data)
	// namespace := pipeline.Namespace
	// list, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	// if err != nil {
	// 	panic(err.Error())
	// }
	// fmt.Printf("{}\n", list)
	for _, step := range pipeline.Steps {
		if flags.StepName != "" && step.Step != flags.StepName {
			continue
		}
		err = runPipelineStep(pipeline, &step, flags)
		if err != nil {
			log.Fatalf("pipeline step failed: %s", err.Error())
		}
	}
	// for _, step := range pipeline.Steps {
	// 	stepPod := compilePodTemplate(pipeline, &step)
	// 	decode := scheme.Codecs.UniversalDeserializer().Decode
	// 	obj, groupVersionKind, err := decode([]byte(stepPod), nil, nil)

	// 	if err != nil {
	// 		log.Fatal(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
	// 	}
	// }
}

func runPipelineStep(pipeline *PipelineDefinition, step *PipelineDefinitionStep, flags *runCmdFlagsStruct) error {
	log.Printf("[paddle] Running step %s", step.Step)
	podDefinition := NewPodDefinition(pipeline, step)
	stepPodBuffer := podDefinition.compile()
	pod := &v1.Pod{}
	yaml.NewYAMLOrJSONDecoder(stepPodBuffer, 4096).Decode(pod)
	pods := clientset.CoreV1().Pods(pipeline.Namespace)

	err := deleteAndWait(clientset, podDefinition)
	if err != nil {
		return err
	}

	pod, err = pods.Create(pod)
	if err != nil {
		return err
	}

	stopWatching := make(chan bool)
	defer close(stopWatching)

	watcher, err := NewPodWatcher(clientset, pod, stopWatching)
	if err != nil {
		return err
	}

	for {
		event, ok := <-watcher
		if !ok {
			stopWatching <- true
			return fmt.Errorf("pod %s channel has been closed ", pod.Name)
		}
		switch event.Object.(type) {
		case *v1.Pod:
			eventPod := event.Object.(*v1.Pod)
			switch event.Type {
			case watch.Added, watch.Modified:
				if eventPod.Status.Phase == v1.PodSucceeded {
					watcher = nil
					break
					log.Printf("Post succeed")

				}
				if eventPod.Status.Phase == v1.PodFailed {
					stopWatching <- true
					return fmt.Errorf("pod failed: '%s'", eventPod.Status.Message)
				}
				for i := 0; i < len(eventPod.Status.ContainerStatuses); i++ {
					containerStatus := eventPod.Status.ContainerStatuses[i]
					term := containerStatus.State.Terminated
					if term != nil && term.ExitCode != 0 {
						return fmt.Errorf("pod container %s exited with error %s", containerStatus.Name, term.Message)
					}
				}
			case watch.Deleted:
				stopWatching <- true
				return fmt.Errorf("pod deleted")
			case watch.Error:
				stopWatching <- true
				return fmt.Errorf("pod error")
			}
		}
		if watcher == nil {
			break
		}
	}

	stopWatching <- true

	err = pods.Delete(podDefinition.PodName, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	log.Printf("[paddle] Finishing pod execution")
	return nil
}

func deleteAndWait(c *kubernetes.Clientset, podDefinition *PodDefinition) error {
	pods := clientset.CoreV1().Pods(podDefinition.Namespace)
	deleting := false
	err := wait.PollImmediate(defaultPollInterval, defaultTimeout, func() (bool, error) {
		var err error
		err = pods.Delete(podDefinition.PodName, &metav1.DeleteOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return true, nil
			} else {
				return true, err
			}
		}
		if deleting {
			log.Print("[paddle] .")
		} else {
			log.Printf("[paddle] deleting pod %s", podDefinition.PodName)
			deleting = true
		}
		return false, nil
	})
	return err
}
