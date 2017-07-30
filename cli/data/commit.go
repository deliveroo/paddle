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
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/deliveroo/paddle/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var commitBranch string

var commitCmd = &cobra.Command{
	Use:   "commit [source path] [version]",
	Short: "Commit data to S3",
	Args:  cobra.ExactArgs(2),
	Long: `Store data into S3 under a versioned path, and update HEAD.

Example:

$ paddle data commit -b experimental source/path trained-model/version1
`,
	Run: func(cmd *cobra.Command, args []string) {
		if !viper.IsSet("bucket") {
			exitErrorf("Bucket not defined. Please define 'bucket' in your config file.")
		}
		commitPath(args[0], viper.GetString("bucket"), args[1], commitBranch)
	},
}

func init() {
	commitCmd.Flags().StringVarP(&commitBranch, "branch", "b", "master", "Branch to work on")
}

func commitPath(path string, bucket string, version string, branch string) {
	fd, err := os.Stat(path)
	if err != nil {
		exitErrorf("Path %v not found", path)
	}
	if !fd.Mode().IsDir() {
		exitErrorf("Path %v must be a directory", path)
	}

	hash, err := common.DirHash(path)
	if err != nil {
		exitErrorf("Unable to hash input folder")
	}

	t := time.Now().UTC()

	datePath := fmt.Sprintf("%d/%02d/%02d/%02d%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute())

	destPath := fmt.Sprintf("%s/%s/%s_%s", version, branch, datePath, hash)

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	fileList := []string{}
	filepath.Walk(path, func(p string, f os.FileInfo, err error) error {
		if common.IsDirectory(p) {
			return nil
		} else {
			fileList = append(fileList, p)
			return nil
		}
	})

	uploader := s3manager.NewUploader(sess)

	for _, file := range fileList {
		key := destPath + "/" + strings.TrimPrefix(file, path+"/")
		fmt.Println(file + " -> " + key)
		uploadFileToS3(uploader, bucket, key, file)
	}

	// Update HEAD

	headFile := fmt.Sprintf("%s/%s/HEAD", version, branch)

	uploadDataToS3(sess, destPath, bucket, headFile)
}

func uploadFileToS3(uploader *s3manager.Uploader, bucketName string, key string, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Failed to open file", file, err)
		os.Exit(1)
	}
	defer file.Close()

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   file,
	})

	if err != nil {
		exitErrorf("Failed to upload data to %s/%s, %s", bucketName, key, err.Error())
		return
	}
}

func uploadDataToS3(sess *session.Session, data string, bucket string, key string) {
	s3Svc := s3.New(sess)

	_, err := s3Svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader([]byte(data)),
	})

	if err != nil {
		exitErrorf("Unable to update %s", key)
	}
}
