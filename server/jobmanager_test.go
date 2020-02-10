package server

import (
	"testing"
)

func TestJobManager_Partition(t *testing.T) {
	job := &Job{ID: 1}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job}, &Task{TestFunction: "TestFunc2", Job: job})

	manager := NewJobManager()
	if err := manager.Partition(job, 2); err != nil {
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
	if err := manager.Partition(job, 0); err == nil {
		t.Fatal("nil error")
	}
}

func TestJobManager_Partition_AffectedTasks(t *testing.T) {
	job := &Job{ID: 1}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job, Affected: true}, &Task{TestFunction: "TestFunc2", Job: job, Affected: false})

	manager := NewJobManager()
	if err := manager.Partition(job, 2); err != nil {
		t.Fatal(err)
	}
	if len(job.TaskSets) != 4 {
		t.Errorf("wrong number of task sets: %d", len(job.TaskSets))
	}
	if !job.TaskSets[0].Tasks[0].Affected {
		t.Error("affected task should come first")
	}
}

func TestJobManager_AddJob(t *testing.T) {
	job := &Job{
		ID:         1,
		finishedCh: make(chan struct{}),
	}
	numTaskSets := 2
	for i := 0; i < numTaskSets; i++ {
		ts := NewTaskSet(i, job.ID)
		ts.Tasks = []*Task{&Task{}}
		job.TaskSets = append(job.TaskSets, ts)
	}

	manager := NewJobManager()
	manager.AddJob(job)
	if manager.scheduler.Size() != numTaskSets {
		t.Errorf("wrong size: %d", manager.scheduler.Size())
	}
	if _, ok := manager.jobs[job.ID]; !ok {
		t.Errorf("job is not stored: %d", job.ID)
	}
}

func TestJobManager_AddJob_NoTasks(t *testing.T) {
	job := &Job{ID: 1, finishedCh: make(chan struct{})}

	manager := NewJobManager()
	manager.AddJob(job)
	if manager.scheduler.Size() != 0 {
		t.Errorf("wrong size: %d", manager.scheduler.Size())
	}
	if job.Status != JobStatusSuccessful {
		t.Errorf("wrong status: %v", job.Status)
	}
}
