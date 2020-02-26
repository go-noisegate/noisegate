package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ks888/hornet/common"
)

func TestHandleHint(t *testing.T) {
	server := NewHornetServer("", 1)

	curr, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	path := filepath.Join(curr, "testdata", "sum.go")
	req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	server.handleHint(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	if string(out) != "accepted\n" {
		t.Errorf("unexpected response body: %s", string(out))
	}

	if changes := server.changeManager.Find(filepath.Dir(path)); len(changes) != 1 {
		t.Errorf("wrong changes: %#v", changes)
	}
}

func TestHandleHint_InvalidJSON(t *testing.T) {
	server := NewHornetServer("", 1)

	req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(`{`))
	w := httptest.NewRecorder()
	server.handleHint(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleHint_RelativePath(t *testing.T) {
	server := NewHornetServer("", 1)

	req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(`{"path": "rel/path"}`))
	w := httptest.NewRecorder()
	server.handleHint(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleHint_PathNotFound(t *testing.T) {
	server := NewHornetServer("", 1)

	req := httptest.NewRequest("GET", common.HintPath, strings.NewReader(`{"path": "/path/to/not/exist/file"}`))
	w := httptest.NewRecorder()
	server.handleHint(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleTest_InputIsFile(t *testing.T) {
	server := NewHornetServer("", 1)

	curr, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	path := filepath.Join(curr, "testdata", "sum.go")
	req := httptest.NewRequest("GET", common.TestPath, strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	server.handleTest(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	matched, _ := regexp.Match(`(?m)^PASS \(`, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
}

func TestHandleTest_InputIsDir(t *testing.T) {
	server := NewHornetServer("", 1)

	curr, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	path := filepath.Join(curr, "testdata")
	req := httptest.NewRequest("GET", common.TestPath, strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	server.handleTest(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	matched, _ := regexp.Match(`(?m)^PASS \(`, out)
	if !matched {
		t.Errorf("unexpected content: %s", string(out))
	}
}

func TestHandleTest_EmptyBody(t *testing.T) {
	server := NewHornetServer("", 1)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	server.handleTest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleTest_RelativePath(t *testing.T) {
	server := NewHornetServer("", 1)

	req := httptest.NewRequest("GET", "/test", strings.NewReader(`{"path": "rel/path"}`))
	w := httptest.NewRecorder()
	server.handleTest(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHornetServer_Parallel(t *testing.T) {
	server := NewHornetServer("", 1)
	path := "/path/to/dir"
	if res := server.parallel(path, common.ParallelOn); !res {
		t.Errorf("wrong result: %v", res)
	}

	if res := server.parallel(path, common.ParallelOff); res {
		t.Errorf("wrong result: %v", res)
	}

	// no last result
	if res := server.parallel(path, common.ParallelAuto); !res {
		t.Errorf("wrong result: %v", res)
	}

	// slow test
	server.jobProfiler.Add(path, time.Second)
	if res := server.parallel(path, common.ParallelAuto); !res {
		t.Errorf("wrong result: %v", res)
	}

	// fast test
	server.jobProfiler.Add(path, time.Millisecond)
	if res := server.parallel(path, common.ParallelAuto); res {
		t.Errorf("wrong result: %v", res)
	}
}
