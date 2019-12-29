package server

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

var jobs = make(map[int64]Job)

// Job represents the job to test one package.
type Job struct {
	ID                    int64
	ImportPath, DirPath   string
	Status                JobStatus
	TestBinaryPath        string
	CreatedAt, FinishedAt time.Time
	DependencyDepth       int
	TaskSets              []TaskSet
	Tasks                 []Task
}

// JobStatus represents the status of the job.
type JobStatus int

const (
	JobStatusCreated JobStatus = iota
	JobStatusSuccessful
	JobStatusFailed
)

func NewJob(importPath, dirPath string, dependencyDepth int) (Job, error) {
	// TODO: build

	testFuncNames, err := retrieveTestFuncNames(dirPath)
	if err != nil {
		return Job{}, err
	}
	var tasks []Task
	for _, testFuncName := range testFuncNames {
		tasks = append(tasks, Task{TestFunction: testFuncName, Status: TaskStatusCreated})
	}

	return Job{
		ID:              generateID(),
		ImportPath:      importPath,
		DirPath:         dirPath,
		Status:          JobStatusCreated,
		CreatedAt:       time.Now(),
		DependencyDepth: dependencyDepth,
		Tasks:           tasks,
	}, nil
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
			log.Printf("failed to read %s: %v", path, err)
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

// TaskSet represents the set of tasks handled by one worker.
type TaskSet struct {
	Status                TaskSetStatus
	StartedAt, FinishedAt time.Time
	Log                   []byte
	Tasks                 []Task
}

// TaskSetStatus represents the status of the task set.
type TaskSetStatus int

const (
	TaskSetStatusCreated TaskSetStatus = iota
	TaskSetStatusStarted
	TaskSetStatusSuccessful
	TaskSetStatusFailed
)

// Task represents one test function.
type Task struct {
	TestFunction string
	Status       TaskStatus
}

// TaskStatus
type TaskStatus int

const (
	TaskStatusCreated TaskStatus = iota
	TaskStatusStarted
	TaskStatusSuccessful
	TaskStatusFailed
)
