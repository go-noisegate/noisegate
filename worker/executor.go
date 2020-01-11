package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ks888/hornet/common"
)

// Executor fetches the task set, execute it and report its result. Repeat.
type Executor struct {
	GroupName string
	ID        int
	Addr      string
	Workspace string
}

var waitTime = time.Second

// Run starts the main loop.
func (e Executor) Run(ctx context.Context) error {
	for {
		nextTaskSet, err := e.nextTaskSet(ctx)
		if err != nil {
			if err != errNoTaskSet {
				log.Printf("failed to get the next task set: %w", err)
			}
			time.Sleep(waitTime)
			continue
		}

		successful := false
		if err := e.createWorkspace(ctx, nextTaskSet); err == nil {
			err := e.execute(ctx, nextTaskSet)
			successful = err == nil

			if err := e.removeWorkspace(ctx.nextTaskSet); err != nil {
				log.Debugf("failed to remove the workspace: %v", err)
			}
		}

		if err := e.reportResult(ctx, nextTaskSet, successful); err != nil {
			log.Printf("failed to report the result: %w", err)
		}
	}
}

type nextTaskSet common.NextTaskSetResponse

var errNoTaskSet = errors.New("no task set found")

func (e Executor) nextTaskSet(ctx context.Context) (nextTaskSet, error) {
	reqData := common.NextTaskSetRequest{WorkerGroupName: e.GroupName, WorkerID: e.ID}
	reqBody, err := json.Marshal(&reqData)
	if err != nil {
		return nextTaskSet{}, err
	}

	url := fmt.Sprintf("http://%s%s", e.Addr, common.NextTaskSetPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nextTaskSet{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nextTaskSet{}, err
	} else if resp.StatusCode == http.StatusNotFound {
		return nextTaskSet{}, errNoTaskSet
	} else if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nextTaskSet{}, fmt.Errorf("failed to get next task set: %s", string(body))
	}

	respData := common.NextTaskSetResponse{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&respData); err != nil {
		return nextTaskSet{}, err
	}

	return nextTaskSet(respData), nil
}

func (e Executor) createWorkspace(ctx context.Context, taskSet nextTaskSet) error {
	cmd := exec.CommandContext(ctx, "tar", "-xf", taskSet.RepoArchivePath)

	logFile, err := os.OpenFile(taskSet.LogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open the log file %s: %w\n", taskSet.LogPath, err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	cmd.Dir = e.workspacePath(taskSet)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to archive: %w", err)
	}
	return nil
}

func (e Executor) workspacePath(taskSet nextTaskSet) string {
	return filepath.Join(e.Workspace, taskSet.JobID)
}

func (e Executor) execute(ctx context.Context, taskSet nextTaskSet) error {
	cmd := exec.CommandContext(ctx, taskSet.TestBinaryPath, "-test.v", "-test.run", strings.Join(taskSet.TestFunctions, "|"))
	cmd.Dir = e.workspacePath(taskSet)

	logFile, err := os.OpenFile(taskSet.LogPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to open the log file %s: %w\n", taskSet.LogPath, err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute the task set: %w", err)
	}
	return nil
}

func (e Executor) removeWorkspace(ctx context.Context, taskSet nextTaskSet) error {
	return os.RemoveAll(e.workspacePath(taskSet))
}

func (e Executor) reportResult(ctx context.Context, taskSet nextTaskSet, successful bool) error {
	reqData := common.ReportResultRequest{JobID: taskSet.JobID, TaskSetID: taskSet.TaskSetID, Successful: successful}
	reqBody, err := json.Marshal(&reqData)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s%s", e.Addr, common.ReportResultPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	if resp, err := http.DefaultClient.Do(req); err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to report result: %s", string(body))
	}
	return nil
}
