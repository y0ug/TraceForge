package mq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
)

func NewClient(serverURL string) *Client {
	return &Client{
		serverURL: serverURL,
		logger:    logrus.New(),
	}
}

func (c *Client) PushMessage(queueID, body string) error {
	pushBody := map[string]string{"body": body}
	pushData, _ := json.Marshal(pushBody)

	url := fmt.Sprintf("%s/%s", c.serverURL, queueID)
	resp, err := http.Post(url, "application/json",
		bytes.NewReader(pushData))
	if err != nil {
		return fmt.Errorf("failed to push message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response from server: %s", string(responseBody))
	}

	return nil
}

func (c *Client) PullMessage(queueID string) (*MessageResponse, error) {
	resp, err := http.Get(fmt.Sprintf("%s/%s", c.serverURL, queueID))
	if err != nil {
		return nil, fmt.Errorf("failed to pull message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	body, _ := io.ReadAll(resp.Body)
	message := &MessageResponse{}
	if err := json.Unmarshal(body, message); err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}
	return message, nil
}

func (c *Client) DeleteMessage(messageID string) error {
	req, err := http.NewRequest(http.MethodDelete,
		fmt.Sprintf("%s/message/%s", c.serverURL, messageID),
		nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response from server: %s", string(responseBody))
	}

	return nil
}
