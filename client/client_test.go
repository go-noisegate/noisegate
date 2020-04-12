package client_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
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

func TestTestAction_Offsets(t *testing.T) {
	mux := http.NewServeMux()
	var req common.TestRequest
	mux.HandleFunc(common.TestPath, func(w http.ResponseWriter, r *http.Request) {
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
		options := client.TestOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://"), TestLogger: &strings.Builder{}}
		err := client.TestAction(context.Background(), query, options)
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

		if !reflect.DeepEqual(testdata.expect, req.Ranges) {
			t.Errorf("unexpected ranges: %#v", req.Ranges)
		}
	}
}

func TestHintAction(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc(common.HintPath, func(w http.ResponseWriter, r *http.Request) {})
	server := httptest.NewServer(mux)

	query := "/path/to/test/file:#1"
	options := client.HintOptions{ServerAddr: strings.TrimPrefix(server.URL, "http://")}
	err := client.HintAction(context.Background(), query, options)
	if err != nil {
		t.Error(err)
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
