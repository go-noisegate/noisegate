package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Worker struct {
	ctx                                  context.Context
	testFuncs                            []string
	testBinaryPath, packagePath, logPath string
	cmd                                  *exec.Cmd
}

// NewWorker returns the worker which executes the task set.
func NewWorker(ctx context.Context, job *Job, taskSet *TaskSet) *Worker {
	var testFuncs []string
	for _, t := range taskSet.Tasks {
		testFuncs = append(testFuncs, t.TestFunction)
	}

	return &Worker{
		ctx:            ctx,
		testFuncs:      testFuncs,
		testBinaryPath: job.TestBinaryPath,
		packagePath:    job.Package.path,
		logPath:        taskSet.LogPath,
	}
}

// Start starts the new test.
func (w *Worker) Start() error {
	if len(w.testFuncs) == 0 {
		return nil
	}

	w.cmd = exec.CommandContext(w.ctx, w.testBinaryPath, "-test.v", "-test.run", "^"+strings.Join(w.testFuncs, "$|^")+"$")
	w.cmd.Dir = w.packagePath

	logFile, err := os.OpenFile(w.logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open the log file %s: %w\n", w.logPath, err)
	}
	w.cmd.Stdout = logFile
	w.cmd.Stderr = logFile
	if err := w.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start the test: %w", err)
	}
	return nil
}

// Wait waits until the test finishes.
func (w *Worker) Wait() (bool, error) {
	if w.cmd == nil {
		return true, nil
	}
	err := w.cmd.Wait()
	return err == nil, err
}
