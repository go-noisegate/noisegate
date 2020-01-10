package server

import "testing"

func TestWorkerManager_AddWorker(t *testing.T) {
	manager := &WorkerManager{}

	orgWGName := workerGroupName
	workerGroupName = "test"
	defer func() {
		manager.RemoveWorkers()
		workerGroupName = orgWGName
	}()

	err := manager.AddWorker("", "alpine:3.11")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
