package pipeline

import (
	"log"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

type PipelineDefinitionStep struct {
	Step       string `yaml:"step" json:"step"`
	Version    string `yaml:"version" json:"version"`
	Branch     string `yaml:"branch" json:"branch"`
	Image      string `yaml:"image" json:"image"`
	Parameters string `yaml:"parameters" json:"parameters"`
	Inputs     []struct {
		Step    string   `yaml:"step" json:"step"`
		Version string   `yaml:"version" json:"version"`
		Branch  string   `yaml:"branch" json:"branch"`
		Path    string   `yaml:"path" json:"path"`
		Bucket  string   `yaml:"bucket" json:"bucket"`
		Keys    []string `yaml:"keys" json:"keys"`
		Subdir  string   `yaml:"subdir" json:"subdir"`
	} `yaml:"inputs" json:"inputs"`
	Commands  []string `yaml:"commands" json:"commands"`
	Resources struct {
		CPU     int    `yaml:"cpu" json:"cpu"`
		Memory  string `yaml:"memory" json:"memory"`
		Storage int    `yaml:"storage-mb" json:"storage-mb"`
	} `yaml:"resources" json:"resources"`
}

type PipelineDefinition struct {
	Pipeline  string                   `yaml:"pipeline"`
	Bucket    string                   `yaml:"bucket"`
	Namespace string                   `yaml:"namespace"`
	Steps     []PipelineDefinitionStep `yaml:"steps"`
	Secrets   []string
}

func ParsePipeline(data []byte) *PipelineDefinition {
	pipeline := PipelineDefinition{}

	err := yaml.Unmarshal(data, &pipeline)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	// For compatibility with Ansible executor
	r, _ := regexp.Compile("default\\('(.+)'\\)")
	matches := r.FindStringSubmatch(pipeline.Bucket)
	if matches != nil && matches[1] != "" {
		pipeline.Bucket = matches[1]
	}

	return &pipeline
}

func (p *PipelineDefinitionStep) OverrideTag(tag string) {
	if tag != "" {
		currentParts := strings.Split(p.Image, ":")
		parts := []string{
			currentParts[0],
			tag,
		}
		p.Image = strings.Join(parts, ":")
	}
}

func (p *PipelineDefinitionStep) OverrideVersion(version string, overrideInputs bool) {
	if version != "" {
		p.Version = version

		if overrideInputs {
			for i := range p.Inputs {
				p.Inputs[i].Version = version
			}
		}
	}
}

func (p *PipelineDefinitionStep) OverrideBranch(branch string, overrideInputs bool) {
	if branch != "" {
		p.Branch = branch

		if overrideInputs {
			for i := range p.Inputs {
				p.Inputs[i].Branch = branch
			}
		}
	}
}
