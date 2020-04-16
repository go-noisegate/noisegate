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

	"github.com/ks888/noisegate/client"
	"github.com/ks888/noisegate/common"
)

func TestTestAction(t *testing.T) {
	out := "ok"
	mux := http.NewServeMux()
	mux.HandleFunc(common.TestPath, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(out))
	})
	server := httptest.NewServer(mux)

	testdir := "/path/to/test/dir"
	logger := &strings.Builder{}
	options := client.TestOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://"), TestLogger: logger}
	err := client.TestAction(context.Background(), testdir, options)
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

	testdir := "/path/to/test/dir"
	logger := &strings.Builder{}
	options := client.TestOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://"), TestLogger: logger}
	err := client.TestAction(context.Background(), testdir, options)
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

	testdir := "rel/path"
	logger := &strings.Builder{}
	options := client.TestOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://"), TestLogger: logger}
	err := client.TestAction(context.Background(), testdir, options)
	if err != nil {
		t.Error(err)
	}
}

func TestTestAction_RangeIsSpecified(t *testing.T) {
	server := httptest.NewServer(http.NewServeMux())

	query := "/path/to/test/dir:#1"
	logger := &strings.Builder{}
	options := client.TestOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://"), TestLogger: logger}
	err := client.TestAction(context.Background(), query, options)
	if err == nil {
		t.Error("nil error")
	}
}

func TestHintAction_Offsets(t *testing.T) {
	mux := http.NewServeMux()
	var req common.HintRequest
	mux.HandleFunc(common.HintPath, func(w http.ResponseWriter, r *http.Request) {
		data, _ := ioutil.ReadAll(r.Body)
		_ = json.Unmarshal(data, &req)
	})
	server := httptest.NewServer(mux)

	for _, testdata := range []struct {
		offset string
		expect []common.Range
		err    bool
	}{
		{"#1", []common.Range{{1, 1}}, false},
		{"#1-2", []common.Range{{1, 2}}, false},
		{"#1-2,#3-4", []common.Range{{1, 2}, {3, 4}}, false},
		{"#1,#2", []common.Range{{1, 1}, {2, 2}}, false},
		{"#1,2", []common.Range{{1, 1}, {2, 2}}, false},
		{"#1:#2", nil, true},
		{"#1-", nil, true},
	} {
		query := "/path/to/test/file:" + testdata.offset
		options := client.HintOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://")}
		err := client.HintAction(context.Background(), query, options)
		if err != nil {
			if !testdata.err {
				t.Fatal(err)
			}
			continue
		} else {
			if testdata.err {
				t.Fatal("not error")
			}
		}
	}
}

func TestHintAction_RelativePath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.HintPath, func(w http.ResponseWriter, r *http.Request) {
		data, _ := ioutil.ReadAll(r.Body)
		req := common.HintRequest{}
		if err := json.Unmarshal(data, &req); err != nil {
			t.Errorf("failed to decode: %v", err)
		}
		if !filepath.IsAbs(req.Path) {
			t.Errorf("relative path")
		}
	})
	server := httptest.NewServer(mux)

	testfile := "test/file"
	options := client.HintOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://")}
	err := client.HintAction(context.Background(), testfile, options)
	if err != nil {
		t.Error(err)
	}
}
