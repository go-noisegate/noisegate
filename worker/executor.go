package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/ks888/hornet/common"
)

// Executor fetches the task set, execute it and report its result. Repeat.
type Executor struct {
	GroupName string
	ID        int
	Addr      string
	Workspace string
}

// Run starts the main loop.
func (e Executor) Run(ctx context.Context) error {
	for {
		// get next task set
		// copy
		// execute
		// report
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

func (e Executor) extractRepoArchive(ctx context.Context, repoArchivePath string) error {
	cmd := exec.CommandContext(ctx, "tar", "-xf", repoArchivePath)
	cmd.Dir = e.Workspace
	archiveLog, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to archive: %w\nlog:\n%s", err, string(archiveLog))
	}
	return nil
}

func (e Executor) execute(ctx context.Context, taskSet nextTaskSet) error {
	cmd := exec.CommandContext(ctx, taskSet.TestBinaryPath, "-test.v", "-test.run", strings.Join(taskSet.TestFunctions, "|"))
	cmd.Dir = e.Workspace

	logFile, err := os.Create(taskSet.LogPath)
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

func (e Executor) reportResult(ctx context.Context, taskSet nextTaskSet) error {
	return nil
}
