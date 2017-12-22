package pipeline

import (
	"gopkg.in/yaml.v2"
	"log"
)

type PipelineDefinitionStep struct {
	Step    string `yaml:"step"`
	Version string `yaml:"version"`
	Branch  string `yaml:"branch"`
	Image   string `yaml:"image"`
	Inputs  []struct {
		Step    string `yaml:"step"`
		Version string `yaml:"version"`
		Branch  string `yaml:"branch"`
		Path    string `yaml:"path"`
	} `yaml:"inputs"`
	Commands  []string `yaml:"commands"`
	Resources struct {
		CPU    int    `yaml:"cpu"`
		Memory string `yaml:"memory"`
	} `yaml:"resources"`
}

type PipelineDefinition struct {
	Pipeline  string                   `yaml:"pipeline"`
	Bucket    string                   `yaml:"bucket"`
	Namespace string                   `yaml:"namespace"`
	Steps     []PipelineDefinitionStep `yaml:"steps"`
}

func parsePipeline(data []byte) *PipelineDefinition {
	pipeline := PipelineDefinition{}

	err := yaml.Unmarshal(data, &pipeline)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	return &pipeline
}
