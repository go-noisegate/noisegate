package server

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	job := &Job{ID: 1, finishedCh: make(chan struct{})}

	manager := NewJobManager()
	manager.AddJob(job)
	if job.Status != JobStatusSuccessful {
		t.Errorf("wrong status: %v", job.Status)
	}
}

func TestJobServer_HandleNextTaskSet(t *testing.T) {
	manager := NewJobManager()
	job := &Job{ID: 1, DirPath: "/path/to/dir/", TestBinaryPath: "/path/to/binary", RepoArchivePath: "/path/to/archive", finishedCh: make(chan struct{})}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job})
	manager.AddJob(job)

	server := NewJobServer("", manager)
	req := httptest.NewRequest(http.MethodGet, nextTaskSetPath, strings.NewReader(`{"worker_id": 1}`))
	resp := httptest.NewRecorder()
	server.handleNextTaskSet(resp, req)
	if resp.Code != http.StatusOK {
		t.Errorf("unexpected status: %d", resp.Code)
	}

	decodedResp := &nextTaskSetResponse{}
	if err := json.Unmarshal(resp.Body.Bytes(), decodedResp); err != nil {
		t.Fatalf("failed to unmarshal resp body: %v", err)
	}
	taskSet := job.TaskSets[0]
	if decodedResp.TaskSetID != taskSet.ID {
		t.Errorf("unexpected task set id: %d", decodedResp.TaskSetID)
	}
	if decodedResp.JobID != job.ID {
		t.Errorf("unexpected job id: %d", decodedResp.JobID)
	}
	if len(decodedResp.TestFunctions) != 1 || decodedResp.TestFunctions[0] != "TestFunc1" {
		t.Errorf("unexpected test functions: %v", decodedResp.TestFunctions)
	}
	if decodedResp.DirPath != "/path/to/dir/" {
		t.Errorf("unexpected dir path: %s", decodedResp.DirPath)
	}
	if decodedResp.TestBinaryPath != "/path/to/binary" {
		t.Errorf("unexpected test binary path: %s", decodedResp.TestBinaryPath)
	}
	if decodedResp.RepoArchivePath != "/path/to/archive" {
		t.Errorf("unexpected test binary path: %s", decodedResp.TestBinaryPath)
	}

	if taskSet.Status != TaskSetStatusStarted {
		t.Errorf("unexpected task set status: %v", taskSet.Status)
	}
}

func TestJobServer_HandleNextTaskSet_NoTaskSet(t *testing.T) {
	manager := NewJobManager()
	server := NewJobServer("", manager)
	req := httptest.NewRequest(http.MethodGet, nextTaskSetPath, strings.NewReader(`{}`))
	resp := httptest.NewRecorder()
	server.handleNextTaskSet(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}

func TestJobServer_HandleNextTaskSet_EmptyTaskSet(t *testing.T) {
	manager := NewJobManager()
	emptyJob := &Job{ID: 1, finishedCh: make(chan struct{})}
	manager.AddJob(emptyJob)

	job := &Job{ID: 2, finishedCh: make(chan struct{})}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestFunc1", Job: job})
	manager.AddJob(job)

	server := NewJobServer("", manager)
	req := httptest.NewRequest(http.MethodGet, nextTaskSetPath, strings.NewReader(`{}`))
	resp := httptest.NewRecorder()
	server.handleNextTaskSet(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("unexpected status: %d", resp.Code)
	}
	decodedResp := &nextTaskSetResponse{}
	if err := json.Unmarshal(resp.Body.Bytes(), decodedResp); err != nil {
		t.Fatalf("failed to unmarshal resp body: %v", err)
	}
	if decodedResp.JobID != 2 {
		t.Errorf("unexpected job id: %d", decodedResp.JobID)
	}
}

func TestJobServer_HandleNextTaskSet_InvalidReqBody(t *testing.T) {
	manager := NewJobManager()
	server := NewJobServer("", manager)
	req := httptest.NewRequest(http.MethodGet, nextTaskSetPath, strings.NewReader(`{`))
	resp := httptest.NewRecorder()
	server.handleNextTaskSet(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}

func TestJobServer_HandleReportResult(t *testing.T) {
	job := &Job{ID: 1, finishedCh: make(chan struct{})}
	job.Tasks = append(job.Tasks, &Task{TestFunction: "TestJobManager_ReportResult", Job: job})
	manager := NewJobManager()
	manager.AddJob(job)

	logContent := "=== RUN   TestJobManager_ReportResult\n--- PASS: TestJobManager_ReportResult (1.00s)"
	err := ioutil.WriteFile(filepath.Join(sharedDir, job.TaskSets[0].LogPath), []byte(logContent), 0644)
	if err != nil {
		t.Fatalf("failed to write log %s: %v", job.TaskSets[0].LogPath, err)
	}

	server := NewJobServer("", manager)
	reqBody := reportResultRequest{
		JobID:      1,
		TaskSetID:  0,
		Successful: true,
	}
	data, _ := json.Marshal(&reqBody)
	req := httptest.NewRequest(http.MethodGet, reportResultPath, bytes.NewReader(data))
	resp := httptest.NewRecorder()
	server.handleReportResult(resp, req)
	if resp.Code != http.StatusOK {
		t.Errorf("unexpected status: %d", resp.Code)
	}

	taskSet := job.TaskSets[0]
	if taskSet.Status != TaskSetStatusSuccessful {
		t.Errorf("unexpected status: %v", taskSet.Status)
	}
	if job.Status != JobStatusSuccessful {
		t.Errorf("unexpected status: %v", job.Status)
	}
	if _, ok := manager.jobs[job.ID]; ok {
		t.Errorf("the job is not removed")
	}
	if job.Tasks[0].Status != TaskStatusSuccessful {
		t.Errorf("unexpected status: %v", job.Tasks[0].Status)
	}
	if job.Tasks[0].ElapsedTime != time.Second {
		t.Errorf("unexpected elapsed time: %v", job.Tasks[0].ElapsedTime)
	}
}

func TestJobServer_HandleReportResult_JobNotFound(t *testing.T) {
	manager := NewJobManager()
	server := NewJobServer("", manager)
	reqBody := reportResultRequest{JobID: 1, TaskSetID: 0, Successful: true}
	data, _ := json.Marshal(&reqBody)
	req := httptest.NewRequest(http.MethodGet, nextTaskSetPath, bytes.NewReader(data))
	resp := httptest.NewRecorder()
	server.handleReportResult(resp, req)
	if resp.Code != http.StatusInternalServerError {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}

func TestJobServer_HandleReportResult_InvalidJSON(t *testing.T) {
	manager := NewJobManager()
	server := NewJobServer("", manager)

	req := httptest.NewRequest(http.MethodGet, nextTaskSetPath, strings.NewReader("{"))
	resp := httptest.NewRecorder()
	server.handleReportResult(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("unexpected status: %d", resp.Code)
	}
}
