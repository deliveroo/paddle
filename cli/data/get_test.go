package data

import (
	"errors"
	"github.com/aws/aws-sdk-go/service/s3"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"time"
)

func TestFilterObjects(t *testing.T) {
	var (
		key1   = "path/file1.csv"
		key2   = "path/file2.csv"
		key3   = "path/folder/file3.csv"
		obj1   = &s3.Object{Key: &key1}
		obj2   = &s3.Object{Key: &key2}
		obj3   = &s3.Object{Key: &key3}
		keys   = []string{"file1.csv", "file2.csv", "folder/file3.csv"}
		s3Path = S3Path{bucket: "bucket", path: "path/"}
	)

	result, err := filterObjects(s3Path, []*s3.Object{obj1, obj2, obj3}, keys)
	if err != nil {
		t.Errorf("It should filter objects properly, but %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Failed to filter keys got: %v, want: 3", len(result))
	}
}

func TestFilterObjectsWithNoKeys(t *testing.T) {
	var (
		key    = "path/file.csv"
		obj    = &s3.Object{Key: &key}
		s3Path = S3Path{bucket: "bucket", path: "path/"}
	)

	result, err := filterObjects(s3Path, []*s3.Object{obj}, []string{})
	if err != nil {
		t.Errorf("It should filter objects properly, but %v", err)
	}

	length := len(result)
	if length != 1 {
		t.Errorf("It should return all objects, but got: %v, want: 1.", length)
	}
}

func TestFilterObjectsUsingNonExistentKeys(t *testing.T) {
	var (
		key    = "path/f1.csv"
		obj    = &s3.Object{Key: &key}
		s3Path = S3Path{bucket: "bucket", path: "path/"}
		keys   = []string{"f2.csv", "f3.csv"}
	)

	result, err := filterObjects(s3Path, []*s3.Object{obj}, keys)
	if result != nil {
		t.Error("It should not return a list of S3 objects")
	}

	if err == nil {
		t.Error("It should return an error")
	}
}

type s3GetterFromString struct {
	s string
}

func (s3FromString s3GetterFromString) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	out := s3.GetObjectOutput{
		Body: ioutil.NopCloser(strings.NewReader(s3FromString.s)),
	}
	return &out, nil
}

func Test_copyS3ObjectToFile_worksFirstTime(t *testing.T) {
	var s3Client S3Getter = s3GetterFromString{"foobar"}

	s3Path := S3Path{bucket: "bucket", path: "path/"}
	filePath := "foo/bar"
	tempFile, _ := ioutil.TempFile("", "testDownload")

	err := copyS3ObjectToFile(s3Client, s3Path, filePath, tempFile)
	if err != nil {
		t.Errorf("Should have downloaded file successfully but didn't: %v", err)
	}

	bytes, err := ioutil.ReadFile(tempFile.Name())
	if err != nil {
		t.Errorf("Should be able to read from 'downloaded' file but couldn't %v", err)
	}

	if string(bytes) != "foobar" {
		t.Errorf("File contents were incorrect.  Expected '%s' but got '%s'", "foobar", string(bytes))
	}
}

type s3FailingGetter struct {
}

func (s3FailingGetter *s3FailingGetter) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	return nil, errors.New("can't connect to S3")
}

func Test_copyS3ObjectToFile_failsToGetObjectFromS3(t *testing.T) {
	var s3Client S3Getter = &s3FailingGetter{}
	s3RetriesSleep = 1 * time.Second

	s3Path := S3Path{bucket: "bucket", path: "path/"}
	filePath := "foo/bar"
	tempFile, _ := ioutil.TempFile("", "testDownload")

	err := copyS3ObjectToFile(s3Client, s3Path, filePath, tempFile)
	if err == nil {
		t.Errorf("Shouldn't have been able to download file successfully but did")
	}
}

type s3FailingReader struct {
}

func (s3FailingReader *s3FailingReader) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	out := s3.GetObjectOutput{
		Body: ioutil.NopCloser(&failingReader{}),
	}
	return &out, nil
}

type failingReader struct {
}

func (r *failingReader) Read(p []byte) (int, error) {
	return 0, errors.New("failing reader")
}

func Test_copyS3ObjectToFile_failsToReadFromS3(t *testing.T) {
	var s3Client S3Getter = &s3FailingReader{}
	s3RetriesSleep = 1 * time.Second

	s3Path := S3Path{bucket: "bucket", path: "path/"}
	filePath := "foo/bar"
	tempFile, _ := ioutil.TempFile("", "testDownload")

	err := copyS3ObjectToFile(s3Client, s3Path, filePath, tempFile)
	if err == nil {
		t.Errorf("Shouldn't have been able to download file successfully but did")
	}
}

type s3GetterFailOnClose struct {
	s string
}

func (s3GetterFailOnClose *s3GetterFailOnClose) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	out := s3.GetObjectOutput{
		Body: failOnClose{strings.NewReader(s3GetterFailOnClose.s)},
	}
	return &out, nil
}

type failOnClose struct {
	io.Reader
}

func (failOnClose) Close() error {
	return errors.New("failed while closing")
}

func Test_copyS3ObjectToFile_failsWhenClosingStream(t *testing.T) {
	var s3Client S3Getter = &s3FailingReader{}
	s3RetriesSleep = 1 * time.Second

	s3Path := S3Path{bucket: "bucket", path: "path/"}
	filePath := "foo/bar"
	tempFile, _ := ioutil.TempFile("", "testDownload")

	err := copyS3ObjectToFile(s3Client, s3Path, filePath, tempFile)
	if err == nil {
		t.Errorf("Shouldn't have been able to download file successfully but did")
	}
}

type s3GetterFailsFirstFewAttempts struct {
	unsuccessfulReads int
	s                 string
}

func (s3GetterFailsFirstFewAttempts *s3GetterFailsFirstFewAttempts) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	var out s3.GetObjectOutput
	if s3GetterFailsFirstFewAttempts.unsuccessfulReads == 0 {
		out = s3.GetObjectOutput{
			Body: ioutil.NopCloser(strings.NewReader(s3GetterFailsFirstFewAttempts.s)),
		}
	} else {
		s3GetterFailsFirstFewAttempts.unsuccessfulReads--
		out = s3.GetObjectOutput{
			Body: ioutil.NopCloser(&failingReader{}),
		}
	}

	return &out, nil
}

func Test_copyS3ObjectToFile_failsFirstFewReadAttemptsButRetries(t *testing.T) {
	var s3Client S3Getter = &s3GetterFailsFirstFewAttempts{5, "foobar"}
	s3RetriesSleep = 1 * time.Second

	s3Path := S3Path{bucket: "bucket", path: "path/"}
	filePath := "foo/bar"
	tempFile, _ := ioutil.TempFile("", "testDownload")

	err := copyS3ObjectToFile(s3Client, s3Path, filePath, tempFile)
	if err != nil {
		t.Errorf("Should have downloaded file successfully but didn't: %v", err)
	}

	bytes, err := ioutil.ReadFile(tempFile.Name())
	if err != nil {
		t.Errorf("Should be able to read from 'downloaded' file but couldn't %v", err)
	}

	if string(bytes) != "foobar" {
		t.Errorf("File contents were incorrect.  Expected '%s' but got '%s'", "foobar", string(bytes))
	}
}
