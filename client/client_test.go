package client_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ks888/hornet/client"
	"github.com/ks888/hornet/common"
)

func TestTestAction(t *testing.T) {
	out := "ok"
	mux := http.NewServeMux()
	mux.HandleFunc(common.TestPath, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(out))
	})
	server := httptest.NewServer(mux)

	testfile := "/path/to/test/file"
	logger := &strings.Builder{}
	options := client.TestOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://"), TestLogger: logger}
	err := client.TestAction(context.Background(), testfile, options)
	if err != nil {
		t.Error(err)
	}

	if logger.String() != out {
		t.Errorf("unexpected log: %v", logger.String())
	}
}

func TestTestAction_BadRequest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.TestPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})
	server := httptest.NewServer(mux)

	testfile := "/path/to/test/file"
	logger := &strings.Builder{}
	options := client.TestOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://"), TestLogger: logger}
	err := client.TestAction(context.Background(), testfile, options)
	if err == nil {
		t.Errorf("nil error")
	}
}

func TestTestAction_RelativePath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.TestPath, func(w http.ResponseWriter, r *http.Request) {
		data, _ := ioutil.ReadAll(r.Body)
		req := common.TestRequest{}
		if err := json.Unmarshal(data, &req); err != nil {
			t.Errorf("failed to decode: %v", err)
		}
		if !filepath.IsAbs(req.Path) {
			t.Errorf("relative path")
		}
	})
	server := httptest.NewServer(mux)

	testfile := "rel/path"
	logger := &strings.Builder{}
	options := client.TestOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://"), TestLogger: logger}
	err := client.TestAction(context.Background(), testfile, options)
	if err != nil {
		t.Error(err)
	}
}

func TestSetupAction(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.SetupPath, func(w http.ResponseWriter, r *http.Request) {})
	server := httptest.NewServer(mux)

	testfile := "/path/to/test/file"
	options := client.SetupOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://")}
	err := client.SetupAction(context.Background(), testfile, options)
	if err != nil {
		t.Error(err)
	}
}

func TestSetupAction_RelativePath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.SetupPath, func(w http.ResponseWriter, r *http.Request) {
		data, _ := ioutil.ReadAll(r.Body)
		req := common.SetupRequest{}
		if err := json.Unmarshal(data, &req); err != nil {
			t.Errorf("failed to decode: %v", err)
		}
		if !filepath.IsAbs(req.Path) {
			t.Errorf("relative path")
		}
	})
	server := httptest.NewServer(mux)

	testfile := "test/file"
	options := client.SetupOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://")}
	err := client.SetupAction(context.Background(), testfile, options)
	if err != nil {
		t.Error(err)
	}
}
