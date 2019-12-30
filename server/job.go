package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var jobs = make(map[int64]Job)

// Job represents the job to test one package.
type Job struct {
	ID                  int64
	ImportPath, DirPath string
	Status              JobStatus
	// The path from the NFS root
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
	var tasks []Task
	for _, testFuncName := range testFuncNames {
		tasks = append(tasks, Task{TestFunction: testFuncName, Status: TaskStatusCreated})
	}

	return Job{
		ID:              id,
		ImportPath:      importPath,
		DirPath:         dirPath,
		Status:          JobStatusCreated,
		TestBinaryPath:  testBinaryPath,
		CreatedAt:       time.Now(),
		DependencyDepth: dependencyDepth,
		Tasks:           tasks,
	}, nil
}

var errNoGoFiles = errors.New("no go files")

func buildTestBinary(dirPath string, jobID int64) (string, error) {
	filename := strconv.FormatInt(jobID, 10)
	cmd := exec.Command("go", "test", "-c", "-o", filepath.Join(sharedTestBinaryDir, filename), ".")
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

	joinedPath := filepath.Join(sharedTestBinaryDir, j.TestBinaryPath)
	if err := os.Remove(joinedPath); err != nil {
		log.Printf("failed to remove the test binary %s: %v\n", joinedPath, err)
	}
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

// Partition divides the tasks into the list of the task sets.
func Partition(tasks []Task) ([]TaskSet, error) {
	return nil, nil
}
