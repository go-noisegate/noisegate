package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ks888/hornet/common/log"
	"go.uber.org/multierr"
)

// Job represents the job to test one package.
type Job struct {
	ID                  int64
	ImportPath, DirPath string
	Status              JobStatus
	// The path from the NFS root
	TestBinaryPath, RepoArchivePath string
	CreatedAt, FinishedAt           time.Time
	DependencyDepth                 int
	TaskSets                        []*TaskSet
	Tasks                           []*Task
	finishedCh                      chan struct{}
}

// JobStatus represents the status of the job.
type JobStatus int

const (
	JobStatusCreated JobStatus = iota
	JobStatusSuccessful
	JobStatusFailed
)

// NewJob returns the new job.
func NewJob(importPath, dirPath string, dependencyDepth int) (*Job, error) {
	id := generateID()
	errCh := make(chan error)
	binaryPathCh := make(chan string, 1) // to avoid go routine leaks
	archivePathCh := make(chan string, 1)
	funcNamesCh := make(chan []string, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		testBinaryPath, err := buildTestBinary(dirPath, id)
		if err == errNoGoTestFiles {
			err = nil
			testBinaryPath = ""
		}
		errCh <- err
		binaryPathCh <- testBinaryPath
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		repoArchivePath, err := archiveRepository(dirPath, id)
		errCh <- err
		archivePathCh <- repoArchivePath
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		testFuncNames, err := retrieveTestFuncNames(dirPath)
		errCh <- err
		funcNamesCh <- testFuncNames
	}()

	go func() {
		wg.Wait()
		close(errCh)
	}()

	var err error
	for e := range errCh {
		err = multierr.Combine(err, e)
	}
	if err != nil {
		// TODO: remove created files
		return nil, err
	}

	job := &Job{
		ID:              id,
		ImportPath:      importPath,
		DirPath:         dirPath,
		Status:          JobStatusCreated,
		TestBinaryPath:  <-binaryPathCh,
		RepoArchivePath: <-archivePathCh,
		CreatedAt:       time.Now(),
		DependencyDepth: dependencyDepth,
		finishedCh:      make(chan struct{}),
	}

	for _, testFuncName := range <-funcNamesCh {
		job.Tasks = append(job.Tasks, &Task{TestFunction: testFuncName, Status: TaskStatusCreated, Job: job})
	}
	return job, nil
}

func findRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

var errNoGoTestFiles = errors.New("no go test files (though there may be go files)")

var patternNoTestFiles = regexp.MustCompile(`(?m)\s+\[no test files\]$`)

func buildTestBinary(dirPath string, jobID int64) (string, error) {
	path := filepath.Join("bin", strconv.FormatInt(jobID, 10))
	cmd := exec.Command("go", "test", "-c", "-o", filepath.Join(sharedDir, path), ".")
	cmd.Env = append(os.Environ(), "GOOS=linux")
	cmd.Dir = dirPath
	buildLog, err := cmd.CombinedOutput()
	if err != nil {
		if strings.HasPrefix(string(buildLog), "can't load package: package .: no Go files in ") {
			return "", errNoGoTestFiles
		}
		return "", fmt.Errorf("failed to build: %w\nbuild log:\n%s", err, string(buildLog))
	}

	if matched := patternNoTestFiles.Match(buildLog); matched {
		return "", errNoGoTestFiles
	}
	return path, nil
}

func archiveRepository(dirPath string, jobID int64) (string, error) {
	root, err := findRepoRoot(dirPath)
	if err != nil {
		log.Debugf("failed to find the repo root of %s: %v", dirPath, err)
		root = dirPath
	}

	path := filepath.Join("lib", strconv.FormatInt(jobID, 10)+".tar")
	cmd := exec.Command("tar", "-cf", filepath.Join(sharedDir, path), "--exclude=./.git/", ".")
	cmd.Dir = root
	archiveLog, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to archive: %w\nlog:\n%s", err, string(archiveLog))
	}

	return path, nil
}

var patternTestFuncName = regexp.MustCompile(`(?m)^ *func *(Test[^(]+)`)

func retrieveTestFuncNames(dirPath string) ([]string, error) {
	fis, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var testFileNames []string
	for _, fi := range fis {
		if !fi.Mode().IsRegular() {
			continue
		}

		if !strings.HasSuffix(fi.Name(), "_test.go") {
			continue
		}

		testFileNames = append(testFileNames, fi.Name())
	}

	var testFuncNames []string
	for _, filename := range testFileNames {
		path := filepath.Join(dirPath, filename)
		content, err := ioutil.ReadFile(path)
		if err != nil {
			log.Printf("failed to read %s: %v\n", path, err)
			continue
		}

		matches := patternTestFuncName.FindAllStringSubmatch(string(content), -1)
		for _, match := range matches {
			testFuncNames = append(testFuncNames, match[1])
		}
	}
	return testFuncNames, nil
}

var jobIDCounter int64

// generateID generates the unique id. This id is unique only among this server process.
func generateID() int64 {
	return atomic.AddInt64(&jobIDCounter, 1)
}

// CanFinish returns true if all the task sets are done. False otherwise.
func (j *Job) CanFinish() bool {
	for _, taskSet := range j.TaskSets {
		st := taskSet.Status
		if st != TaskSetStatusSuccessful && st != TaskSetStatusFailed {
			return false
		}
	}
	return true
}

