package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkerManager_AddWorker(t *testing.T) {
	manager := &WorkerManager{ServerAddress: "host.docker.internal:48059"}

	orgWGName := workerGroupName
	workerGroupName = "test"
	orgPath := os.Getenv("PATH")
	os.Setenv("PATH", "testdata"+string(filepath.ListSeparator)+orgPath)
	defer func() {
		manager.RemoveWorkers()
		workerGroupName = orgWGName
		os.Setenv("PATH", orgPath)
	}()

	err := manager.AddWorker("", "alpine:3.11")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(manager.Workers) != 1 {
		t.Fatalf("invalid size of workers: %d", len(manager.Workers))
	}
	w := manager.Workers[0]
	if w.ID != 0 {
		t.Errorf("invalid id: %d", w.ID)
	}
	if w.Name != "hornet-worker-test-000" {
		t.Errorf("invalid name: %s", w.Name)
	}

	cmd := exec.Command("docker", "logs", w.Name)
	out, _ := cmd.CombinedOutput()
	if strings.TrimSpace(string(out)) != "--addr host.docker.internal:48059 --debug test 0" {
		t.Errorf("invalid output: %s", string(out))
	}
}

func TestWorkerManager_AddWorker_NoWorkerBin(t *testing.T) {
	manager := &WorkerManager{WorkerBinPath: "/file/not/exist"}

	err := manager.AddWorker("", "alpine:3.11")
	if err == nil {
		t.Fatalf("nil error")
	}
}
