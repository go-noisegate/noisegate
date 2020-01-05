package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestHandleTest(t *testing.T) {
	manager := NewManager()
	server := NewHornetServer("", sharedDir, manager)

	_, filename, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(filename), "testdata")
	req := httptest.NewRequest("GET", "/test", strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	go func() {
		var job *Job
		var taskSet *TaskSet
		for {
			var err error
			job, taskSet, err = manager.NextTaskSet(0)
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		if err := manager.ReportResult(job.ID, taskSet.ID, true, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}()
	server.handleTest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, err := ioutil.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "successful\n" {
		t.Errorf("unexpected content: %s", string(out))
	}
}

func TestHandleTest_EmptyBody(t *testing.T) {
	manager := NewManager()
	server := NewHornetServer("", "", manager)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	server.handleTest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}
