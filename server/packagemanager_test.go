package server

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ks888/hornet/common/log"
)

func TestPackageManager_Watch(t *testing.T) {
	m := NewPackageManager()
	if err := m.Watch("."); err != nil {
		t.Fatalf("failed to watch pkg: %v", err)
	}

	_, ok := m.Find(".")
	if !ok {
		t.Fatalf("pkg not found")
	}
}

func TestPackage_Prebuild(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata")

	p := Package{path: dirPath}
	ch := make(chan bool)
	numGoRoutines := 5
	numIt := 2
	for i := 0; i < numGoRoutines; i++ {
		go func() {
			for j := 0; j < numIt; j++ {
				err := p.Prebuild()
				ch <- errors.Is(err, context.Canceled)
			}
		}()
	}

	numCanceled := 0
	for i := 0; i < numGoRoutines*numIt; i++ {
		if <-ch {
			numCanceled++
		}
	}
	if numCanceled == 0 {
		t.Errorf("never canceled")
	}
}

func TestPackage_Build(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "hornet")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata")

	artifactPath := filepath.Join(tempDir, "test")
	p := Package{path: dirPath}
	if err := p.Build(artifactPath); err != nil {
		t.Fatalf("failed to build pkg: %v", err)
	}

	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		t.Errorf("artifact not exist: %s", artifactPath)
	}
}

func TestPackage_Build_NoGoFiles(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "no_go_files")

	p := Package{path: dirPath}
	if err := p.Build("/not/exist"); err != errNoGoTestFiles {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPackage_Build_NoGoTestFiles(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "no_go_test_files")

	p := Package{path: dirPath}
	if err := p.Build("/not/exist"); err != errNoGoTestFiles {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPackage_Build_BuildError(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "build_error")

	p := Package{path: dirPath}
	if err := p.Build("/not/exist"); err == nil {
		t.Fatalf("nil error")
	}
}
