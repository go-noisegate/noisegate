package server

import (
	"testing"
)

func TestJobManager_AddJob(t *testing.T) {
	job := &Job{ID: 1, finishedCh: make(chan struct{})}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job})

	manager := NewJobManager()
	manager.AddJob(job)
	if len(job.TaskSets) != 1 {
		t.Errorf("wrong number of task sets: %d", len(job.TaskSets))
	}
	if manager.scheduler.Size() != 1 {
		t.Errorf("wrong size: %d", manager.scheduler.Size())
	}
	if _, ok := manager.jobs[job.ID]; !ok {
		t.Errorf("job is not stored: %d", job.ID)
	}
}

func TestJobManager_AddJob_NoTasks(t *testing.T) {
	job := &Job{ID: 1, finishedCh: make(chan struct{}), Repository: NewSyncedRepository("/path/to/file")}

	manager := NewJobManager()
	manager.AddJob(job)
	if job.Status != JobStatusSuccessful {
		t.Errorf("wrong status: %v", job.Status)
	}
}
