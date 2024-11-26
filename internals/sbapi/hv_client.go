// internals/sbapi/hv_client.go

package sbapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Existing structs
type HttpResp struct {
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
	Status  string      `json:"status"`
}

type HvClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

// NewHvClient remains unchanged
func NewHvClient(baseURL, apiKey string) *HvClient {
	return &HvClient{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Custom error type for HV API errors
type HvError struct {
	Status  string
	Message string
}

func (e *HvError) Error() string {
	return fmt.Sprintf("HV API error - Status: %s, Message: %s", e.Status, e.Message)
}

func (c *HvClient) newRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	var buf bytes.Buffer
	if body != nil {
		err := json.NewEncoder(&buf).Encode(body)
		if err != nil {
			return nil, err
		}
	}

	fullURL := fmt.Sprintf("%s%s", c.BaseURL, path)
	req, err := http.NewRequestWithContext(ctx, method, fullURL, &buf)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	return req, nil
}

func (c *HvClient) do(req *http.Request, v *HttpResp) error {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("API error: %s", string(bodyBytes))
	}

	// Unmarshal into HttpResp
	err = json.Unmarshal(bodyBytes, v)
	if err != nil {
		return err
	}

	return nil
}

// RevertVM reverts a specific virtual machine.
func (c *HvClient) RevertVM(ctx context.Context, provider, vmName string) (*HttpResp, error) {
	path := fmt.Sprintf("/%s/%s/revert", provider, vmName)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp HttpResp
	err = c.do(req, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Status != "success" {
		return nil, &HvError{Status: resp.Status, Message: resp.Message}
	}

	return &resp, nil
}

// StartVM starts a virtual machine.
func (c *HvClient) StartVM(ctx context.Context, provider, vmName string) (*HttpResp, error) {
	path := fmt.Sprintf("/%s/%s/start", provider, vmName)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp HttpResp
	err = c.do(req, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Status != "success" {
		return nil, &HvError{Status: resp.Status, Message: resp.Message}
	}

	return &resp, nil
}

// StopVM stops a virtual machine.
func (c *HvClient) StopVM(ctx context.Context, provider, vmName string) (*HttpResp, error) {
	path := fmt.Sprintf("/%s/%s/stop", provider, vmName)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resp HttpResp
	err = c.do(req, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Status != "success" {
		return nil, &HvError{Status: resp.Status, Message: resp.Message}
	}

	return &resp, nil
}

// ... other HvClient methods remain unchanged
