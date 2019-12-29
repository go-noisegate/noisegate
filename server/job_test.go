package server

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestNewJob(t *testing.T) {
	importPath := "github.com/ks888/hornet/server/testdata"
	_, filename, _, _ := runtime.Caller(0)
	dirPath := filepath.Join(filepath.Dir(filename), "testdata")
	job, err := NewJob(importPath, dirPath, 1)
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}
	if job.ImportPath != importPath {
		t.Errorf("wrong import path: %s", job.ImportPath)
	}
	if job.ID == 0 {
		t.Errorf("id is 0")
	}
	if job.Status != JobStatusCreated {
		t.Errorf("wrong status: %v", job.Status)
	}
	if job.DependencyDepth != 1 {
		t.Errorf("wrong dependency depth: %v", job.DependencyDepth)
	}
	expectedTasks := []Task{{TestFunction: "TestSum"}, {TestFunction: "TestSum_ErrorCase"}, {TestFunction: "TestSum_Add1"}}
	if len(expectedTasks) != len(job.Tasks) {
		t.Errorf("invalid number of tasks: %d, %#v", len(job.Tasks), job.Tasks)
	}
	for i, actualTask := range job.Tasks {
		if !reflect.DeepEqual(expectedTasks[i], actualTask) {
			t.Errorf("wrong task: %#v", actualTask)
		}
	}
}
