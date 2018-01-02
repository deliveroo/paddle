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
