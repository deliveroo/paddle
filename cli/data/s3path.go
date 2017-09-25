package data

import (
	"strings"
)

type S3Path struct {
	bucket string
	path string
}

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
