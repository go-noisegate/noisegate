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
	ID      int64
	DirPath string
	Status  JobStatus
	Package *Package
	// The path from the shared dir
	TestBinaryPath        string
	Repository            *SyncedRepository
	RepoToPackagePath     string
	CreatedAt, FinishedAt time.Time
	DependencyDepth       int
	TaskSets              []*TaskSet
	Tasks                 []*Task
	finishedCh            chan struct{}
}

// JobStatus represents the status of the job.
type JobStatus int

const (
	JobStatusCreated JobStatus = iota
	JobStatusSuccessful
	JobStatusFailed
)

// NewJob returns the new job.
// When the job is created successfully, the repository is locked.
func NewJob(repository *SyncedRepository, pkg *Package, dependencyDepth int) (*Job, error) {
	job := &Job{
		ID:              generateID(),
		DirPath:         pkg.path,
		Package:         pkg,
		Status:          JobStatusCreated,
		Repository:      repository,
		CreatedAt:       time.Now(),
		DependencyDepth: dependencyDepth,
		finishedCh:      make(chan struct{}),
	}

	relPath, err := filepath.Rel(repository.srcPath, pkg.path)
	if err != nil {
		return nil, err
	}
	job.RepoToPackagePath = relPath

	errCh := make(chan error)
	binaryPathCh := make(chan string, 1) // to avoid go routine leaks
	funcNamesCh := make(chan []string, 1)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		defer func() {
			log.Debugf("build time: %v\n", time.Since(start))
		}()

		testBinaryPath := filepath.Join("bin", strconv.FormatInt(job.ID, 10))
		err := pkg.Build(filepath.Join(sharedDir, testBinaryPath))
		if err != nil {
			if err == errNoGoTestFiles {
				err = nil
			}
			testBinaryPath = ""
		}
		errCh <- err
		binaryPathCh <- testBinaryPath
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		repository.Lock(job)

		start := time.Now()
		defer func() {
			log.Debugf("sync time: %v\n", time.Since(start))
		}()

		errCh <- repository.SyncInLock()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		testFuncNames, err := retrieveTestFuncNames(pkg.path)
		errCh <- err
		funcNamesCh <- testFuncNames
	}()

	go func() {
		wg.Wait()
		close(errCh)
	}()

	for e := range errCh {
		err = multierr.Combine(err, e)
	}
	// assumes the go routines send the data anyway
	job.TestBinaryPath = <-binaryPathCh
	for _, testFuncName := range <-funcNamesCh {
		job.Tasks = append(job.Tasks, &Task{TestFunction: testFuncName, Status: TaskStatusCreated, Job: job})
	}

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

var errNoGoTestFiles = errors.New("no go test files (though there may be go files)")

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

	j.clean()

	close(j.finishedCh)
}

func (j *Job) clean() {
	if j.TestBinaryPath != "" {
		absPath := filepath.Join(sharedDir, j.TestBinaryPath)
		if err := os.Remove(absPath); err != nil {
			log.Debugf("failed to remove the test binary %s: %v\n", absPath, err)
		}
	}

	j.Repository.Unlock()

	// should not remove task sets' logs here because the client may not read them yet.
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
	// The path from the shared dir
	LogPath         string
	Tasks           []*Task
	WorkerGroupName string
	WorkerID        int
	finishedCh      chan struct{}
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
		ID:         id,
		Status:     TaskSetStatusCreated,
		LogPath:    filepath.Join("log", fmt.Sprintf("%d_%d", job.ID, id)),
		finishedCh: make(chan struct{}),
	}
}

func (s *TaskSet) Start(groupName string, workerID int) {
	s.StartedAt = time.Now()
	s.WorkerGroupName = groupName
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
func (p LPTPartitioner) Partition(job *Job, numPartitions int) []*TaskSet {
	sortedTasks, noProfileTasks := p.sortByExecTime(job.Tasks)

	// O(numPartitions * numTasks). Can be O(numTasks * log(numPartitions)) using pq at the cost of complexity.
	taskSets := make([]*TaskSet, numPartitions)
	for i := 0; i < numPartitions; i++ {
		taskSets[i] = NewTaskSet(i, job)
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
	p.sortTasksByName(taskSets)
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

func (p LPTPartitioner) sortTasksByName(taskSets []*TaskSet) {
	for _, ts := range taskSets {
		sort.Slice(ts.Tasks, func(i, j int) bool { return ts.Tasks[i].TestFunction < ts.Tasks[j].TestFunction })
	}
}
