package common

import (
	"crypto/sha1"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const filechunk = 8192

func DirHash(path string) (string, error) {
	fileList := []string{}
	sha1List := []string{}
	filepath.Walk(path, func(p string, f os.FileInfo, err error) error {
		if IsDirectory(p) {
			return nil
		} else {
			fileList = append(fileList, p)
			return nil
		}
	})
	for _, file := range fileList {
		sha, err := FileHash(file)
		if err == nil {
			sha1List = append(sha1List, fmt.Sprintf("%s:%s", file, sha))
		} else {
			return "", err
		}
	}
	files := strings.Join(sha1List, "\n")
	hasher := sha1.New()
	hasher.Write([]byte(files))
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func FileHash(path string) (string, error) {
	// Open the file for reading
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Cannot find file:", os.Args[1])
		return "", err
	}

	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		fmt.Println("Cannot access file:", os.Args[1])
		return "", err
	}

	// Get the filesize
	filesize := info.Size()

	// Calculate the number of blocks
	blocks := uint64(math.Ceil(float64(filesize) / float64(filechunk)))

	hash := sha1.New()

	// Check each block
	for i := uint64(0); i < blocks; i++ {
		// Calculate block size
		blocksize := int(math.Min(filechunk, float64(filesize-int64(i*filechunk))))

		// Make a buffer
		buf := make([]byte, blocksize)

		// Make a buffer
		file.Read(buf)

		// Write to the buffer
		io.WriteString(hash, string(buf))
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
