package server

import (
	"fmt"
	"os/exec"

	"github.com/ks888/hornet/common/log"
)

// For the testing purpose, we allow multiple sets of workers.
var workerGroupName = "default"

// WorkerManager manages the workers.
type WorkerManager struct {
	Workers []Worker
}

// AddWorker starts a new worker. `host` specifies daemon socket(s) to connect to. If `host` is empty,
// the default docker daemon is used.
func (m *WorkerManager) AddWorker(host, image string) error {
	workerID := len(m.Workers)
	workerName := fmt.Sprintf("bee-%s-%03d", workerGroupName, workerID)

	// TODO: workerバイナリを差し込むために、create -> cp -> startの順にする
	var args []string
	if host != "" {
		args = append(args, "--host", host)
	}
	// TODO: 共有ディレクトリのvolumeマウント、workerバイナリの起動コマンド
	args = append(args, "run", "-d", "--name", workerName, image)
	cmd := exec.Command("docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run the container: %w\noutput:\n%s", err, string(out))
	}

	m.Workers = append(m.Workers, Worker{workerID, workerName, host, image})
	return nil
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
