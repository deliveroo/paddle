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
	pipeline := parsePipeline(data)

	if len(pipeline.Steps) != 2 {
		t.Errorf("excepted two steps, got %i", len(pipeline.Steps))
	}

	if pipeline.Bucket != "canoe-sample-pipeline" {
		t.Errorf("Expected bucket to be canoe-sample-pipeline, got %s", pipeline.Bucket)
	}
}

func TestOverrideTag(t *testing.T) {
	data, err := ioutil.ReadFile("test/sample_steps_passing.yml")
	if err != nil {
		panic(err.Error())
	}
	pipeline := parsePipeline(data)

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
	pipeline := parsePipeline(data)

	pipeline.Steps[0].OverrideVersion("")

	if pipeline.Steps[0].Version != "version1" {
		t.Errorf("Version is %s", pipeline.Steps[0].Version)
	}

	pipeline.Steps[0].OverrideVersion("foo")

	if pipeline.Steps[0].Version != "foo" {
		t.Errorf("Version is %s", pipeline.Steps[0].Version)
	}
}

func TestOverrideBranch(t *testing.T) {
	data, err := ioutil.ReadFile("test/sample_steps_passing.yml")
	if err != nil {
		panic(err.Error())
	}
	pipeline := parsePipeline(data)

	pipeline.Steps[0].OverrideBranch("")

	if pipeline.Steps[0].Branch != "master" {
		t.Errorf("Branch is %s", pipeline.Steps[0].Branch)
	}

	pipeline.Steps[0].OverrideBranch("foo")

	if pipeline.Steps[0].Branch != "foo" {
		t.Errorf("Branch is %s", pipeline.Steps[0].Branch)
	}
}
