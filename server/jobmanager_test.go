package server

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestJobManager_StartAndWaitJob(t *testing.T) {
	currDir, _ := os.Getwd()
	dirPath := filepath.Join(currDir, "testdata")

	var buff bytes.Buffer
	job, err := NewJob(dirPath, []change{{filepath.Join(dirPath, "sum.go"), 0, 0}}, "", &buff)
	if err != nil {
		t.Fatalf("failed to create new job: %v", err)
	}

	manager := NewJobManager()

	manager.StartJob(context.Background(), job)
	if _, ok := manager.jobs[job.ID]; !ok {
		t.Errorf("job is not stored: %d", job.ID)
	}
	if len(job.TaskSets) != 1 {
		t.Fatalf("wrong number of task sets: %d", len(job.TaskSets))
	}
	if job.TaskSets[0].Worker == nil {
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
