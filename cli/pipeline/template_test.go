package pipeline

import (
	"io/ioutil"
	"strings"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func TestCompileTemplate(t *testing.T) {
	data, err := ioutil.ReadFile("test/sample_steps_passing.yml")
	if err != nil {
		panic(err.Error())
	}
	pipeline := ParsePipeline(data)

	podDefinition := NewPodDefinition(pipeline, &pipeline.Steps[0])

	stepPodBuffer := podDefinition.compile()

	pod := &v1.Pod{}
	yaml.NewYAMLOrJSONDecoder(stepPodBuffer, 4096).Decode(pod)

	if pod.Name != "sample-steps-passing-version1-step1-master" {
		t.Errorf("Pod name is %s", pod.Name)
	}

	if pod.Spec.Containers[0].Image != pipeline.Steps[0].Image {
		t.Errorf("First image is %s", pod.Spec.Containers[0].Image)
	}
}

func TestSecrets(t *testing.T) {
	data, err := ioutil.ReadFile("test/sample_steps_passing.yml")
	if err != nil {
		panic(err.Error())
	}
	pipeline := ParsePipeline(data)

	podDefinition := NewPodDefinition(pipeline, &pipeline.Steps[0])
	secrets := []string{"ENV_VAR:secret_store:key_name"}
	podDefinition.parseSecrets(secrets)

	stepPodBuffer := podDefinition.compile()

	pod := &v1.Pod{}
	yaml.NewYAMLOrJSONDecoder(stepPodBuffer, 4096).Decode(pod)

	found := false

	for _, value := range pod.Spec.Containers[0].Env {
		if value.Name == "ENV_VAR" {
			secret := value.ValueFrom.SecretKeyRef
			if secret.Key == "key_name" && secret.LocalObjectReference.Name == "secret_store" {
				found = true
			}
		}
	}

	if !found {
		t.Errorf("Did not find secret")
	}
}

func TestEnv(t *testing.T) {
	data, err := ioutil.ReadFile("test/sample_steps_passing.yml")
	if err != nil {
		panic(err.Error())
	}
	pipeline := ParsePipeline(data)

	podDefinition := NewPodDefinition(pipeline, &pipeline.Steps[0])
	env := []string{"ENV_VAR:env_value"}
	podDefinition.parseEnv(env)

	stepPodBuffer := podDefinition.compile()

	pod := &v1.Pod{}
	yaml.NewYAMLOrJSONDecoder(stepPodBuffer, 4096).Decode(pod)

	found := false

	for _, value := range pod.Spec.Containers[0].Env {
		if value.Name == "ENV_VAR" && value.Value == "env_value" {
			found = true
		}
	}

	if !found {
		t.Errorf("Did not find env var")
	}
}

func TestKeys(t *testing.T) {
	data, err := ioutil.ReadFile("test/sample_keys.yml")
	if err != nil {
		panic(err.Error())
	}

	pipeline := ParsePipeline(data)
	podDefinition := NewPodDefinition(pipeline, &pipeline.Steps[0])

	keys := podDefinition.Step.Inputs[0].Keys
	if len(keys) != 3 {
		t.Errorf("Failed to parse keys, got: %v, want: 3.", len(keys))
	}

	stepPodBuffer := podDefinition.compile()

	pod := &v1.Pod{}
	yaml.NewYAMLOrJSONDecoder(stepPodBuffer, 4096).Decode(pod)

	if pod.Name != "sample-keys-version1-step1-master" {
		t.Errorf("Pod name is %s", pod.Name)
	}

	if pod.Spec.Containers[0].Image != pipeline.Steps[0].Image {
		t.Errorf("First image is %s", pod.Spec.Containers[0].Image)
	}

	command := pod.Spec.Containers[1].Command[2]
	if !strings.Contains(command, "--keys file1.json,file2.json,folder/file3.json") {
		t.Errorf("Failed to build paddle get, keys flag is missing")
	}
}
