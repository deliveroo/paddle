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
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

var stepName string

var runCmd = &cobra.Command{
	Use:   "run [pipeline_yaml] [-s step_name]",
	Short: "Run a pipeline or a pipeline step",
	Args:  cobra.ExactArgs(1),
	Long: `Store data into S3 under a versioned path, and update HEAD.

Example:

$ paddle pipeline run test_pipeline.yaml
`,
	Run: func(cmd *cobra.Command, args []string) {
		runPipeline(args[0])
	},
}

func init() {
	runCmd.Flags().StringVarP(&stepName, "step", "s", "", "Single step to execute")
}

func runPipeline(path string) {
	config, err := getKubernetesConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	pipeline := parsePipeline(data)
	namespace := pipeline.Namespace
	list, err := clientset.CoreV1().Pods(namespace).List(v1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	fmt.Printf("{}", list)
	for _, step := range pipeline.Steps {
		stepPod := compilePodTemplate(pipeline, &step)
		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, groupVersionKind, err := decode([]byte(stepPod), nil, nil)

		if err != nil {
			log.Fatal(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
		}
	}
}
