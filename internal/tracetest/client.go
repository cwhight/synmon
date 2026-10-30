package tracetest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) UpsertTest(def TestDefinition) error {
	body, err := json.Marshal(def)
	if err != nil {
		return fmt.Errorf("marshal test: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, c.baseURL+"/api/tests/"+def.Spec.ID, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upsert test: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upsert test %s: status %d: %s", def.Spec.ID, resp.StatusCode, string(respBody))
	}

	return nil
}

func (c *Client) RunTest(testID string) (int, error) {
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/api/tests/"+testID+"/run", nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("run test: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("run test %s: status %d: %s", testID, resp.StatusCode, string(respBody))
	}

	var result RunResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode run response: %w", err)
	}

	return result.ID, nil
}

func (c *Client) GetTestRun(testID string, runID int) (*RunResult, error) {
	url := fmt.Sprintf("%s/api/tests/%s/run/%d", c.baseURL, testID, runID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get test run: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get test run %s/%d: status %d: %s", testID, runID, resp.StatusCode, string(respBody))
	}

	var result RunResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode run result: %w", err)
	}

	return &result, nil
}

func (c *Client) WaitForRun(testID string, runID int, timeout, pollInterval time.Duration) (*RunResult, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, err := c.GetTestRun(testID, runID)
		if err != nil {
			return nil, err
		}

		switch result.State {
		case "FINISHED", "FAILED":
			return result, nil
		}

		time.Sleep(pollInterval)
	}

	return nil, fmt.Errorf("test run %s/%d timed out after %s", testID, runID, timeout)
}
