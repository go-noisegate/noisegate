package server

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestJobManager_Partition(t *testing.T) {
	job := &Job{ID: 1}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job}, &Task{TestFunction: "TestFunc2", Job: job})

	manager := NewJobManager()
	if err := manager.partition(job, 2); err != nil {
		t.Fatal(err)
	}
	if len(job.TaskSets) != 2 {
		t.Errorf("wrong number of task sets: %d", len(job.TaskSets))
	}
}

func TestJobManager_Partition_NoPartitions(t *testing.T) {
	job := &Job{ID: 1}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job}, &Task{TestFunction: "TestFunc2", Job: job})

	manager := NewJobManager()
	if err := manager.partition(job, 0); err == nil {
		t.Fatal("nil error")
	}
}

func TestJobManager_Partition_ImportantTasks(t *testing.T) {
	job := &Job{ID: 1}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job, Important: true}, &Task{TestFunction: "TestFunc2", Job: job, Important: false})

	manager := NewJobManager()
	if err := manager.partition(job, 2); err != nil {
		t.Fatal(err)
	}
	if len(job.TaskSets) != 4 {
		t.Errorf("wrong number of task sets: %d", len(job.TaskSets))
	}
	if !job.TaskSets[0].Tasks[0].Important {
		t.Error("important task should come first")
	}
}

func TestJobManager_StartAndWaitJob(t *testing.T) {
	job := &Job{
		ID:             1,
		finishedCh:     make(chan struct{}),
		Package:        &Package{},
		TestBinaryPath: "echo",
	}
	for _, t := range []string{"Test1", "Test2"} {
		job.Tasks = append(job.Tasks, &Task{TestFunction: t, Job: job})
	}

	manager := NewJobManager()
	manager.StartJob(context.Background(), job, 2, &bytes.Buffer{})
	if _, ok := manager.jobs[job.ID]; !ok {
		t.Errorf("job is not stored: %d", job.ID)
	}
	if len(job.TaskSets) != 2 {
		t.Errorf("wrong number of task sets: %d", len(job.TaskSets))
	}
	if job.TaskSets[0].Worker == nil {
		t.Errorf("work in task set is nil")
	}
	if job.TaskSets[1].Worker == nil {
		t.Errorf("work in task set is nil")
	}

	if err := manager.WaitJob(job.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.jobs[job.ID]; ok {
		t.Errorf("job %d still exists", job.ID)
	}
	if job.Status != JobStatusSuccessful {
		t.Errorf("unexpected job status: %v", job.Status)
	}
}

func TestJobManager_StartAndWaitJob_Failed(t *testing.T) {
	job := &Job{
		ID:             1,
		finishedCh:     make(chan struct{}),
		Package:        &Package{},
		TestBinaryPath: "/bin/not/exist",
	}
	for _, t := range []string{"Test1"} {
		job.Tasks = append(job.Tasks, &Task{TestFunction: t, Job: job})
	}

	manager := NewJobManager()
	manager.StartJob(context.Background(), job, 1, &bytes.Buffer{})

	if err := manager.WaitJob(job.ID); err != nil {
		t.Fatal(err)
	}
	if job.Status != JobStatusFailed {
		t.Errorf("unexpected job status: %v", job.Status)
	}
}

func TestJobManager_TestResultHandler(t *testing.T) {
	m := NewJobManager()
	job := &Job{finishedCh: make(chan struct{}), testResultCh: make(chan TestResult), Tasks: []*Task{{TestFunction: "Test1"}}}
	buff := &bytes.Buffer{}
	output := "--- PASS: Test1 (0.01s)\n"
	go func() {
		res := TestResult{TestName: "Test1", Successful: true, Output: []string{output}}
		job.testResultCh <- res
		close(job.finishedCh)
	}()

	m.testResultHandler(job, buff)
	if !strings.Contains(buff.String(), output) {
		t.Errorf("invalid output: %s", buff.String())
	}

	if job.Tasks[0].Status != TaskStatusSuccessful {
		t.Errorf("invalid status: %v", job.Tasks[0].Status)
	}

	if job.Tasks[0].ElapsedTime != 10*time.Millisecond {
		t.Errorf("invalid time: %v", job.Tasks[0].ElapsedTime)
	}
}

func TestJobManager_HandleImportantTestFirst(t *testing.T) {
	m := NewJobManager()
	job := &Job{
		finishedCh:   make(chan struct{}),
		testResultCh: make(chan TestResult),
		Tasks: []*Task{
			{TestFunction: "Test1"},
			{TestFunction: "Test2", Important: true},
		},
	}

	buff := &bytes.Buffer{}
	go func() {
		job.testResultCh <- TestResult{TestName: "Test1", Successful: true, Output: []string{"Test1\n"}}
		job.testResultCh <- TestResult{TestName: "Test2", Successful: true, Output: []string{"Test2\n"}}
		close(job.finishedCh)
	}()

	m.testResultHandler(job, buff)
	if strings.Index(buff.String(), "Test2\n") > strings.Index(buff.String(), "Test1\n") {
		t.Errorf("invalid output: %s", buff.String())
	}
}
