package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/ks888/hornet/common/log"
)

const workerBinName = "hornet-worker"

// WorkerManager manages the workers.
type WorkerManager struct {
	ServerAddress   string // the hornetd server address usable inside container
	WorkerBinPath   string
	WorkerGroupName string
	workers         []Worker
	mtx             sync.Mutex
}

// AddWorker starts a new worker.
// To find the worker binary, `WorkerBinPath` is used if it's not empty. Otherwise, search the PATH directories.
func (m *WorkerManager) AddWorker() error {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	workerBinPath, err := m.findBinPath()
	if err != nil {
		return fmt.Errorf("failed to find the %s command: %w", workerBinName, err)
	}

	workerID := len(m.workers)
	workerName := fmt.Sprintf("worker-%s-%03d", m.WorkerGroupName, workerID)
	logPath := filepath.Join(sharedDir, "log", "worker", workerName)
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open the log file: %w", err)
	}

	args := []string{"--addr", m.ServerAddress}
	if log.DebugLogEnabled() {
		args = append(args, "--debug")
	}
	args = append(args, m.WorkerGroupName, strconv.Itoa(workerID))
	cmd := exec.Command(workerBinPath, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to run the worker: %w", err)
	}

	m.workers = append(m.workers, Worker{ID: workerID, Name: workerName, LogPath: logPath, Process: cmd.Process})
	return nil
}

func (m *WorkerManager) findBinPath() (string, error) {
	if m.WorkerBinPath != "" {
		if _, err := os.Stat(m.WorkerBinPath); os.IsNotExist(err) {
			return "", fmt.Errorf("%s not exist", m.WorkerBinPath)
		}
		return m.WorkerBinPath, nil
	}

	workerBinPath, err := exec.LookPath(workerBinName)
	if err != nil {
		return "", fmt.Errorf("failed to find the %s command: %w", workerBinName, err)
	}
	return workerBinPath, nil
}

// RemoveWorkers stops and removes all the worker containers.
func (m *WorkerManager) RemoveWorkers() {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	for _, w := range m.workers {
		if err := w.Process.Kill(); err != nil {
			log.Printf("failed to kill the process: %v", err)
		}
	}
}

// NumWorkers returns the number of workers.
func (m *WorkerManager) NumWorkers() int {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	return len(m.workers)
}

// Worker represents one worker.
type Worker struct {
	// This id is unique only among the worker group.
	ID      int
	Name    string
	LogPath string
	Process *os.Process
}
