package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/ks888/hornet/common"
)

// Executor fetches the task set, execute it and report its result. Repeat.
type Executor struct {
	GroupName string
	ID        int
	Addr      string
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
	}
	// TODO: check status code and handle 404 case and other error case.

	respData := common.NextTaskSetResponse{}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&respData); err != nil {
		return nextTaskSet{}, err
	}

	fmt.Printf("next task: %#v\n", respData)
	return nextTaskSet(respData), nil
}
