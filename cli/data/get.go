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
		fetchPath(viper.GetString("bucket"), args[0], getBranch, getCommitPath, args[1])
	},
}

func init() {
	getCmd.Flags().StringVarP(&getBranch, "branch", "b", "master", "Branch to work on")
	getCmd.Flags().StringVarP(&getCommitPath, "path", "p", "HEAD", "Path to fetch (instead of HEAD)")
}

func fetchPath(bucket string, version string, branch string, path string, destination string) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	if path == "HEAD" {
		svc := s3.New(sess)
		headPath := fmt.Sprintf("%s/%s/HEAD", version, branch)
		fmt.Println(headPath)
		out, err := svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(headPath),
		})
		if err != nil {
			exitErrorf("%v", err)
		}
		buf := new(bytes.Buffer)
		buf.ReadFrom(out.Body)
		path = buf.String()
	} else {
		path = fmt.Sprintf("%s/%s/%s", version, branch, path)
	}
	fmt.Println("Fetching " + path)
	getBucketObjects(sess, bucket, path, destination)
}

func getBucketObjects(sess *session.Session, bucket string, prefix string, dest string) {
	query := &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}
	svc := s3.New(sess)

	truncatedListing := true

	for truncatedListing {
		resp, err := svc.ListObjectsV2(query)

		if err != nil {
			fmt.Println(err.Error())
			return
		}
		getObjectsAll(bucket, resp, svc, prefix, dest)
		query.ContinuationToken = resp.NextContinuationToken
		truncatedListing = *resp.IsTruncated
	}
}

func getObjectsAll(bucket string, bucketObjectsList *s3.ListObjectsV2Output, s3Client *s3.S3, prefix string, dest string) {
	for _, key := range bucketObjectsList.Contents {
		destFilename := *key.Key
		if strings.HasSuffix(*key.Key, "/") {
			fmt.Println("Got a directory")
			continue
		}
		out, err := s3Client.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    key.Key,
		})
		if err != nil {
			exitErrorf("%v", err)
		}
		destFilePath := dest + "/" + strings.TrimPrefix(destFilename, prefix+"/")
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
