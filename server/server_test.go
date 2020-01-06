package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestHandleTest_Depth0(t *testing.T) {
	manager := NewManager()
	server := NewHornetServer("", sharedDir, manager)

	_, filename, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(filename), "testdata")
	req := httptest.NewRequest("GET", "/test", strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	go func() {
		executeTaskSet(t, manager)
	}()
	server.handleTest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	matched, _ := regexp.Match(`(?m)^=== PASS \(job: \d+, task set: 0, path: `+path+`\)$`, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
	matched, _ = regexp.Match(`(?m)^ok$`, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
}

func TestHandleTest_Depth1(t *testing.T) {
	manager := NewManager()
	server := NewHornetServer("", sharedDir, manager)
	server.depthLimit = 1

	_, filename, _, _ := runtime.Caller(0)
	// Importgraph builder ignores testdata dir
	path := filepath.Dir(filename)
	req := httptest.NewRequest("GET", "/test", strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	go func() {
		executeTaskSet(t, manager)
		executeTaskSet(t, manager)
	}()
	server.handleTest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	matched, _ := regexp.Match(`(?m)^=== PASS \(job: \d+, task set: 0, path: `+path+`\)$`, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
	matched, _ = regexp.Match(`(?m)^=== PASS \(job: \d+, task set: 0, path: `+filepath.Dir(path)+`/cmd/hornetd\)$`, out)
	if !matched {
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

func executeTaskSet(t *testing.T, manager *Manager) {
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

	if err := manager.ReportResult(job.ID, taskSet.ID, true, []byte("ok")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
