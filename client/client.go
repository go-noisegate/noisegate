package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/ks888/hornet/common"
)

// TestOptions represents the options which the test action accepts.
type TestOptions struct {
	ServerAddr string
	TestLogger io.Writer
}

// TestAction runs the test of the packages related to the specified file.
func TestAction(ctx context.Context, filepath string, options TestOptions) error {
	reqData := common.TestRequest{Path: filepath}
	reqBody, err := json.Marshal(&reqData)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s%s", options.ServerAddr, common.TestPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to run the test: %s: %s", resp.Status, string(body))
	}

	io.Copy(options.TestLogger, resp.Body)

	return nil
}

// WatchOptions represents the options which the watch action accepts.
type WatchOptions struct {
	ServerAddr string
}

// WatchAction watches the repository to which the specified file belongs.
func WatchAction(ctx context.Context, filepath string, options WatchOptions) error {
	reqData := common.WatchRequest{Path: filepath}
	reqBody, err := json.Marshal(&reqData)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s%s", options.ServerAddr, common.WatchPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to watch the repository: %s: %s", resp.Status, string(body))
	}

	return nil
}
