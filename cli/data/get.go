// Copyright © 2017 RooFoods LTD
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package data

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	getBranch     string
	getCommitPath string
	getBucket     string
	getKeys       []string
	getSubdir     string
)

const (
	s3ParallelGets = 100
	s3Retries      = 10
	s3RetriesSleep = 10 * time.Second
)

var getCmd = &cobra.Command{
	Use:   "get [step/version] [destination path]",
	Short: "Fetch data from S3",
	Args:  cobra.ExactArgs(2),
	Long: `Fetch data from a S3 versioned path.

Example:

$ paddle data get -b experimental --bucket roo-pipeline --subdir version1 trained-model/version1 dest/path
$ paddle data get -b experimental --bucket roo-pipeline --keys file1.csv,file2.csv trained-model/version1 dest/path
`,
	Run: func(cmd *cobra.Command, args []string) {
		if getBucket == "" {
			getBucket = viper.GetString("bucket")
		}
		if getBucket == "" {
			exitErrorf("Bucket not defined. Please define 'bucket' in your config file.")
		}

		source := S3Path{
			bucket: getBucket,
			path:   fmt.Sprintf("%s/%s/%s", args[0], getBranch, getCommitPath),
		}

		copyPathToDestination(source, args[1], getKeys, getSubdir)
	},
}

func init() {
	getCmd.Flags().StringVarP(&getBranch, "branch", "b", "master", "Branch to work on")
	getCmd.Flags().StringVar(&getBucket, "bucket", "", "Bucket to use")
	getCmd.Flags().StringVarP(&getCommitPath, "path", "p", "HEAD", "Path to fetch (instead of HEAD)")
	getCmd.Flags().StringSliceVarP(&getKeys, "keys", "k", []string{}, "A list of keys to download separated by comma")
	getCmd.Flags().StringVarP(&getSubdir, "subdir", "d", "", "Custom subfolder name for export path")
}

func CopyPathToDestinationWithoutS3Path(bucket string, step string, version string, branch string, path string, destination string, files []string, subdir string) {
	source := S3Path{
		bucket: bucket,
		path:   fmt.Sprintf("%s/%s/%s/%s", step, version, branch, path),
	}

	copyPathToDestination(source, destination, files, subdir)
}

func copyPathToDestination(source S3Path, destination string, keys []string, subdir string) {
	session := session.Must(session.NewSessionWithOptions(session.Options{
		Config:            aws.Config{Region: aws.String("eu-west-1")},
		SharedConfigState: session.SharedConfigEnable,
	}))

	/*
	 * HEAD contains the path to latest folder
	 */
	if source.Basename() == "HEAD" {
		latestFolder := readHEAD(session, source)
		source.path = latestFolder
	}
	if !strings.HasSuffix(source.path, "/") {
		source.path += "/"
	}
	if subdir != "" {
		destination = parseDestination(destination, subdir)
	}

	fmt.Println("Copying " + source.path + " to " + destination)
	copy(session, source, destination, keys)
	f, err := os.OpenFile("/data/output/inputs.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	f.WriteString(source.path + "\n")
}

func readHEAD(session *session.Session, source S3Path) string {
	svc := s3.New(session)

	out, err := getObject(svc, aws.String(source.bucket), aws.String(source.path))

	if err != nil {
		exitErrorf("Error reading HEAD: %v", err)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(out.Body)
	return buf.String()
}

func parseDestination(destination string, subdir string) string {
	if !strings.HasSuffix(destination, "/") {
		destination += "/" + subdir
	} else {
		destination += subdir
	}
	return destination
}

func copy(session *session.Session, source S3Path, destination string, keys []string) {
	query := &s3.ListObjectsV2Input{
		Bucket: aws.String(source.bucket),
		Prefix: aws.String(source.path),
	}
	svc := s3.New(session)

	for {
		response, err := svc.ListObjectsV2(query)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		copyToLocalFiles(svc, response.Contents, source, destination, keys)

		// Check if more results
		query.ContinuationToken = response.NextContinuationToken

		if !(*response.IsTruncated) {
			break
		}
	}
}

func copyToLocalFiles(s3Client *s3.S3, objects []*s3.Object, source S3Path, destination string, keys []string) {
	var (
		wg  = new(sync.WaitGroup)
		sem = make(chan struct{}, s3ParallelGets)
	)

	downloadList, err := filterObjects(source, objects, keys)
	if err != nil {
		exitErrorf("Error downloading keys: %v", err)
	}

	wg.Add(len(downloadList))

	for _, key := range downloadList {
		go process(s3Client, source, destination, *key.Key, sem, wg)
	}

	wg.Wait()
}

func filterObjects(source S3Path, objects []*s3.Object, keys []string) ([]*s3.Object, error) {
	var (
		downloadList []*s3.Object
		objsByKey    = make(map[string]*s3.Object)
		keysNotFound []string
	)
	if len(keys) == 0 {
		return objects, nil
	}
	for _, obj := range objects {
		objsByKey[*obj.Key] = obj
	}
	for _, key := range keys {
		if obj, contains := objsByKey[source.path+key]; contains {
			downloadList = append(downloadList, obj)
			continue
		}
		keysNotFound = append(keysNotFound, key)
	}
	if len(keysNotFound) > 0 {
		return nil, errors.New("couldn't find " + strings.Join(keysNotFound, ","))
	}
	return downloadList, nil
}

func process(s3Client *s3.S3, src S3Path, basePath string, filePath string, sem chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	// block if N goroutines are already active (buffer full).
	sem <- struct{}{}

	defer func() {
		// frees up buffer slot
		<-sem
	}()

	if strings.HasSuffix(filePath, "/") {
		fmt.Println("Got a directory")
		return
	}

	out, err := getObject(s3Client, aws.String(src.bucket), &filePath)
	if err != nil {
		exitErrorf("%v", err)
	}
	defer out.Body.Close()

	target := basePath + "/" + strings.TrimPrefix(filePath, src.Dirname()+"/")
	err = store(out, target)
	if err != nil {
		exitErrorf("%v", err)
	}
}

func getObject(s3Client *s3.S3, bucket *string, key *string) (*s3.GetObjectOutput, error) {
	var (
		err error
		out *s3.GetObjectOutput
	)

	retries := s3Retries
	for retries > 0 {
		out, err = s3Client.GetObject(&s3.GetObjectInput{
			Bucket: bucket,
			Key:    key,
		})
		if err == nil {
			return out, nil
		}

		retries--
		if retries > 0 {
			fmt.Printf("Error fetching from S3: %s, (%s); will retry in %v...	\n", *key, err.Error(), s3RetriesSleep)
			time.Sleep(s3RetriesSleep)
		}
	}
	return nil, err
}

func store(obj *s3.GetObjectOutput, destination string) error {
	err := os.MkdirAll(filepath.Dir(destination), 0777)

	file, err := os.Create(destination)
	if err != nil {
		return errors.Wrapf(err, "creating destination %s", destination)
	}
	defer file.Close()

	bytes, err := io.Copy(file, obj.Body)
	if err != nil {
		return errors.Wrapf(err, "copying file %s", destination)
	}

	fmt.Printf("%s -> %d bytes\n", destination, bytes)
	return nil
}
