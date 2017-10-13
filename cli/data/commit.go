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
	"github.com/deliveroo/paddle/rand"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"strings"
	"time"
)

var commitBranch string
var AppFs = afero.NewOsFs()

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

		destination := S3Path{
			bucket: viper.GetString("bucket"),
			path:   fmt.Sprintf("%s/%s", args[1], commitBranch),
		}

		validatePath(args[0])
		commitPath(args[0], destination)
	},
}

func init() {
	commitCmd.Flags().StringVarP(&commitBranch, "branch", "b", "master", "Branch to work on")
}

func validatePath(path string) {
	fd, err := AppFs.Stat(path)
	if err != nil {
		exitErrorf("Source path %v not found", path)
	}
	if !fd.Mode().IsDir() {
		exitErrorf("Source path %v must be a directory", path)
	}
}

func commitPath(path string, destination S3Path) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	rootKey := generateRootKey(destination)
	keys := filesToKeys(path)
	uploader := s3manager.NewUploader(sess)

	for _, file := range keys {
		key := fmt.Sprintf("%s/%s", rootKey, strings.TrimPrefix(file, path+"/"))
		fmt.Println(file + " -> " + key)
		uploadFileToS3(uploader, destination.bucket, key, file)
	}

	// Update HEAD
	headKey := fmt.Sprintf("%s/HEAD", destination.path)
	uploadDataToS3(sess, destination.bucket, headKey, rootKey)
}

func filesToKeys(path string) (keys []string) {
	afero.Walk(AppFs, path, func(p string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}
		keys = append(keys, p)
		return nil
	})
	return keys
}

func generateRootKey(destination S3Path) string {
	t := time.Now().UTC()
	datePath := fmt.Sprintf("%d/%02d/%02d/%02d/%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute())

	return fmt.Sprintf("%s/%s_%s", destination.path, datePath, rand.String(10))
}

func uploadFileToS3(uploader *s3manager.Uploader, bucket string, key string, filePath string) {
	file, err := AppFs.Open(filePath)
	if err != nil {
		exitErrorf("Failed to open file", file, err)
	}
	defer file.Close()

	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})

	if err != nil {
		exitErrorf("Failed to upload data to %s/%s, %s", bucket, key, err.Error())
	}
}

func uploadDataToS3(sess *session.Session, bucket string, key string, data string) {
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
