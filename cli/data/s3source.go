package data

import (
	"fmt"
)

type S3Source struct {
	bucket string
	prefix string
	path string
}

func (s *S3Source) copy() *S3Source {
	clone := *s
	return &clone
}

func (t *S3Source) fullPath() string {
	return fmt.Sprintf("%s/%s/%s", t.bucket, t.prefix, t.path);
}
