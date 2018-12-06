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
	"io/ioutil"
	"log"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

type runCmdFlagsStruct struct {
	StepName           string
	BucketName         string
	ImageTag           string
	StepBranch         string
	StepVersion        string
	OverrideInputs     bool
	TailLogs           bool
	Secrets            []string
	Env                []string
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
	runCmd.Flags().StringVarP(&runCmdFlags.ImageTag, "tag", "t", "", "Image tag (overrides the one defined in the pipeline)")
	runCmd.Flags().StringVarP(&runCmdFlags.StepBranch, "step-branch", "B", "", "Step branch (overrides the one defined in the pipeline)")
	runCmd.Flags().StringVarP(&runCmdFlags.StepVersion, "step-version", "V", "", "Step version (overrides the one defined in the pipeline)")
	runCmd.Flags().BoolVarP(&runCmdFlags.TailLogs, "logs", "l", true, "Tail logs")
	runCmd.Flags().BoolVarP(&runCmdFlags.OverrideInputs, "override-inputs", "I", false, "Override input version/branch (only makes sense to use with -B or -V)")
	runCmd.Flags().StringSliceVarP(&runCmdFlags.Secrets, "secret", "S", []string{}, "Secret to pull into the environment (in the form ENV_VAR:secret_store:key_name)")
	runCmd.Flags().StringSliceVarP(&runCmdFlags.Env, "env", "e", []string{}, "Environment variables to set (in the form name:value)")
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
		if flags.ImageTag != "" {
			step.OverrideTag(flags.ImageTag)
		}
		if flags.StepBranch != "" {
			step.OverrideBranch(flags.StepBranch, flags.OverrideInputs)
		}
		if flags.StepVersion != "" {
			step.OverrideVersion(flags.StepVersion, flags.OverrideInputs)
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
	podDefinition.parseSecrets(flags.Secrets)
	podDefinition.parseEnv(flags.Env)

	stepPodBuffer := podDefinition.compile()
	pod := &v1.Pod{}
	yaml.NewYAMLOrJSONDecoder(stepPodBuffer, 4096).Decode(pod)
	pods := clientset.CoreV1().Pods(pipeline.Namespace)

	err := deleteAndWait(clientset, podDefinition, flags)
	if err != nil {
		return err
	}

	if podDefinition.needsVolume() {
		err := createVolumeClaim(clientset, podDefinition, flags)
		if err != nil {
			return err
		}
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

	removed := map[string]bool{}

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
				if !removed[e.Container] {
					log.Printf("[paddle] Container removed: %s", e.Container)
				}
				removed[e.Container] = true
				continue
			case Completed:
				log.Printf("[paddle] Pod execution completed")
				if podDefinition.needsVolume() {
					deleteVolumeClaim(clientset, podDefinition, flags)
				}
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
			pod, _ := pods.Get(podDefinition.PodName, metav1.GetOptions{})
			reason := "Timed out waiting for pod to start. Cluster might not have sufficient resources."
			if pod != nil {
				for _, container := range pod.Status.ContainerStatuses {
					if container.State.Waiting != nil {
						reason = container.State.Waiting.Message
					}
				}
			}
			pods.Delete(podDefinition.PodName, &metav1.DeleteOptions{})
			return errors.New(reason)
		}
	}

	log.Printf("[paddle] Finishing pod execution")
	return nil
}

func deleteAndWait(c kubernetes.Interface, podDefinition *PodDefinition, flags *runCmdFlagsStruct) error {
	pods := clientset.CoreV1().Pods(podDefinition.Namespace)
	deleting := false
	var gracePeriod int64
	opts := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}
	err := wait.PollImmediate(flags.DeletePollInterval, deleteTimeout, func() (bool, error) {
		var err error
		err = pods.Delete(podDefinition.PodName, &opts)
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
		if !deleting {
			log.Printf("[paddle] deleting pod %s", podDefinition.PodName)
			deleting = true
		}
		return false, nil
	})
	return err
}

func createVolumeClaim(c kubernetes.Interface, podDefinition *PodDefinition, flags *runCmdFlagsStruct) error {
	err := deleteVolumeClaim(c, podDefinition, flags)
	if err != nil {
		log.Printf("[paddle] Unable to delete volume claim: %s. Attempting to continue.")
	}

	log.Printf("[paddle] Creating volume claim for %s", podDefinition.PodName)
	claim := &v1.PersistentVolumeClaim{}
	claimBuffer := podDefinition.compileVolumeClaim()
	yaml.NewYAMLOrJSONDecoder(claimBuffer, 4096).Decode(claim)

	claims := clientset.CoreV1().PersistentVolumeClaims(podDefinition.Namespace)

	_, err = claims.Create(claim)
	if err != nil {
		return err
	}
	log.Printf("[paddle] Created volume claim %s", claim.Name)
	return nil
}

func deleteVolumeClaim(c kubernetes.Interface, podDefinition *PodDefinition, flags *runCmdFlagsStruct) error {
	claim := &v1.PersistentVolumeClaim{}
	claimBuffer := podDefinition.compileVolumeClaim()
	yaml.NewYAMLOrJSONDecoder(claimBuffer, 4096).Decode(claim)

	claims := clientset.CoreV1().PersistentVolumeClaims(podDefinition.Namespace)

	deleting := false
	var gracePeriod int64
	opts := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}
	err := wait.PollImmediate(flags.DeletePollInterval, deleteTimeout, func() (bool, error) {
		var err error
		err = claims.Delete(claim.Name, &opts)
		if err != nil {
			if k8errors.IsNotFound(err) {
				if deleting {
					log.Printf("[paddle] Deleted volume claim %s", claim.Name)
				}
				return true, nil
			} else if k8errors.IsForbidden(err) {
				return true, nil // k8s is returning forbidden if the claim does not exists...
			} else {
				return true, err
			}
		}
		if !deleting {
			log.Printf("[paddle] Deleting volume claim %s", claim.Name)
			deleting = true
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	return nil
}
