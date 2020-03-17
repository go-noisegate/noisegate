package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ks888/hornet/common"
)

// TestOptions represents the options which the test action accepts.
type TestOptions struct {
	ServerAddr string
	TestLogger io.Writer
	Parallel   string
	BuildTags  string
}

// TestAction runs the test of the packages related to the specified file.
// If the path is relative, it assumes it's the relative path from the current working directory.
func TestAction(ctx context.Context, query string, options TestOptions) error {
	path, begin, end, err := parseQuery(query)
	if err != nil {
		return err
	}

	if !filepath.IsAbs(path) {
		curr, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to find the abs path: %w", err)
		}
		path = filepath.Join(curr, path)
	}

	reqData := common.TestRequest{Path: path, Begin: begin, End: end, Parallel: options.Parallel, BuildTags: options.BuildTags}
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
		return fmt.Errorf("failed to run the test: %s:\n%s", resp.Status, string(body))
	}

	io.Copy(options.TestLogger, resp.Body)

	return nil
}

func parseQuery(pathAndRange string) (string, int, int, error) {
	chunks := strings.Split(pathAndRange, ":")
	if len(chunks) > 2 {
		return "", 0, 0, errors.New("too many `:`")
	} else if len(chunks) == 1 {
		return pathAndRange, 0, 0, nil
	}

	path := chunks[0]
	rawRange := chunks[1]
	if strings.HasPrefix(rawRange, "#") {
		rawRange = rawRange[1:]
	}

	index := strings.Index(rawRange, "-")
	if index == -1 {
		offset, err := strconv.Atoi(rawRange)
		if err != nil {
			return "", 0, 0, fmt.Errorf("failed to parse the query: %w", err)
		}
		return path, offset, offset, nil
	}

	begin, err := strconv.Atoi(rawRange[0:index])
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to parse the query: %w", err)
	}

	end, err := strconv.Atoi(rawRange[index+1:])
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to parse the query: %w", err)
	}
	return path, begin, end, nil
}

// HintOptions represents the options which the hint action accepts.
type HintOptions struct {
	ServerAddr string
}

// HintAction hints the recent change of the specified file.
// If the path is relative, it assumes it's the relative path from the current working directory.
func HintAction(ctx context.Context, query string, options HintOptions) error {
	path, begin, end, err := parseQuery(query)
	if err != nil {
		return err
	}

	if !filepath.IsAbs(path) {
		curr, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to find the abs path: %w", err)
		}
		path = filepath.Join(curr, path)
	}

	reqData := common.HintRequest{Path: path, Begin: begin, End: end}
	reqBody, err := json.Marshal(&reqData)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://%s%s", options.ServerAddr, common.HintPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to hint the recent change: %s:\n%s", resp.Status, string(body))
	}

	return nil
}
