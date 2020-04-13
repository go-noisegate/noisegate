package server

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type worker struct {
	testFuncs     []string
	packagePath   string
	goTestOptions []string
	writer        io.Writer
	cmd           *exec.Cmd
}

func newWorker(job *Job, taskSet *TaskSet) *worker {
	var testFuncs []string
	for _, t := range taskSet.Tasks {
		testFuncs = append(testFuncs, t.TestFunction)
	}

	return &worker{
		testFuncs:     testFuncs,
		packagePath:   job.DirPath,
		goTestOptions: job.GoTestOptions,
		writer:        job.writer,
	}
}

// Start starts the new test.
func (w *worker) Start(ctx context.Context) error {
	args := append([]string{"test"}, w.goTestOptions...)
	runOptIndex := findOptionValueIndex(args, "run")
	runOptValue := "^" + strings.Join(w.testFuncs, "$|^") + "$"
	if runOptIndex != -1 {
		args[runOptIndex] += "|" + runOptValue
	} else {
		args = append(args, "-run", runOptValue)
	}

	w.cmd = exec.CommandContext(ctx, "go", args...)
	w.cmd.Dir = w.packagePath
	w.cmd.Stdout = w.writer
	w.cmd.Stderr = w.writer
	if err := w.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start the test: %w", err)
	}
	return nil
}

// Wait waits until the test finishes.
func (w *worker) Wait() (bool, error) {
	if w.cmd == nil {
		return true, nil
	}
	err := w.cmd.Wait()
	return err == nil, err
}
