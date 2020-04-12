package server

import (
	"context"
	"errors"
	"go/build"
	"io"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ks888/noisegate/common/log"
	"go.uber.org/multierr"
)

// Job represents the job to test one package.
type Job struct {
	ID                               int64
	DirPath                          string
	Status                           JobStatus
	BuildTags                        string
	CreatedAt, StartedAt, FinishedAt time.Time
	// build time is not included
	ElapsedTestTime time.Duration
	TaskSets        []*TaskSet
	Tasks           []*Task
	influences      []influence
	jobFinishedCh   chan struct{}
	writer          io.Writer
}

// JobStatus represents the status of the job.
type JobStatus int

const (
	JobStatusCreated JobStatus = iota
	JobStatusSuccessful
	JobStatusFailed
)

// NewJob returns the new job. `changedFilename` and `changedOffset` specifies the position
// where the package is changed. If `changedFilename` is not empty, important test functions are executed first.
func NewJob(dirPath string, changes []change, tags string, bypass bool, w io.Writer) (*Job, error) {
	job := &Job{
		ID:            generateID(),
		DirPath:       dirPath,
		Status:        JobStatusCreated,
		BuildTags:     tags,
		CreatedAt:     time.Now(),
		jobFinishedCh: make(chan struct{}),
		writer:        w,
	}

	errCh := make(chan error)
	funcNamesCh := make(chan []string, 1)
	influencesCh := make(chan []influence, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		testFuncNames, err := retrieveTestFuncNames(dirPath)
		errCh <- err
		funcNamesCh <- testFuncNames
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var infs []influence
		var err error
		if len(changes) > 0 {
			start := time.Now()
			defer func() {
				log.Debugf("dep analysis time: %v\n", time.Since(start))
			}()
			ctxt := &build.Default
			ctxt.BuildTags = strings.Split(tags, ",")
			infs, err = FindInfluencedTests(ctxt, changes)

			if log.DebugLogEnabled() {
				for _, inf := range infs {
					var fs []string
					for f := range inf.to {
						fs = append(fs, f)
					}
					log.Debugf("%v -> [%v]\n", inf.from.Name(), strings.Join(fs, ", "))
				}
			}
		}
		errCh <- err
		influencesCh <- infs
	}()

	go func() {
		wg.Wait()
		close(errCh)
	}()

	var err error
	for e := range errCh {
		err = multierr.Combine(err, e)
	}
	// assumes the go routines send the data anyway
	job.influences = <-influencesCh
	influenced := make(map[string]struct{})
	for _, inf := range job.influences {
		for k := range inf.to {
			influenced[k] = struct{}{}
		}
	}

	ts := NewTaskSet(0, job)
	for _, testFuncName := range <-funcNamesCh {
		_, ok := influenced[testFuncName]
		t := &Task{TestFunction: testFuncName, Important: ok, Job: job}
		job.Tasks = append(job.Tasks, t)

		if ok || bypass {
			ts.Tasks = append(ts.Tasks, t)
		}
	}
	job.TaskSets = []*TaskSet{ts}

	return job, err
}

var errNoGoTestFiles = errors.New("no go test files")

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

var jobIDCounter int64

// generateID generates the unique id. This id is unique only among this server process.
func generateID() int64 {
	return atomic.AddInt64(&jobIDCounter, 1)
}

// Start starts all the task sets.
func (j *Job) Start(ctx context.Context) {
	if log.DebugLogEnabled() {
		for _, taskSet := range j.TaskSets {
			var ts []string
			for _, t := range taskSet.Tasks {
				ts = append(ts, t.TestFunction)
			}
			log.Debugf("task set %d: [%v]\n", taskSet.ID, ts)
		}
	}

	j.StartedAt = time.Now()

	for _, taskSet := range j.TaskSets {
		if err := taskSet.Start(ctx); err != nil {
			log.Printf("failed to start the worker: %v", err)
		}
	}
}

// Wait waits all the task sets finished.
func (j *Job) Wait() {
	successful := true
	for _, taskSet := range j.TaskSets {
		taskSet.Wait()

		if taskSet.Status == TaskSetStatusSuccessful {
			continue
		} else if taskSet.Status == TaskSetStatusFailed {
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

	close(j.jobFinishedCh)
}

// ChangedIdentityNames returns the list of the changed identities.
func (j *Job) ChangedIdentityNames() (result []string) {
	for _, inf := range j.influences {
		result = append(result, inf.from.Name())
	}
	return result
}

// HasAffectedTests returns true when the job has at least 1 affected test.
func (j *Job) HasAffectedTests() bool {
	for _, inf := range j.influences {
		if len(inf.to) > 0 {
			return true
		}
	}
	return false
}

// TaskSet represents the set of tasks handled by one worker.
type TaskSet struct {
	// this id must be the valid index of the Job.TaskSets.
	ID                    int
	Status                TaskSetStatus
	StartedAt, FinishedAt time.Time
	Tasks                 []*Task
	Job                   *Job
	Worker                *Worker
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
		Job:    job,
	}
}

// Start starts the worker.
func (s *TaskSet) Start(ctx context.Context) error {
	s.StartedAt = time.Now()
	s.Status = TaskSetStatusStarted

	s.Worker = NewWorker(s.Job, s)
	return s.Worker.Start(ctx)
}

// Wait waits the worker finished.
// TODO: support context
func (s *TaskSet) Wait() {
	successful, _ := s.Worker.Wait()

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
	Job          *Job
}
