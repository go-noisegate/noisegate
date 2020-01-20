package server

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ks888/hornet/common/log"
)

// For the testing purpose, we allow multiple sets of workers.
var workerGroupName = "default"

const workerBinName = "hornet-worker"

// WorkerManager manages the workers.
type WorkerManager struct {
	ServerAddress string // the hornetd server address usable inside container
	WorkerBinPath string // if empty, search the PATH directories
	Workers       []Worker
}

// AddWorker starts a new worker. `host` specifies daemon socket(s) to connect to. If `host` is empty,
// the default docker daemon is used.
func (m *WorkerManager) AddWorker(host, image string) error {
	workerBinPath, err := m.findBinPath()
	if err != nil {
		return fmt.Errorf("failed to find the %s command: %w", workerBinName, err)
	}

	workerID := len(m.Workers)
	workerName := fmt.Sprintf("hornet-worker-%s-%03d", workerGroupName, workerID)

	var commonArgs []string
	if host != "" {
		commonArgs = append(commonArgs, "--host", host)
	}

	// TODO: need `--restart always` option?
	createArgs := append(commonArgs, "create", "--volume", sharedDir+":"+sharedDirOnContainer, "--name", workerName, image, workerBinName, "--addr", m.ServerAddress)
	if log.DebugLogEnabled() {
		createArgs = append(createArgs, "--debug")
	}
	createArgs = append(createArgs, workerGroupName, strconv.Itoa(workerID))
	cmd := exec.Command("docker", createArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create container: %w\n%s", err, string(out))
	}
	containerID := strings.TrimSpace(string(out))

	cpArgs := append(commonArgs, "cp", workerBinPath, containerID+":/usr/bin/"+workerBinName)
	cmd = exec.Command("docker", cpArgs...)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy binary: %v\n%s", err, string(out))
	}

	startArgs := append(commonArgs, "start", containerID)
	cmd = exec.Command("docker", startArgs...)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run the container: %w\noutput:\n%s", err, string(out))
	}

	m.Workers = append(m.Workers, Worker{workerID, workerName, host, image})
	return nil
}

// TODO: leave the doc to explain the logic here.
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
	for _, w := range m.Workers {
		for _, dockerCmd := range []string{"stop", "rm"} {
			var args []string
			if w.Host != "" {
				args = append(args, "--host", w.Host)
			}
			args = append(args, dockerCmd, w.Name)
			cmd := exec.Command("docker", args...)
			out, err := cmd.CombinedOutput()
			if err != nil {
				log.Debugf("failed to %s the container %s: %v\noutput:\n%s", dockerCmd, w.Name, err, string(out))
				break
			}
		}
	}
}

// Worker represents one worker.
type Worker struct {
	// This id is unique only among the worker group.
	ID    int
	Name  string
	Host  string
	Image string
}
