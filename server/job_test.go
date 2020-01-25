package server

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
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
	SetUpSharedDir(dir)

	log.EnableDebugLog(true)

	os.Exit(m.Run())
}

func TestNewJob(t *testing.T) {
	currDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	dirPath := filepath.Join(currDir, "testdata")
	repoRoot := filepath.Dir(currDir) + string(filepath.Separator)

	destDir, err := ioutil.TempDir("", "hornet_test")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	repo := NewSyncedRepository(repoRoot, destDir)
	job, err := NewJob(dirPath, repo, 1)
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
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
	if job.Repository != repo {
		t.Errorf("wrong repository: %v", job.Repository)
	}
	checkRepositoryContent(t, job.Repository.destPath, "README.md")

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

func checkRepositoryContent(t *testing.T, repoPath, filenameToCheck string) {
	fis, err := ioutil.ReadDir(repoPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", repoPath, err)
	}

	for _, fi := range fis {
		if fi.Name() == filenameToCheck {
			return
		}
	}

	t.Errorf("can't find %s under %s", filenameToCheck, repoPath)
}

func TestNewJob_InvalidDirPath(t *testing.T) {
	dirPath := "/not/exist"
	_, err := NewJob(dirPath, NewSyncedRepository("/not", ""), 1)
	if err == nil {
		t.Fatalf("err should not be nil: %v", err)
	}
}

func TestNewJob_UniqueIDCheck(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "no_go_files")

	destDir, err := ioutil.TempDir("", "hornet_test")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)
	repo := NewSyncedRepository(dirPath, destDir)

	ch := make(chan int64)
	numGoRoutines := 10
	numIter := 10
	for i := 0; i < numGoRoutines; i++ {
		go func() {
			for j := 0; j < numIter; j++ {
				job, err := NewJob(dirPath, repo, 1)
				if err != nil {
					panic(err)
				}
				ch <- job.ID
				job.Finish()
			}
		}()
	}

	usedIDs := make(map[int64]struct{})
	for i := 0; i < numGoRoutines*numIter; i++ {
		usedID := <-ch
		if _, ok := usedIDs[usedID]; ok {
			t.Errorf("duplicate id: %d", usedID)
		}
		usedIDs[usedID] = struct{}{}
	}
}

func TestNewJob_NoGoFiles(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "no_go_files")

	destDir, err := ioutil.TempDir("", "hornet_test")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	job, err := NewJob(dirPath, NewSyncedRepository(dirPath, destDir), 1)
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
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata", "no_go_test_files")

	destDir, err := ioutil.TempDir("", "hornet_test")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	job, err := NewJob(dirPath, NewSyncedRepository(dirPath, destDir), 1)
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
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata")

	destDir, err := ioutil.TempDir("", "hornet_test")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(destDir)

	repo := NewSyncedRepository(currDir, destDir)
	job, err := NewJob(dirPath, repo, 1)
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
	job.Repository.Lock(nil) // Finish must unlock the repository
	job.WaitFinished()
}

func TestTaskSet_Start(t *testing.T) {
	set := NewTaskSet(1, &Job{ID: 1})
	set.Start(workerGroupName, 1)
	if set.Status != TaskSetStatusStarted {
		t.Errorf("wrong status: %v", set.Status)
	}
	if set.WorkerID != 1 {
		t.Errorf("wrong worker: %d", set.WorkerID)
	}
	if set.StartedAt.IsZero() {
		t.Errorf("StartedAt is zero")
	}
	if set.LogPath != "log/1_1" {
		t.Errorf("wrong log path: %s", set.LogPath)
	}
}

func TestTaskSet_Finish(t *testing.T) {
	set := NewTaskSet(1, &Job{ID: 1})
	set.Finish(true)
	if set.Status != TaskSetStatusSuccessful {
		t.Errorf("wrong status: %v", set.Status)
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
	job.Tasks = []*Task{
		{TestFunction: "f1", Job: job},
		{TestFunction: "f2", Job: job},
		{TestFunction: "f3", Job: job},
	}
	taskSets := p.Partition(job, 2)
	if len(taskSets) != 2 {
		t.Fatalf("wrong number of task sets: %d", len(taskSets))
	}
	if taskSets[0].ID != 0 {
		t.Fatalf("wrong id: %d", taskSets[0].ID)
	}
	if len(taskSets[0].Tasks) != 1 {
		t.Fatalf("wrong number of tasks: %d", len(taskSets[0].Tasks))
	}
	if *taskSets[0].Tasks[0] != *job.Tasks[1] {
		t.Errorf("wrong task ptr: %v", taskSets[0].Tasks[0])
	}

	if taskSets[1].ID != 1 {
		t.Fatalf("wrong id: %d", taskSets[1].ID)
	}
	if *taskSets[1].Tasks[0] != *job.Tasks[0] {
		t.Errorf("wrong task ptr: %v", taskSets[1].Tasks[0])
	}
	if *taskSets[1].Tasks[1] != *job.Tasks[2] {
		t.Errorf("wrong task ptr: %v", taskSets[1].Tasks[0])
	}
}

func TestLPTPartition_EmptyProfile(t *testing.T) {
	job := &Job{}
	job.Tasks = []*Task{
		{TestFunction: "f1", Job: job},
		{TestFunction: "f2", Job: job},
		{TestFunction: "f3", Job: job},
	}
	p := NewLPTPartitioner(NewSimpleProfiler())
	taskSets := p.Partition(job, 2)
	if len(taskSets) != 2 {
		t.Fatalf("wrong number of task sets: %d", len(taskSets))
	}
	if taskSets[0].Status != TaskSetStatusCreated {
		t.Errorf("wrong status: %v", taskSets[0].Status)
	}
	if len(taskSets[0].Tasks) != 2 {
		t.Fatalf("wrong number of tasks: %d", len(taskSets[0].Tasks))
	}
	if *taskSets[0].Tasks[0] != *job.Tasks[0] {
		t.Errorf("wrong task ptr: %v", taskSets[0].Tasks[0])
	}
	if *taskSets[1].Tasks[0] != *job.Tasks[1] {
		t.Errorf("wrong task ptr: %v", taskSets[1].Tasks[0])
	}
	if *taskSets[0].Tasks[1] != *job.Tasks[2] {
		t.Errorf("wrong task ptr: %v", taskSets[0].Tasks[1])
	}
}

func TestFindRepoRoot_File(t *testing.T) {
	curr, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}

	repoRoot := findRepoRoot("job_test.go")
	if repoRoot != filepath.Dir(curr) {
		t.Errorf("unexpected repo root: %s", repoRoot)
	}
}

func TestFindRepoRoot_Dir(t *testing.T) {
	curr, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}

	repoRoot := findRepoRoot(".")
	if repoRoot != filepath.Dir(curr) {
		t.Errorf("unexpected repo root: %s", repoRoot)
	}
}

func TestFindRepoRoot_NotExist(t *testing.T) {
	path := "/path/to/not/exist/file"
	repoRoot := findRepoRoot(path)
	if repoRoot != path {
		t.Errorf("unexpected repo root: %s", repoRoot)
	}
}
