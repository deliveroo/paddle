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
		t.Errorf("excepted two steps, got %d", len(pipeline.Steps))
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
