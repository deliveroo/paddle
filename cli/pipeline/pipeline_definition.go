package pipeline

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"gopkg.in/yaml.v2"
)

type PipelineDefinitionStepInput struct {
	Step    string   `yaml:"step" json:"step"`
	Version string   `yaml:"version" json:"version"`
	Branch  string   `yaml:"branch" json:"branch"`
	Image   string   `yaml:"image" json:"image"`
	Path    string   `yaml:"path" json:"path"`
	Bucket  string   `yaml:"bucket" json:"bucket"`
	Keys    []string `yaml:"keys" json:"keys"`
	Subdir  string   `yaml:"subdir" json:"subdir"`
}

type PipelineDefinitionStep struct {
	Step      string                        `yaml:"step" json:"step"`
	Version   string                        `yaml:"version" json:"version"`
	Branch    string                        `yaml:"branch" json:"branch"`
	Image     string                        `yaml:"image" json:"image"`
	Inputs    []PipelineDefinitionStepInput `yaml:"inputs" json:"inputs"`
	Commands  []string                      `yaml:"commands" json:"commands"`
	Resources struct {
		CPU     int    `yaml:"cpu" json:"cpu"`
		Memory  string `yaml:"memory" json:"memory"`
		Storage int    `yaml:"storage-mb" json:"storage-mb"`
	} `yaml:"resources" json:"resources"`
}

type PipelineDefinition struct {
	GlobalEnv       map[string]map[string]string `yaml:"global_env"`
	JenkinsEnv      map[string]string            `yaml:"jenkins_env"`
	Pipeline        string                       `yaml:"pipeline"`
	Bucket          string                       `yaml:"bucket"`
	Namespace       string                       `yaml:"namespace"`
	EcrPath         string                       `yaml:"ecr_path"`
	Steps           []PipelineDefinitionStep     `yaml:"steps"`
	Secrets         []string
	Env             []string
	CurrentBranch   string
	BucketOverrides []string
	OverrideInputs  bool
	ImageTag        string
	StepBranch      string
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

	pipeline.CurrentBranch = findGitBranch()
	pipeline.parseImageTag()
	pipeline.parseStepBranch()

	fmt.Println("-------------")
	pipeline.parseGlobalEnv()
	fmt.Println("-------------")
	pipeline.parseJenkinsEnv()

	return &pipeline
}

func (p *PipelineDefinition) parseImageTag() {
	// make it lowercase and replace /_ with dashes -
	p.ImageTag = sanitizeName(p.CurrentBranch)
}

func (p *PipelineDefinition) parseStepBranch() {
	p.StepBranch = sanitizeName(p.CurrentBranch)
}

func (p *PipelineDefinition) parseJenkinsEnv() {

}

func (p *PipelineDefinition) parseGlobalEnv() {
	for envName, branchMapping := range p.GlobalEnv {
		for branch, value := range branchMapping {
			if p.CurrentBranch == branch || branch == "other" {
				res := fmt.Sprintf("%s:%s", envName, value)
				switch envName {
				case "replace_buckets":
					p.BucketOverrides = append(p.BucketOverrides, res)
				case "override_inputs":
					p.OverrideInputs = value == "true"
				case "bucket_name":
					p.Bucket = value
				default:
					p.Env = append(p.Env, res)
				}
			}
		}
	}
}

func (p *PipelineDefinitionStep) OverrideTag(tag string) {
	if tag != "" {
		currentParts := strings.Split(p.Image, ":")
		parts := []string{
			currentParts[0],
			sanitizeName(tag),
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
