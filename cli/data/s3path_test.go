package data

import "testing"

func TestBasename(t *testing.T) {
	path := S3Path{
		bucket: "foo",
		path: "aaa/bbb/ccc",
	}

	dirname := path.Basename()
	expectation := "ccc"

	if dirname != expectation {
		t.Errorf("Basename was incorrect, got: %s, want: %s.", dirname, expectation)
	}
}

func TestBasenameWithEmptyPath(t *testing.T) {
	path := S3Path{
		bucket: "foo",
		path: "",
	}

	dirname := path.Basename()
	expectation := ""

	if dirname != expectation {
		t.Errorf("Basename was incorrect, got: %s, want: %s.", dirname, expectation)
	}
}

func TestDirname(t *testing.T) {
	path := S3Path{
		bucket: "foo",
		path: "aaa/bbb/ccc",
	}

	dirname := path.Dirname()
	expectation := "aaa/bbb"

	if dirname != expectation {
		t.Errorf("Dirname was incorrect, got: %s, want: %s.", dirname, expectation)
	}
}

func TestDirnameOnBasicPath(t *testing.T) {
	path := S3Path{
		bucket: "foo",
		path: "aaa",
	}

	dirname := path.Dirname()
	expectation := ""

	if dirname != expectation {
		t.Errorf("Dirname was incorrect, got: %s, want: %s.", dirname, expectation)
	}
}
