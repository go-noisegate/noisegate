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

func TestNewJob_InvalidDirPath(t *testing.T) {
	importPath := "github.com/ks888/hornet/server/testdata"
	dirPath := "/not/exist"
	_, err := NewJob(importPath, dirPath, 1)
	if err == nil {
		t.Fatalf("err should not be nil: %v", err)
	}
}

func TestNewJob_UniqueIDCheck(t *testing.T) {
	importPath := "github.com/ks888/hornet/server/testdata"
	_, filename, _, _ := runtime.Caller(0)
	dirPath := filepath.Join(filepath.Dir(filename), "testdata")

	usedIDs := make(map[int64]struct{})
	ch := make(chan int64)
	numGoRoutines := 10
	numIter := 2
	for i := 0; i < numGoRoutines; i++ {
		go func() {
			for j := 0; j < numIter; j++ {
				job, err := NewJob(importPath, dirPath, 1)
				if err != nil {
					panic(err)
				}
				ch <- job.ID
			}
		}()
	}

	for i := 0; i < numGoRoutines*numIter; i++ {
		usedID := <-ch
		if _, ok := usedIDs[usedID]; ok {
			t.Errorf("duplicate id: %d", usedID)
		}
		usedIDs[usedID] = struct{}{}
	}
}