// Finish is called when all the tasks are done.
func (j *Job) Finish() {
	successful := true
	for _, taskSet := range j.TaskSets {
		st := taskSet.Status
		if st == TaskSetStatusSuccessful {
			continue
		} else if st == TaskSetStatusFailed {
			successful = false
		} else {
			log.Printf("can not finish the job %d yet\n", j.ID)
			return
		}
	}

	if successful {
		j.Status = JobStatusSuccessful
	} else {
		j.Status = JobStatusFailed
	}
	j.FinishedAt = time.Now()

	if j.TestBinaryPath != "" {
		absPath := filepath.Join(sharedDir, j.TestBinaryPath)
		if err := os.Remove(absPath); err != nil {
			log.Debugf("failed to remove the test binary %s: %v\n", absPath, err)
		}
	}

	absPath := filepath.Join(sharedDir, j.RepoArchivePath)
	if err := os.Remove(absPath); err != nil {
		log.Debugf("failed to remove the archive %s: %v\n", absPath, err)
	}

	close(j.finishedCh)
}

// WaitFinished waits until the job finished.
func (j *Job) WaitFinished() {
	<-j.finishedCh
}

// TaskSet represents the set of tasks handled by one worker.
type TaskSet struct {
	// this id must be the valid index of the Job.TaskSets.
	ID                    int
	Status                TaskSetStatus
	StartedAt, FinishedAt time.Time
	// The path from the NFS root
	LogPath    string
	Tasks      []*Task
	WorkerID   int64
	finishedCh chan struct{}
}

// TaskSetStatus represents the status of the task set.
type TaskSetStatus int

const (
	TaskSetStatusCreated TaskSetStatus = iota
	TaskSetStatusStarted
	TaskSetStatusSuccessful
	TaskSetStatusFailed
)

// NewTaskSet returns the new task set.
func NewTaskSet(id int) *TaskSet {
	return &TaskSet{
		ID:         id,
		Status:     TaskSetStatusCreated,
		LogPath:    "",
		finishedCh: make(chan struct{}),
	}
}

func (s *TaskSet) Start(workerID int64) {
	s.StartedAt = time.Now()
	s.WorkerID = workerID
	s.Status = TaskSetStatusStarted
}

func (s *TaskSet) Finish(successful bool) {
	s.FinishedAt = time.Now()
	if successful {
		s.Status = TaskSetStatusSuccessful
	} else {
		s.Status = TaskSetStatusFailed
	}
	close(s.finishedCh)
}

// WaitFinished waits until the task set finished.
func (s *TaskSet) WaitFinished() {
	<-s.finishedCh
}

// Task represents one test function.
type Task struct {
	TestFunction string
	Status       TaskStatus
	ElapsedTime  time.Duration
	Job          *Job
}

// TaskStatus
type TaskStatus int

const (
	TaskStatusCreated TaskStatus = iota
	TaskStatusStarted
	TaskStatusSuccessful
	TaskStatusFailed
)

func (t *Task) Finish(successful bool, elapsedTime time.Duration) {
	if successful {
		t.Status = TaskStatusSuccessful
	} else {
		t.Status = TaskStatusFailed
	}
	t.ElapsedTime = elapsedTime
}

// LPTPartitioner is the partitioner based on the longest processing time algorithm.
type LPTPartitioner struct {
	profiler *SimpleProfiler
}

// NewLPTPartitioner return the new LPTPartitioner.
func NewLPTPartitioner(profiler *SimpleProfiler) LPTPartitioner {
	return LPTPartitioner{profiler: profiler}
}

type taskWithExecTime struct {
	task     *Task
	execTime time.Duration
}

// Partition divides the tasks into the list of the task sets.
func (p LPTPartitioner) Partition(tasks []*Task, numPartitions int) []*TaskSet {
	sortedTasks, noProfileTasks := p.sortByExecTime(tasks)

	// O(numPartitions * numTasks). Can be O(numTasks * log(numPartitions)) using pq at the cost of complexity.
	taskSets := make([]*TaskSet, numPartitions)
	for i := 0; i < numPartitions; i++ {
		taskSets[i] = NewTaskSet(i)
	}
	totalExecTimes := make([]time.Duration, numPartitions)
	for _, t := range sortedTasks {
		minIndex := 0
		for i, totalExecTime := range totalExecTimes {
			if totalExecTime < totalExecTimes[minIndex] {
				minIndex = i
			}
		}

		taskSets[minIndex].Tasks = append(taskSets[minIndex].Tasks, t.task)
		totalExecTimes[minIndex] += t.execTime
	}

	p.distributeTasks(taskSets, noProfileTasks)
	return taskSets
}

func (p LPTPartitioner) sortByExecTime(tasks []*Task) (sorted []taskWithExecTime, noProfile []*Task) {
	for i := range tasks {
		execTime := p.profiler.ExpectExecTime(tasks[i].Job.DirPath, tasks[i].TestFunction)
		if execTime == 0 {
			noProfile = append(noProfile, tasks[i])
			continue
		}
		sorted = append(sorted, taskWithExecTime{task: tasks[i], execTime: execTime})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].execTime > sorted[j].execTime
	})
	return
}

func (p LPTPartitioner) distributeTasks(taskSets []*TaskSet, tasks []*Task) {
	curr := 0
	for _, task := range tasks {
		taskSets[curr].Tasks = append(taskSets[curr].Tasks, task)
		curr++
		if curr == len(taskSets) {
			curr = 0
		}
	}
}
