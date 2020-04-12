package server

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type Worker struct {
	testFuncs   []string
	packagePath string
	buildTags   string
	writer      io.Writer
	cmd         *exec.Cmd
}

// NewWorker returns the worker which executes the task set.
func NewWorker(job *Job, taskSet *TaskSet) *Worker {
	var testFuncs []string
	for _, t := range taskSet.Tasks {
		testFuncs = append(testFuncs, t.TestFunction)
	}

	return &Worker{
		testFuncs:   testFuncs,
		packagePath: job.DirPath,
		buildTags:   job.BuildTags,
		writer:      job.writer,
	}
}

// Start starts the new test.
func (w *Worker) Start(ctx context.Context) error {
	if len(w.testFuncs) == 0 {
		return nil
	}

	w.cmd = exec.CommandContext(ctx, "go", "test", "-tags", w.buildTags, "-v", "-run", "^"+strings.Join(w.testFuncs, "$|^")+"$")
	w.cmd.Dir = w.packagePath

	w.cmd.Stdout = w.writer
	w.cmd.Stderr = w.writer
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
