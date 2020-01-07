package server

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ks888/hornet/common/log"
)

func TestMain(m *testing.M) {
	dir, err := ioutil.TempDir("", "hornet")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	setSharedDir(dir)

	log.EnableDebugLog(true)

	os.Exit(m.Run())
}

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
	if !strings.HasPrefix(job.TestBinaryPath, "bin/") {
		t.Errorf("wrong path: %v", job.TestBinaryPath)
	}
	if !strings.HasPrefix(job.RepoArchivePath, "lib/") {
		t.Errorf("wrong path: %v", job.RepoArchivePath)
	}
	checkArchiveContent(t, filepath.Join(sharedDir, job.RepoArchivePath), "./README.md")

	expectedTasks := []Task{
		{TestFunction: "TestSum", Job: job},
		{TestFunction: "TestSum_ErrorCase", Job: job},
		{TestFunction: "TestSum_Add1", Job: job},
	}
	if len(expectedTasks) != len(job.Tasks) {
		t.Errorf("invalid number of tasks: %d, %#v", len(job.Tasks), job.Tasks)
	}
	for i, actualTask := range job.Tasks {
		if !reflect.DeepEqual(expectedTasks[i], *actualTask) {
			t.Errorf("wrong task: %#v", actualTask)
		}
	}
}

func checkArchiveContent(t *testing.T, archivePath, filenameToCheck string) {
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("failed to open %s: %v", archivePath, err)
	}

	tarReader := tar.NewReader(f)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("failed to check tar contents: %v", err)
		}

		if header.Name == filenameToCheck {
			return
		}
	}
	t.Errorf("can't find %s in %s", filenameToCheck, archivePath)
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
	importPath := "github.com/ks888/hornet/server/testdata/no_go_files"
	_, filename, _, _ := runtime.Caller(0)
	dirPath := filepath.Join(filepath.Dir(filename), "testdata", "no_go_files")

	usedIDs := make(map[int64]struct{})
	ch := make(chan int64)
	numGoRoutines := 10
	numIter := 10
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

func TestNewJob_NoGoFiles(t *testing.T) {
	importPath := "github.com/ks888/hornet/server/testdata/no_go_files"
	_, filename, _, _ := runtime.Caller(0)
	dirPath := filepath.Join(filepath.Dir(filename), "testdata", "no_go_files")

	job, err := NewJob(importPath, dirPath, 1)
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}
	if job.TestBinaryPath != "" {
		t.Errorf("test binary path is not empty")
	}
	if len(job.Tasks) != 0 {
		t.Errorf("the dir has no go test functions: %d", len(job.Tasks))
	}
}

func TestNewJob_NoGoTestFiles(t *testing.T) {
	importPath := "github.com/ks888/hornet/server/testdata/no_go_test_files"
	_, filename, _, _ := runtime.Caller(0)
	dirPath := filepath.Join(filepath.Dir(filename), "testdata", "no_go_test_files")

	job, err := NewJob(importPath, dirPath, 1)
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}
	if job.TestBinaryPath != "" {
		t.Errorf("test binary path is not empty")
	}
	if len(job.Tasks) != 0 {
		t.Errorf("the dir has no go test functions: %d", len(job.Tasks))
	}
}

func TestJob_Finish(t *testing.T) {
	importPath := "github.com/ks888/hornet/server/testdata"
	_, filename, _, _ := runtime.Caller(0)
	dirPath := filepath.Join(filepath.Dir(filename), "testdata")
	job, err := NewJob(importPath, dirPath, 1)
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}

	job.Finish()
	if job.Status != JobStatusSuccessful {
		t.Errorf("wrong status: %v", job.Status)
	}
	if _, err := os.Stat(filepath.Join(sharedDir, job.TestBinaryPath)); !os.IsNotExist(err) {
		t.Errorf("test binary still exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sharedDir, job.RepoArchivePath)); !os.IsNotExist(err) {
		t.Errorf("archive still exist: %v", err)
	}
	job.WaitFinished()
}

