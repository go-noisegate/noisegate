package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorker_StartAndWait(t *testing.T) {
	testCases := []struct {
		// input
		PackagePath string
		Tasks       []string
		// expect
		startErr bool
		waitErr  bool
	}{
		{
			PackagePath: filepath.Join("testdata", "typical"),
			Tasks:       []string{"TestSum"},
			startErr:    false,
			waitErr:     false,
		},
		{
			PackagePath: filepath.Join("testdata", "no_go_test_files"),
			Tasks:       []string{"TestSum"},
			startErr:    false,
			waitErr:     false,
		},
		{
			PackagePath: filepath.Join("testdata", "no_go_files"),
			Tasks:       []string{"TestSum"},
			startErr:    false,
			waitErr:     true,
		},
		{
			PackagePath: filepath.Join("testdata", "build_error"),
			Tasks:       []string{"TestSum"},
			startErr:    false,
			waitErr:     true,
		},
		{
			PackagePath: "/path/to/not/exist/file",
			Tasks:       []string{"TestSum"},
			startErr:    true,
			waitErr:     false,
		},
		{
			PackagePath: filepath.Join("testdata", "build_error"),
			Tasks:       []string{}, // build even if there is no tasks
			startErr:    false,
			waitErr:     true,
		},
	}

	for i, testCase := range testCases {
		w := &worker{
			packagePath: testCase.PackagePath,
			testFuncs:   testCase.Tasks,
		}
		err := w.Start(context.Background())
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
	currDir, _ := os.Getwd()

	var buff strings.Builder
	job := &Job{
		DirPath:       filepath.Join(currDir, "testdata", "typical"),
		GoTestOptions: []string{"-v"},
		writer:        &buff,
	}
	taskSet := &TaskSet{
		Tasks: []*Task{{TestFunction: "TestSum"}},
	}
	w := newWorker(job, taskSet)
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	passed, err := w.Wait()
	if err != nil || !passed {
		t.Fatalf("unexpected result: %v, %v", passed, err)
	}

	if !strings.Contains(buff.String(), "--- PASS: TestSum") {
		t.Errorf("unexpected content: %s", buff.String())
	}
}

func TestWorker_WithBuildTags(t *testing.T) {
	currDir, _ := os.Getwd()

	job := &Job{
		DirPath:       filepath.Join(currDir, "testdata", "buildtags"),
		GoTestOptions: []string{"-tags", "example"},
	}
	taskSet := &TaskSet{
		Tasks: []*Task{{TestFunction: "TestSum"}},
	}
	w := newWorker(job, taskSet)
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	passed, err := w.Wait()
	if err != nil || !passed {
		t.Fatalf("unexpected result: %v, %v", passed, err)
	}
}

func TestWorker_WithRunOptions(t *testing.T) {
	currDir, _ := os.Getwd()

	var buff strings.Builder
	job := &Job{
		DirPath:       filepath.Join(currDir, "testdata", "typical"),
		GoTestOptions: []string{"-run", "TestSum_ErrorCase", "-v"},
		writer:        &buff,
	}
	taskSet := &TaskSet{
		Tasks: []*Task{{TestFunction: "TestSum"}},
	}
	w := newWorker(job, taskSet)
	if err := w.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	passed, err := w.Wait()
	if err != nil || !passed {
		t.Fatalf("unexpected result: %v, %v", passed, err)
	}

	if !strings.Contains(buff.String(), "--- PASS: TestSum_ErrorCase") {
		t.Errorf("unexpected content: %s", buff.String())
	}
}
