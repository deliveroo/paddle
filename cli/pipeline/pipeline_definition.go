package pipeline

import (
	"gopkg.in/yaml.v2"
	"log"
	"regexp"
	"strings"
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
	Secrets   []string
}

func parsePipeline(data []byte) *PipelineDefinition {
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

func (p *PipelineDefinitionStep) OverrideVersion(version string, flags *runCmdFlagsStruct) {
	if version != "" {
		p.Version = version

		if flags.OverrideInputs {
			for i := range p.Inputs {
				p.Inputs[i].Version = version
			}
			flags.Env = append(flags.Env, "step_branch:"+flags.StepBranch)
		}
	}
}

func (p *PipelineDefinitionStep) OverrideBranch(branch string, flags *runCmdFlagsStruct) {
	if branch != "" {
		p.Branch = branch

		if flags.OverrideInputs {
			for i := range p.Inputs {
				p.Inputs[i].Branch = branch
			}
			flags.Env = append(flags.Env, "step_branch:"+flags.StepBranch)
		}
	}
}
