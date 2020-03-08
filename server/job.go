package server

import (
	"context"
	"errors"
	"fmt"
	"go/build"
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
	ID                               int64
	DirPath                          string
	Status                           JobStatus
	Package                          *Package
	BuildTags                        string
	TestBinaryPath                   string
	CreatedAt, StartedAt, FinishedAt time.Time
	// build time is not included
	ElapsedTestTime time.Duration
	TaskSets        []*TaskSet
	Tasks           []*Task
	EnableParallel  bool
	influences      []influence
	testEventCh     chan TestEvent
	jobFinishedCh   chan struct{}
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
func NewJob(pkg *Package, changes []change, enableParallel bool, tags string) (*Job, error) {
	job := &Job{
		ID:             generateID(),
		DirPath:        pkg.path,
		Package:        pkg,
		Status:         JobStatusCreated,
		BuildTags:      tags,
		CreatedAt:      time.Now(),
		EnableParallel: enableParallel,
		testEventCh:    make(chan TestEvent), // must be unbuffered to avoid the lost result.
		jobFinishedCh:  make(chan struct{}),
	}

	errCh := make(chan error)
	binaryPathCh := make(chan string, 1) // to avoid go routine leaks
	funcNamesCh := make(chan []string, 1)
	influencesCh := make(chan []influence, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		defer func() {
			log.Debugf("build time: %v\n", time.Since(start))
		}()

		var testBinaryPath string
		var err error
		if job.EnableParallel {
			testBinaryPath = filepath.Join(sharedDir, "bin", strconv.FormatInt(job.ID, 10))
			err = pkg.Build(testBinaryPath, tags)
			if err != nil {
				if err == errNoGoTestFiles {
					err = nil
				}
				testBinaryPath = ""
			}
		} else {
			pkg.Cancel() // to stop the unnecessary build
		}
		errCh <- err
		binaryPathCh <- testBinaryPath
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		testFuncNames, err := retrieveTestFuncNames(pkg.path)
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

	for _, testFuncName := range <-funcNamesCh {
		_, ok := influenced[testFuncName]
		job.Tasks = append(job.Tasks, &Task{TestFunction: testFuncName, Status: TaskStatusCreated, Important: ok, Job: job})
	}
	job.TestBinaryPath = <-binaryPathCh

	if err != nil {
		job.clean()
		return nil, err
	}

	return job, nil
}

func findRepoRoot(path string) string {
	fi, err := os.Stat(path)
	if os.IsNotExist(err) {
		log.Printf("%s not found", path)
		return path
	}
	dirPath := path
	if !fi.IsDir() {
		dirPath = filepath.Dir(path)
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dirPath
	out, err := cmd.Output()
	if err != nil {
		log.Printf("failed to find the repository root: %v", err)
		return dirPath
	}
	return strings.TrimSpace(string(out))
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
	if j.EnableParallel {
		j.ElapsedTestTime = j.FinishedAt.Sub(j.StartedAt)
	} else {
		// on sequential exec, the test results handler updates `ElapsedTestTime`.
	}

	j.clean()

	close(j.jobFinishedCh)
}

func (j *Job) clean() {
	if j.TestBinaryPath != "" {
		if err := os.Remove(j.TestBinaryPath); err != nil {
			log.Debugf("failed to remove the test binary %s: %v\n", j.TestBinaryPath, err)
		}
	}
}

// TaskSet represents the set of tasks handled by one worker.
type TaskSet struct {
	// this id must be the valid index of the Job.TaskSets.
	ID                    int
	Status                TaskSetStatus
	StartedAt, FinishedAt time.Time
	LogPath               string
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
		ID:      id,
		Status:  TaskSetStatusCreated,
		LogPath: filepath.Join(sharedDir, "log", "job", fmt.Sprintf("%d_%d", job.ID, id)),
		Job:     job,
	}
}

// Start starts the worker.
func (s *TaskSet) Start(ctx context.Context) error {
	s.StartedAt = time.Now()
	s.Status = TaskSetStatusStarted

	s.Worker = NewWorker(ctx, s.Job, s)
	return s.Worker.Start()
}

// Wait waits the worker finished.
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
	Status       TaskStatus
	Important    bool
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

// TestResult represents the test result of one test function.
type TestResult struct {
	TestName    string
	Successful  bool
	ElapsedTime time.Duration
	Output      []string
}

// LPTPartitioner is the partitioner based on the longest processing time algorithm.
type LPTPartitioner struct {
	profiler *TaskProfiler
}

// NewLPTPartitioner return the new LPTPartitioner.
func NewLPTPartitioner(profiler *TaskProfiler) LPTPartitioner {
	return LPTPartitioner{profiler: profiler}
}

type taskWithExecTime struct {
	task     *Task
	execTime time.Duration
}

// Partition divides the tasks into the list of the task sets.
func (p LPTPartitioner) Partition(tasks []*Task, job *Job, numPartitions int) []*TaskSet {
	sortedTasks, noProfileTasks := p.sortByExecTime(tasks, job)

	// O(numPartitions * numTasks). Can be O(numTasks * log(numPartitions)) using pq at the cost of complexity.
	taskSets := make([]*TaskSet, numPartitions)
	for i := 0; i < numPartitions; i++ {
		taskSets[i] = NewTaskSet(len(job.TaskSets)+i, job)
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

func (p LPTPartitioner) sortByExecTime(tasks []*Task, job *Job) (sorted []taskWithExecTime, noProfile []*Task) {
	for i := range tasks {
		execTime := p.profiler.ExpectExecTime(job.DirPath, tasks[i].TestFunction)
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
