package data

import (
	"github.com/spf13/afero"
	"reflect"
	"strings"
	"testing"
)

func TestFilesToKeys(t *testing.T) {
	AppFs = afero.NewMemMapFs()
	AppFs.MkdirAll("src/a", 0755)
	afero.WriteFile(AppFs, "src/a/b", []byte("file c"), 0644)
	afero.WriteFile(AppFs, "src/c", []byte("file c"), 0644)

	list := filesToKeys("src")
	expectation := []string{
		"src/a/b",
		"src/c",
	}

	if !reflect.DeepEqual(list, expectation) {
		t.Errorf("list is different got: %s, want: %s.", strings.Join(list, ","), strings.Join(expectation, ","))
	}
}

func TestFilesToKeysWhenEmptyFolder(t *testing.T) {
	AppFs = afero.NewMemMapFs()
	AppFs.MkdirAll("src", 0755)

	list := filesToKeys("src")

	if len(list) != 0 {
		t.Errorf("expecting empty list but got: %s", strings.Join(list, ","))
	}
}