func TestTaskSet_Start(t *testing.T) {
	set := NewTaskSet(1)
	set.Start(1)
	if set.Status != TaskSetStatusStarted {
		t.Errorf("wrong status: %v", set.Status)
	}
	if set.WorkerID != 1 {
		t.Errorf("wrong worker: %d", set.WorkerID)
	}
	if set.StartedAt.IsZero() {
		t.Errorf("StartedAt is zero")
	}
}

func TestTaskSet_Finish(t *testing.T) {
	set := NewTaskSet(1)
	log := "test log"
	set.Finish(true, []byte(log))
	if set.Status != TaskSetStatusSuccessful {
		t.Errorf("wrong status: %v", set.Status)
	}
	if string(set.Log) != log {
		t.Errorf("wrong log: %s", string(set.Log))
	}
	set.WaitFinished()
}

func TestTask_Finish(t *testing.T) {
	task := Task{Status: TaskStatusCreated}
	task.Finish(true, time.Second)
	if task.Status != TaskStatusSuccessful {
		t.Errorf("wrong status: %v", task.Status)
	}
	if task.ElapsedTime == 0 {
		t.Errorf("elapsed time is 0")
	}
}

func TestLPTPartition(t *testing.T) {
	profiler := NewSimpleProfiler()
	profiler.Add("/path", "f1", time.Millisecond)
	profiler.Add("/path", "f2", time.Second)
	profiler.Add("/path", "f3", time.Millisecond)
	p := NewLPTPartitioner(profiler)

	job := &Job{DirPath: "/path"}
	tasks := []*Task{
		{TestFunction: "f1", Job: job},
		{TestFunction: "f2", Job: job},
		{TestFunction: "f3", Job: job},
	}
	taskSets := p.Partition(tasks, 2)
	if len(taskSets) != 2 {
		t.Fatalf("wrong number of task sets: %d", len(taskSets))
	}
	if taskSets[0].ID != 0 {
		t.Fatalf("wrong id: %d", taskSets[0].ID)
	}
	if len(taskSets[0].Tasks) != 1 {
		t.Fatalf("wrong number of tasks: %d", len(taskSets[0].Tasks))
	}
	if *taskSets[0].Tasks[0] != *tasks[1] {
		t.Errorf("wrong task ptr: %v", taskSets[0].Tasks[0])
	}

	if taskSets[1].ID != 1 {
		t.Fatalf("wrong id: %d", taskSets[1].ID)
	}
	if *taskSets[1].Tasks[0] != *tasks[0] {
		t.Errorf("wrong task ptr: %v", taskSets[1].Tasks[0])
	}
	if *taskSets[1].Tasks[1] != *tasks[2] {
		t.Errorf("wrong task ptr: %v", taskSets[1].Tasks[0])
	}
}

func TestLPTPartition_EmptyProfile(t *testing.T) {
	job := &Job{}
	tasks := []*Task{
		{TestFunction: "f1", Job: job},
		{TestFunction: "f2", Job: job},
		{TestFunction: "f3", Job: job},
	}
	p := NewLPTPartitioner(NewSimpleProfiler())
	taskSets := p.Partition(tasks, 2)
	if len(taskSets) != 2 {
		t.Fatalf("wrong number of task sets: %d", len(taskSets))
	}
	if taskSets[0].Status != TaskSetStatusCreated {
		t.Errorf("wrong status: %v", taskSets[0].Status)
	}
	if len(taskSets[0].Tasks) != 2 {
		t.Fatalf("wrong number of tasks: %d", len(taskSets[0].Tasks))
	}
	if *taskSets[0].Tasks[0] != *tasks[0] {
		t.Errorf("wrong task ptr: %v", taskSets[0].Tasks[0])
	}
	if *taskSets[1].Tasks[0] != *tasks[1] {
		t.Errorf("wrong task ptr: %v", taskSets[1].Tasks[0])
	}
	if *taskSets[0].Tasks[1] != *tasks[2] {
		t.Errorf("wrong task ptr: %v", taskSets[0].Tasks[1])
	}
}
