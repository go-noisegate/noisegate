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

	"github.com/ks888/hornet/common"
)

func TestHandleSetup(t *testing.T) {
	server := NewHornetServer("", 1)

	curr, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get wd: %v", err)
	}
	path := filepath.Join(curr, "testdata", "sum.go")
	req := httptest.NewRequest("GET", common.SetupPath, strings.NewReader(fmt.Sprintf(`{"path": "%s"}`, path)))
	w := httptest.NewRecorder()
	server.handleSetup(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("unexpected code: %d", w.Code)
	}

	out, _ := ioutil.ReadAll(w.Body)
	if string(out) != "accepted\n" {
		t.Errorf("unexpected response body: %s", string(out))
	}
}

func TestHandleSetup_InvalidJSON(t *testing.T) {
	server := NewHornetServer("", 1)

	req := httptest.NewRequest("GET", common.SetupPath, strings.NewReader(`{`))
	w := httptest.NewRecorder()
	server.handleSetup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleSetup_RelativePath(t *testing.T) {
	server := NewHornetServer("", 1)

	req := httptest.NewRequest("GET", common.SetupPath, strings.NewReader(`{"path": "rel/path"}`))
	w := httptest.NewRecorder()
	server.handleSetup(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unexpected code: %d", w.Code)
	}
}

func TestHandleSetup_PathNotFound(t *testing.T) {
	server := NewHornetServer("", 1)

	req := httptest.NewRequest("GET", common.SetupPath, strings.NewReader(`{"path": "/path/to/not/exist/file"}`))
	w := httptest.NewRecorder()
	server.handleSetup(w, req)

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
	matched, _ := regexp.Match(`(?m)^PASS: Job#\d+ \(`+filepath.Dir(path)+`\) `, out)
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
	matched, _ := regexp.Match(`(?m)^PASS: Job#\d+ \(`+path+`\) `, out)
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
