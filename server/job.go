package server

import (
	"context"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ks888/noisegate/common/log"
)

// Job represents the job to test one package.
type Job struct {
	ID                               int64
	DirPath                          string
	Status                           JobStatus
	GoTestOptions                    []string
	CreatedAt, StartedAt, FinishedAt time.Time
	TaskSets                         []*TaskSet
	Tasks                            []*Task
	influences                       []influence
	writer                           io.Writer
}

// JobStatus represents the status of the job.
type JobStatus int

const (
	JobStatusCreated JobStatus = iota
	JobStatusSuccessful
	JobStatusFailed
)

// NewJob returns the new job.
func NewJob(dirPath string, bypass bool, changes []Change, goTestOpts []string, w io.Writer) (*Job, error) {
	job := &Job{
		ID:            generateID(),
		DirPath:       dirPath,
		Status:        JobStatusCreated,
		GoTestOptions: goTestOpts,
		CreatedAt:     time.Now(),
		writer:        w,
	}

	testFuncNames, err := retrieveTestFuncNames(dirPath)
	if err != nil {
		return nil, err
	}

	if bypass {
		selectAllTasks(job, testFuncNames)
		w.Write([]byte("Run all tests:\n"))
		return job, nil
	}

	if len(changes) == 0 {
		selectNoTasks(job, testFuncNames)
		w.Write([]byte("Changed: []\n"))
		return job, nil
	}

	start := time.Now()
	defer func() {
		log.Debugf("dependency analysis time: %v\n", time.Since(start))
	}()

	ctxt := &build.Default
	ctxt.BuildTags = strings.Split(findOptionValue(goTestOpts, "tags"), ",")
	job.influences, err = findInfluencedTests(ctxt, job.DirPath, changes)
	if err != nil {
		return nil, err
	}

	if log.DebugLogEnabled() {
		for _, inf := range job.influences {
			var fs []string
			for f := range inf.to {
				fs = append(fs, f)
			}
			log.Debugf("%v -> [%v]\n", inf.from.Name(), strings.Join(fs, ", "))
		}
	}

	selectInfluencedTasks(job, testFuncNames)

	w.Write([]byte(fmt.Sprintf("Changed: [%s]\n", strings.Join(job.changedIdentityNames(), ", "))))
	return job, err
}

var jobIDCounter int64

// generateID generates the unique id. This id is unique only among this server process.
func generateID() int64 {
	return atomic.AddInt64(&jobIDCounter, 1)
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
			if match[1] == "TestMain" {
				continue
			}
			testFuncNames = append(testFuncNames, match[1])
		}
	}
	return testFuncNames, nil
}

func selectNoTasks(job *Job, testFuncNames []string) {
	for _, testFuncName := range testFuncNames {
		job.Tasks = append(job.Tasks, &Task{TestFunction: testFuncName})
	}
	job.TaskSets = []*TaskSet{NewTaskSet(0, job)}
}

func selectAllTasks(job *Job, testFuncNames []string) {
	ts := NewTaskSet(0, job)
	for _, testFuncName := range testFuncNames {
		t := &Task{TestFunction: testFuncName, Important: true}
		job.Tasks = append(job.Tasks, t)
		ts.Tasks = append(ts.Tasks, t)
	}
	job.TaskSets = []*TaskSet{ts}
}

func selectInfluencedTasks(job *Job, testFuncNames []string) {
	influenced := make(map[string]struct{})
	for _, inf := range job.influences {
		for k := range inf.to {
			influenced[k] = struct{}{}
		}
	}

	ts := NewTaskSet(0, job)
	for _, testFuncName := range testFuncNames {
		_, ok := influenced[testFuncName]
		t := &Task{TestFunction: testFuncName, Important: ok}
		job.Tasks = append(job.Tasks, t)

		if ok {
			ts.Tasks = append(ts.Tasks, t)
		}
	}
	job.TaskSets = []*TaskSet{ts}
}

func findOptionValue(opts []string, keyWithoutHyphen string) string {
	i := findOptionValueIndex(opts, keyWithoutHyphen)
	if i == -1 {
		return ""
	}
	return opts[i]
}

func findOptionValueIndex(opts []string, keyWithoutHyphen string) int {
	for i, opt := range opts {
		if opt == "-"+keyWithoutHyphen || opt == "--"+keyWithoutHyphen {
			if i+1 < len(opts) {
				return i + 1
			}
		}
	}
	return -1
}

// Run runs all the task sets in order (not in parallel).
func (j *Job) Run(ctx context.Context) {
	j.StartedAt = time.Now()

	successful := true
	for _, taskSet := range j.TaskSets {
		if err := taskSet.Start(ctx); err != nil {
			log.Printf("failed to start the worker: %v", err)
		}
		taskSet.Wait()

		if taskSet.Status == TaskSetStatusFailed {
			successful = false
		}
	}

	if successful {
		j.Status = JobStatusSuccessful
	} else {
		j.Status = JobStatusFailed
	}
	j.FinishedAt = time.Now()
}

func (j *Job) changedIdentityNames() (result []string) {
	for _, inf := range j.influences {
		result = append(result, inf.from.Name())
	}
	return result
}

// TaskSet represents the set of tasks handled by one worker.
type TaskSet struct {
	// this id must be the valid index of the Job.TaskSets.
	ID                    int
	Status                TaskSetStatus
	StartedAt, FinishedAt time.Time
	Tasks                 []*Task
	job                   *Job
	worker                *worker
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
func NewTaskSet(id int, job *Job) *TaskSet {
	return &TaskSet{
		ID:     id,
		Status: TaskSetStatusCreated,
		job:    job,
	}
}

// Start starts the worker.
func (s *TaskSet) Start(ctx context.Context) error {
	s.StartedAt = time.Now()
	s.Status = TaskSetStatusStarted

	s.worker = newWorker(s.job, s)
	return s.worker.Start(ctx)
}

// Wait waits the worker finished.
func (s *TaskSet) Wait() {
	successful, _ := s.worker.Wait()

	s.FinishedAt = time.Now()
	if successful {
		s.Status = TaskSetStatusSuccessful
	} else {
		s.Status = TaskSetStatusFailed
	}
}

// Task represents one test function.
type Task struct {
	TestFunction string
	Important    bool
}
