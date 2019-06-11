package data

import (
	"github.com/aws/aws-sdk-go/service/s3"
	"testing"
)

func TestFilterObjects(t *testing.T) {
	var (
		file1 = "file1.csv"
		file2 = "file2.csv"
		obj1  = &s3.Object{Key: &file1}
		obj2  = &s3.Object{Key: &file2}
		files = []string{"file1.csv"}
	)

	result := filterObjects([]*s3.Object{obj1, obj2}, files)

	if len(result) != 1 {
		t.Errorf("Failed to filter files, got: %v, want: %v.", len(result), len(files))
	}
}

func TestFilterObjectsWithEmptyFiles(t *testing.T) {
	var (
		file = "file.csv"
		obj  = &s3.Object{Key: &file}
	)

	result := filterObjects([]*s3.Object{obj}, []string{})

	length := len(result)
	if length != 1 {
		t.Errorf("Failed to filter files, got: %v, want: %v.", length, length)
	}
}
