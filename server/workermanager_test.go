package server

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkerManager_AddWorker(t *testing.T) {
	manager := &WorkerManager{WorkerGroupName: "g1", ServerAddress: "localhost:48059"}

	orgPath := os.Getenv("PATH")
	os.Setenv("PATH", "testdata"+string(filepath.ListSeparator)+orgPath)
	defer func() {
		os.Setenv("PATH", orgPath)
	}()

	if err := manager.AddWorker(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(manager.workers) != 1 {
		t.Fatalf("invalid size of workers: %d", len(manager.workers))
	}
	w := manager.workers[0]
	if w.ID != 0 {
		t.Errorf("invalid id: %d", w.ID)
	}
	if w.Name != "worker-g1-000" {
		t.Errorf("invalid name: %s", w.Name)
	}
	if w.LogPath != filepath.Join(sharedDir, "log", "worker", w.Name) {
		t.Errorf("invalid log path: %s", w.LogPath)
	}

	if _, err := w.Process.Wait(); err != nil {
		t.Fatalf("failed to wait: %w", err)
	}
	defer os.Remove(w.LogPath)
	out, _ := ioutil.ReadFile(w.LogPath)
	if strings.TrimSpace(string(out)) != "--addr localhost:48059 --debug g1 0" {
		t.Errorf("invalid log: %s", string(out))
	}
}

func TestWorkerManager_AddWorker_SpecifyWorkerBin(t *testing.T) {
	manager := &WorkerManager{
		WorkerGroupName: "g1",
		ServerAddress:   "localhost:48059",
		WorkerBinPath:   filepath.Join("testdata", workerBinName),
	}
	defer manager.RemoveWorkers()

	if err := manager.AddWorker(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkerManager_AddWorker_NoWorkerBin(t *testing.T) {
	manager := &WorkerManager{WorkerBinPath: "/file/not/exist"}

	err := manager.AddWorker()
	if err == nil {
		t.Fatalf("nil error")
	}
}
