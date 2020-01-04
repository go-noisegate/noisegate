package server

import (
	"errors"
	"fmt"
	"go/build"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// Job represents the job to test one package.
type Job struct {
	ID                  int64
	ImportPath, DirPath string
	Status              JobStatus
	// The path from the NFS root
	TestBinaryPath        string
	CreatedAt, FinishedAt time.Time
	DependencyDepth       int
	TaskSets              []*TaskSet
	Tasks                 []*Task
}

// JobStatus represents the status of the job.
type JobStatus int

const (
	JobStatusCreated JobStatus = iota
	JobStatusSuccessful
	JobStatusFailed
)

// NewJob returns the new job.
func NewJob(importPath, dirPath string, dependencyDepth int) (Job, error) {
	id := generateID()
	testBinaryPath, err := buildTestBinary(dirPath, id)
	if err == errNoGoFiles {
		testBinaryPath = ""
	} else if err != nil {
		return Job{}, err
	}

	testFuncNames, err := retrieveTestFuncNames(dirPath)
	if err != nil {
		return Job{}, err
	}

	job := Job{
		ID:              id,
		ImportPath:      importPath,
		DirPath:         dirPath,
		Status:          JobStatusCreated,
		TestBinaryPath:  testBinaryPath,
		CreatedAt:       time.Now(),
		DependencyDepth: dependencyDepth,
	}

	for _, testFuncName := range testFuncNames {
		job.Tasks = append(job.Tasks, &Task{TestFunction: testFuncName, Status: TaskStatusCreated, Job: &job})
	}
	return job, nil
}

// NewJobWithImportGraph returns the new job and the channel to receive the import graph when ready.
func NewJobWithImportGraph(importPath, dirPath string, dependencyDepth int) (Job, chan ImportGraph, error) {
	ch := make(chan ImportGraph, 1)
	go func() {
		repoRoot, err := findRepoRoot(dirPath)
		if err != nil {
			log.Printf("failed to find the repository root of %s: %v", dirPath, err)
			repoRoot = dirPath
		} else {
			repoRoot = strings.TrimSpace(repoRoot)
		}
		ctxt := &build.Default
		ch <- BuildImportGraph(ctxt, repoRoot)
	}()

	job, err := NewJob(importPath, dirPath, dependencyDepth)
	return job, ch, err
}

func findRepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

var errNoGoFiles = errors.New("no go files")

func buildTestBinary(dirPath string, jobID int64) (string, error) {
	filename := strconv.FormatInt(jobID, 10)
	cmd := exec.Command("go", "test", "-c", "-o", filepath.Join(sharedDir, filename), ".")
	cmd.Env = append(os.Environ(), "GOOS=linux")
	cmd.Dir = dirPath
	buildLog, err := cmd.CombinedOutput()
	if err != nil {
		if strings.HasPrefix(string(buildLog), "can't load package: package .: no Go files in ") {
			return "", errNoGoFiles
		}
		return "", fmt.Errorf("failed to build: %w\nbuild log:\n%s", err, string(buildLog))
	}
	return filename, nil
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

// Finished is called when all the tasks are done.
func (j *Job) Finished(successful bool) {
	if successful {
		j.Status = JobStatusSuccessful
	} else {
		j.Status = JobStatusFailed
	}
	j.FinishedAt = time.Now()

	joinedPath := filepath.Join(sharedDir, j.TestBinaryPath)
	if err := os.Remove(joinedPath); err != nil {
		log.Printf("failed to remove the test binary %s: %v\n", joinedPath, err)
	}
}

// TaskSet represents the set of tasks handled by one worker.
type TaskSet struct {
	// this id must be the valid index of the Job.TaskSets.
	ID                    int
	Status                TaskSetStatus
	StartedAt, FinishedAt time.Time
	Log                   []byte
	Tasks                 []*Task
	WorkerID              int64
}

// TaskSetStatus represents the status of the task set.
type TaskSetStatus int

const (
	TaskSetStatusCreated TaskSetStatus = iota
	TaskSetStatusStarted
	TaskSetStatusSuccessful
	TaskSetStatusFailed
)

func (s *TaskSet) Started(workerID int64) {
	s.StartedAt = time.Now()
	s.WorkerID = workerID
	s.Status = TaskSetStatusStarted
}

func (s *TaskSet) Finished(successful bool, log []byte) {
	s.FinishedAt = time.Now()
	if successful {
		s.Status = TaskSetStatusSuccessful
	} else {
		s.Status = TaskSetStatusFailed
	}
	s.Log = log
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

func (t *Task) Finished(successful bool, elapsedTime time.Duration) {
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
		taskSets[i] = &TaskSet{ID: i, Status: TaskSetStatusCreated}
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
