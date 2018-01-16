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
	"context"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"log"
	"strings"
	"time"
)

type runCmdFlagsStruct struct {
	StepName           string
	BucketName         string
	TailLogs           bool
	Secrets            []string
	DeletePollInterval time.Duration
	StartTimeout       time.Duration
}

const defaultDeletePollInterval = 2 * time.Second
const deleteTimeout = 120 * time.Second
const defaultStartTimeout = 10 * time.Minute

var runCmdFlags *runCmdFlagsStruct
var clientset kubernetes.Interface

var logFatalf = log.Fatalf

var runCmd = &cobra.Command{
	Use:   "run [pipeline_yaml]",
	Short: "Run a pipeline or a pipeline step",
	Args:  cobra.ExactArgs(1),
	Long: `Run a pipeline (or a single step) on the Kubernetes cluster.

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
	runCmd.Flags().StringVarP(&runCmdFlags.BucketName, "bucket", "b", "", "Bucket name")
	runCmd.Flags().BoolVarP(&runCmdFlags.TailLogs, "logs", "l", true, "Tail logs")
	runCmd.Flags().StringSliceVarP(&runCmdFlags.Secrets, "secret", "S", []string{}, "Secret to pull into the environment (in the form ENV_VAR:secret_store:key_name)")
	runCmdFlags.DeletePollInterval = defaultDeletePollInterval
	runCmdFlags.StartTimeout = defaultStartTimeout

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
	if flags.BucketName != "" {
		pipeline.Bucket = flags.BucketName
	}

	for _, step := range pipeline.Steps {
		if flags.StepName != "" && step.Step != flags.StepName {
			continue
		}
		err = runPipelineStep(pipeline, &step, flags)
		if err != nil {
			logFatalf("[paddle] %s", err.Error())
		}
	}
}

func runPipelineStep(pipeline *PipelineDefinition, step *PipelineDefinitionStep, flags *runCmdFlagsStruct) error {
	log.Printf("[paddle] Running step %s", step.Step)
	podDefinition := NewPodDefinition(pipeline, step)
	for _, secret := range flags.Secrets {
		secretParts := strings.Split(secret, ":")

		podDefinition.Secrets = append(podDefinition.Secrets, PodSecret{
			Name:  secretParts[0],
			Store: secretParts[1],
			Key:   secretParts[2],
		})
	}

	stepPodBuffer := podDefinition.compile()
	pod := &v1.Pod{}
	yaml.NewYAMLOrJSONDecoder(stepPodBuffer, 4096).Decode(pod)
	pods := clientset.CoreV1().Pods(pipeline.Namespace)

	err := deleteAndWait(clientset, podDefinition, flags)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watch, err := Watch(ctx, clientset, pod)
	if err != nil {
		return err
	}

	pod, err = pods.Create(pod)
	if err != nil {
		return err
	}

	containers := make(map[string]bool)

	go func() {
		time.Sleep(flags.StartTimeout)
		if len(containers) < len(pod.Spec.Containers) {
			cancel()
		}
	}()

	for {
		select {
		case e := <-watch:
			switch e.Type {
			case Added:
				log.Printf("[paddle] Container %s/%s starting", pod.Name, e.Container)
				containers[e.Container] = true
				if flags.TailLogs {
					TailLogs(ctx, clientset, e.Pod, e.Container)
				}
			case Deleted:
				log.Println("[paddle] Pod deleted")
				return errors.New("Pod was deleted unexpectedly.")
			case Removed:
				log.Printf("[paddle] Container removed: %s", e.Container)
				continue
			case Completed:
				log.Printf("[paddle] Pod execution completed")
				return nil
			case Failed:
				var msg string
				if e.Container != "" {
					if e.Message != "" {
						msg = fmt.Sprintf("Container %s/%s failed: '%s'", pod.Name, e.Container, e.Message)
					} else {
						msg = fmt.Sprintf("Container %s/%s failed", pod.Name, e.Container)
					}
					_, present := containers[e.Container]
					if !present && flags.TailLogs { // container died before being added
						TailLogs(ctx, clientset, e.Pod, e.Container)
						time.Sleep(3 * time.Second) // give it time to tail logs
					}
				} else {
					msg = "Pod failed"
				}
				return errors.New(msg)
			}
		case <-ctx.Done():
			pods.Delete(podDefinition.PodName, &metav1.DeleteOptions{})
			return errors.New("Timeout waiting for pod to start. Cluster might not have sufficient resources.")
		}
	}

	log.Printf("[paddle] Finishing pod execution")
	return nil
}

func deleteAndWait(c kubernetes.Interface, podDefinition *PodDefinition, flags *runCmdFlagsStruct) error {
	pods := clientset.CoreV1().Pods(podDefinition.Namespace)
	deleting := false
	err := wait.PollImmediate(flags.DeletePollInterval, deleteTimeout, func() (bool, error) {
		var err error
		err = pods.Delete(podDefinition.PodName, &metav1.DeleteOptions{})
		if err != nil {
			if k8errors.IsNotFound(err) {
				if deleting {
					log.Printf("[paddle] deleted pod %s", podDefinition.PodName)
				}
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
