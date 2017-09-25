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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type S3Target struct {
	bucket string
	prefix string
	path string
}

func (s *S3Target) copy() *S3Target {
	clone := *s
	return &clone
}

func (t *S3Target) fullPath() string {
	return fmt.Sprintf("%s/%s/%s", t.bucket, t.prefix, t.path);
}

var getBranch string
var getCommitPath string

var getCmd = &cobra.Command{
	Use:   "get [version] [destination path]",
	Short: "Fetch data from S3",
	Args:  cobra.ExactArgs(2),
	Long: `Fetch data from a S3 versioned path.

Example:

$ paddle data get -b experimental trained-model/version1 dest/path
`,
	Run: func(cmd *cobra.Command, args []string) {
		if !viper.IsSet("bucket") {
			exitErrorf("Bucket not defined. Please define 'bucket' in your config file.")
		}

		source := S3Target{
			bucket: viper.GetString("bucket"),
			prefix: fmt.Sprintf("%s/%s", args[0], getBranch),
			path: getCommitPath,
		}

		copyPathToDestination(&source, args[1])
	},
}

func init() {
	getCmd.Flags().StringVarP(&getBranch, "branch", "b", "master", "Branch to work on")
	getCmd.Flags().StringVarP(&getCommitPath, "path", "p", "HEAD", "Path to fetch (instead of HEAD)")
}

func copyPathToDestination(source *S3Target, destination string) {
	session := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	/*
	 * HEAD contains the path to latest folder
	 */
	if source.path == "HEAD" {
		source = source.copy()
		source.path = readHEAD(session, source)
	}

	fmt.Println("Copying " + source.fullPath() + " to " + destination)
	copy(session, source, destination)
}

func readHEAD(session *session.Session, source *S3Target) string {
	svc := s3.New(session)
	key := fmt.Sprintf("%s/HEAD", source.prefix)

	out, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(source.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		exitErrorf("%v", err)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(out.Body)
	return buf.String()
}

func copy(session *session.Session, source *S3Target, destination string) {
	query := &s3.ListObjectsV2Input{
		Bucket: aws.String(source.bucket),
		Prefix: aws.String(source.prefix + "/" + source.path),
	}
	svc := s3.New(session)

	truncatedListing := true

	for truncatedListing {
		response, err := svc.ListObjectsV2(query)

		if err != nil {
			fmt.Println(err.Error())
			return
		}
		copyToLocalFiles(svc, response.Contents, source, destination)

		// Check if more results
		query.ContinuationToken = response.NextContinuationToken
		truncatedListing = *response.IsTruncated
	}
}

func copyToLocalFiles(s3Client *s3.S3, objects []*s3.Object, source *S3Target, destination string) {
	for _, key := range objects {
		destFilename := *key.Key
		if strings.HasSuffix(*key.Key, "/") {
			fmt.Println("Got a directory")
			continue
		}
		out, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(source.bucket),
			Key:    key.Key,
		})
		if err != nil {
			exitErrorf("%v", err)
		}
		destFilePath := destination + "/" + strings.TrimPrefix(destFilename, source.prefix + "/")
		err = os.MkdirAll(filepath.Dir(destFilePath), 0777)
		fmt.Print(destFilePath)
		destFile, err := os.Create(destFilePath)
		if err != nil {
			exitErrorf("%v", err)
		}
		bytes, err := io.Copy(destFile, out.Body)
		if err != nil {
			exitErrorf("%v", err)
		}
		fmt.Printf(" -> %d bytes\n", bytes)
		out.Body.Close()
		destFile.Close()
	}
}
