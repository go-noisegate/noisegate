package server

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorker_StartAndWait(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "noisegate-test")
	if err != nil {
		t.Errorf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		// input
		PackagePath string
		Tasks       []string
		// expect
		startErr bool
		waitErr  bool
	}{
		{
			Tasks:    []string{"TestSum"},
			startErr: false,
			waitErr:  false,
		},
		{
			PackagePath: "/path/to/not/exist/file",
			Tasks:       []string{"TestSum"},
			startErr:    true,
			waitErr:     false,
		},
		{
			Tasks:    []string{},
			startErr: false,
			waitErr:  false,
		},
	}

	for i, testCase := range testCases {
		w := &Worker{
			ctx:         context.Background(),
			packagePath: testCase.PackagePath,
			testFuncs:   testCase.Tasks,
		}
		err := w.Start()
		if (err != nil) != testCase.startErr {
			t.Errorf("[%d] %v", i, err)
		}
		if err != nil {
			continue
		}

		_, err = w.Wait()
		if (err != nil) != testCase.waitErr {
			t.Errorf("[%d] %v", i, err)
		}
	}
}

func TestWorker_CheckOutput(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "noisegate-test")
	if err != nil {
		t.Errorf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	currDir, _ := os.Getwd()

	var buff strings.Builder
	job := &Job{
		DirPath: filepath.Join(currDir, "testdata"),
		writer:  &buff,
	}
	taskSet := &TaskSet{
		Tasks: []*Task{{TestFunction: "TestSum"}},
	}
	w := NewWorker(context.Background(), job, taskSet)
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	passed, err := w.Wait()
	if err != nil || !passed {
		t.Fatalf("unexpected result: %v, %v", passed, err)
	}

	if !strings.Contains(buff.String(), "TestSum") {
		t.Errorf("unexpected content: %s", buff.String())
	}
}

func TestWorker_WithBuildTags(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "noisegate-test")
	if err != nil {
		t.Errorf("failed to create the temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	currDir, _ := os.Getwd()

	job := &Job{
		DirPath:   filepath.Join(currDir, "testdata", "buildtags"),
		BuildTags: "example",
	}
	taskSet := &TaskSet{
		Tasks: []*Task{{TestFunction: "TestSum"}},
	}
	w := NewWorker(context.Background(), job, taskSet)
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}

	passed, err := w.Wait()
	if err != nil || !passed {
		t.Fatalf("unexpected result: %v, %v", passed, err)
	}
}
