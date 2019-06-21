package pipeline

import (
	"io/ioutil"
	"testing"
)

func TestParsePipeline(t *testing.T) {
	data, err := ioutil.ReadFile("test/sample_steps_passing.yml")
	if err != nil {
		panic(err.Error())
	}
	pipeline := ParsePipeline(data)

	if len(pipeline.Steps) != 2 {
		t.Errorf("expected two steps, got %d", len(pipeline.Steps))
	}

	if pipeline.Bucket != "canoe-sample-pipeline" {
		t.Errorf("Expected bucket to be canoe-sample-pipeline, got %s", pipeline.Bucket)
	}

	for _, step := range pipeline.Steps {
		if step.Subdir != true {
			t.Errorf("expected all steps 'subdir' to be true, got %t", step.Subdir)
		}
	}
}

func TestOverrideTag(t *testing.T) {
	data, err := ioutil.ReadFile("test/sample_steps_passing.yml")
	if err != nil {
		panic(err.Error())
	}
	pipeline := ParsePipeline(data)

	pipeline.Steps[0].OverrideTag("")

	if pipeline.Steps[0].Image != "219541440308.dkr.ecr.eu-west-1.amazonaws.com/paddlecontainer:latest" {
		t.Errorf("Image is %s", pipeline.Steps[0].Image)
	}

	pipeline.Steps[0].OverrideTag("foo")

	if pipeline.Steps[0].Image != "219541440308.dkr.ecr.eu-west-1.amazonaws.com/paddlecontainer:foo" {
		t.Errorf("Image is %s", pipeline.Steps[0].Image)
	}
}

func TestOverrideVersion(t *testing.T) {
	data, err := ioutil.ReadFile("test/sample_steps_passing.yml")
	if err != nil {
		panic(err.Error())
	}
	pipeline := ParsePipeline(data)

	pipeline.Steps[0].OverrideVersion("", true)

	if pipeline.Steps[0].Version != "version1" {
		t.Errorf("Version is %s", pipeline.Steps[0].Version)
	}

	pipeline.Steps[1].OverrideVersion("foo", true)

	if pipeline.Steps[1].Version != "foo" {
		t.Errorf("Version is %s", pipeline.Steps[1].Version)
	}

	if pipeline.Steps[1].Inputs[0].Version != "foo" {
		t.Errorf("Dependent input Version is %s", pipeline.Steps[1].Inputs[0].Version)
	}
}

func TestOverrideBranch(t *testing.T) {
	data, err := ioutil.ReadFile("test/sample_steps_passing.yml")
	if err != nil {
		panic(err.Error())
	}
	pipeline := ParsePipeline(data)

	pipeline.Steps[0].OverrideBranch("", true)

	if pipeline.Steps[0].Branch != "master" {
		t.Errorf("Branch is %s", pipeline.Steps[0].Branch)
	}

	pipeline.Steps[1].OverrideBranch("foo", true)

	if pipeline.Steps[1].Branch != "foo" {
		t.Errorf("Branch is %s", pipeline.Steps[1].Branch)
	}

	if pipeline.Steps[1].Inputs[0].Branch != "foo" {
		t.Errorf("Dependent input Branch is %s", pipeline.Steps[1].Inputs[0].Branch)
	}
}
