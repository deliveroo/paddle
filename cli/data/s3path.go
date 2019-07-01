package data

import (
	"strings"
)

type S3Path struct {
	bucket string
	path   string
}

//func CreateS3Path(input pipeline.PipelineDefinitionStepInput) S3Path {
//return S3Path{
//bucket: input.Bucket,
//path:   fmt.Sprintf("%s/%s/%s/%s/", input.Step, input.Version, input.Branch, input.Path),
//}
//}

func (p *S3Path) Basename() string {
	components := strings.Split(p.path, "/")
	return components[len(components)-1]
}

func (p *S3Path) Dirname() string {
	components := strings.Split(p.path, "/")
	if len(components) == 0 {
		return ""
	}
	return strings.Join(components[:len(components)-1], "/")
}
