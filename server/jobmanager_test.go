package server

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJobManager_Partition(t *testing.T) {
	job := &Job{ID: 1, EnableParallel: true}
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
	job := &Job{ID: 1, EnableParallel: true}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job}, &Task{TestFunction: "TestFunc2", Job: job})

	manager := NewJobManager()
	if err := manager.partition(job, 0); err == nil {
		t.Fatal("nil error")
	}
}

func TestJobManager_Partition_ParallelDisabled(t *testing.T) {
	job := &Job{ID: 1, EnableParallel: false}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1"}, &Task{TestFunction: "TestFunc2", Important: true})

	manager := NewJobManager()
	if err := manager.partition(job, 2); err != nil {
		t.Fatal(err)
	}
	if len(job.TaskSets) != 1 {
		t.Errorf("wrong number of task sets: %#v", job.TaskSets)
	}
}

func TestJobManager_Partition_ImportantTasks(t *testing.T) {
	job := &Job{ID: 1, EnableParallel: true}
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
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata")

	job, err := NewJob(&Package{path: dirPath}, []change{{filepath.Join(dirPath, "sum.go"), 0, 0}}, true, "")
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}

	manager := NewJobManager()
	manager.profiler.Add(job.DirPath, "TestSum", time.Hour) // to check the profiler is updated

	var buff bytes.Buffer
	manager.StartJob(context.Background(), job, 2, &buff)
	if _, ok := manager.jobs[job.ID]; !ok {
		t.Errorf("job is not stored: %d", job.ID)
	}
	if len(job.TaskSets) != 2 {
		t.Fatalf("wrong number of task sets: %d", len(job.TaskSets))
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
	if manager.profiler.ExpectExecTime(job.DirPath, "TestSum") == time.Hour {
		t.Errorf("profiler is not updated: %v", manager.profiler.ExpectExecTime(job.DirPath, "TestSum"))
	}
}

func TestJobManager_StartAndWaitJob_Failed(t *testing.T) {
	job := &Job{
		ID:             1,
		jobFinishedCh:  make(chan struct{}),
		testEventCh:    make(chan TestEvent),
		Package:        &Package{},
		TestBinaryPath: "/bin/not/exist",
		EnableParallel: true,
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

func TestEventHandler_HandleResult(t *testing.T) {
	var buff bytes.Buffer
	task := &Task{TestFunction: "Test1"}
	h := newEventHandler(&Job{Tasks: []*Task{task}}, &buff)
	result := TestResult{TestName: "Test1", Output: []string{"output\n", "--- PASS: Test1 (0.01s)\n"}, Successful: true}
	h.handleResult(result)
	if buff.String() != strings.Join(result.Output, "") {
		t.Errorf("wrong content: %s", buff.String())
	}

	if task.Status != TaskStatusSuccessful {
		t.Errorf("wrong status: %v", task.Status)
	}
	if task.ElapsedTime != 10*time.Millisecond {
		t.Errorf("wrong elapsed time: %v", task.ElapsedTime)
	}
}

func TestEventHandler_HandleResultWithBuffer(t *testing.T) {
	var buff bytes.Buffer
	importantTask := &Task{TestFunction: "TestImportant", Important: true}
	notImportantTask := &Task{TestFunction: "TestNotImportant", Important: false}
	h := newEventHandler(&Job{Tasks: []*Task{importantTask, notImportantTask}}, &buff)

	h.handleResultWithBuffer(TestResult{
		TestName:   "TestNotImportant",
		Output:     []string{"not important\n"},
		Successful: true,
	})
	h.handleResultWithBuffer(TestResult{
		TestName:   "TestImportant",
		Output:     []string{"important\n"},
		Successful: true,
	})

	if buff.String() != "important\n\nRun other tests:\nnot important\n" {
		t.Errorf("wrong content: %s", buff.String())
	}

	if importantTask.Status != TaskStatusSuccessful {
		t.Errorf("wrong status: %v", importantTask.Status)
	}
	if notImportantTask.Status != TaskStatusSuccessful {
		t.Errorf("wrong status: %v", notImportantTask.Status)
	}
}

func TestEventHandler_Handle(t *testing.T) {
	var buff bytes.Buffer
	task := &Task{TestFunction: "TestSum"}
	job := &Job{Tasks: []*Task{task}, EnableParallel: false}
	h := newEventHandler(job, &buff)
	for _, ev := range []TestEvent{
		{Action: "run", Test: "TestSum"},
		{Action: "output", Test: "TestSum", Output: "=== RUN   TestSum\n"},
		{Action: "output", Test: "TestSum", Output: "--- PASS: TestSum (0.01s)\n"},
		{Action: "pass", Test: "TestSum", Elapsed: 0.02},
		{Action: "output", Output: "PASS\n"},
		{Action: "output", Output: "ok \n"},
		{Action: "pass", Elapsed: 0.02},
	} {
		h.handle(ev)
	}
	if buff.String() != "=== RUN   TestSum\n--- PASS: TestSum (0.01s)\n" {
		t.Errorf("wrong content: %s", buff.String())
	}
	if task.Status != TaskStatusSuccessful {
		t.Errorf("wrong status: %v", task.Status)
	}
	if task.ElapsedTime != 10*time.Millisecond {
		t.Errorf("wrong elapsed time: %v", task.ElapsedTime)
	}
	if job.ElapsedTestTime != 20*time.Millisecond {
		t.Errorf("wrong job elapsed time: %v", job.ElapsedTestTime)
	}
}

func TestEventHandler_Handle_MergeInnerTest(t *testing.T) {
	var buff bytes.Buffer
	task := &Task{TestFunction: "TestSum"}
	h := newEventHandler(&Job{Tasks: []*Task{task}}, &buff)
	for _, ev := range []TestEvent{
		{Action: "run", Test: "TestSum"},
		{Action: "output", Test: "TestSum", Output: "=== RUN   TestSum\n"},
		{Action: "run", Test: "TestSum/Case1"},
		{Action: "output", Test: "TestSum/Case1", Output: "=== RUN   TestSum/Case1\n"},
		{Action: "output", Test: "TestSum", Output: "--- PASS: TestSum (0.02s)\n"},
		{Action: "output", Test: "TestSum/Case1", Output: "--- PASS: TestSum/Case1 (0.01s)\n"},
		{Action: "pass", Test: "TestSum/Case1", Elapsed: 0.01},
		{Action: "pass", Test: "TestSum", Elapsed: 0.02},
	} {
		h.handle(ev)
	}
	if buff.String() != "=== RUN   TestSum\n=== RUN   TestSum/Case1\n--- PASS: TestSum (0.02s)\n--- PASS: TestSum/Case1 (0.01s)\n" {
		t.Errorf("wrong content: %s", buff.String())
	}
	if task.Status != TaskStatusSuccessful {
		t.Errorf("wrong status: %v", task.Status)
	}
	if task.ElapsedTime != 20*time.Millisecond {
		t.Errorf("wrong elapsed time: %v", task.ElapsedTime)
	}
}
