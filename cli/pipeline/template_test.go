package pipeline

import (
	"io/ioutil"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"testing"
)

func TestCompileTemplate(t *testing.T) {
	data, err := ioutil.ReadFile("test/sample_steps_passing.yml")
	if err != nil {
		panic(err.Error())
	}
	pipeline := parsePipeline(data)

	podDefinition := NewPodDefinition(pipeline, &pipeline.Steps[0])
	stepPodBuffer := podDefinition.compile()

	pod := &v1.Pod{}
	yaml.NewYAMLOrJSONDecoder(stepPodBuffer, 4096).Decode(pod)

	if pod.Name != "sample-steps-passing-step1-master" {
		t.Errorf("Pod name is %s", pod.Name)
	}

	if pod.Spec.Containers[0].Image != pipeline.Steps[0].Image {
		t.Errorf("First image is %s", pod.Spec.Containers[0].Image)
	}
}
