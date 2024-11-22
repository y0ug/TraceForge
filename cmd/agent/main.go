package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	agentUUID := flag.String("uuid", "", "The UUID of the agent")
	serverURL := flag.String("url", "http://127.0.0.1:8888", "The URL of the queue")

	flag.Parse()

	// Set up channel to handle Ctrl+C
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	fmt.Println("Client is running. Press Ctrl+C to exit.")

	// Initial pull before entering the loop
	handleMessage(*serverURL, *agentUUID)

loop:
	for {
		select {
		case <-stop:
			fmt.Println("\nExiting client.")
			break loop
		case <-timeout(4 * time.Second): // Timeout-driven loop
			handleMessage(*serverURL, *agentUUID)
		}
	}
}

// timeout returns a channel that signals after the specified duration
// this to do a time.Sleep that handle ctrl-c interrupt
func timeout(duration time.Duration) <-chan bool {
	timeoutChan := make(chan bool)
	go func() {
		time.Sleep(duration)
		timeoutChan <- true
		close(timeoutChan) // Best practice to close channels
	}()
	return timeoutChan
}

// handleMessage encapsulates the logic to pull and process messages
func handleMessage(serverURL, agentUUID string) {
	err := pullAndProcessMessage(serverURL, agentUUID)
	if err != nil {
		log.Printf("Error pulling message: %v", err)
	}
}

func pushMessage(serverURL, agentID, body string) error {
	pushBody := map[string]string{"agent_id": agentID, "body": body}
	pushData, _ := json.Marshal(pushBody)

	resp, err := http.Post(serverURL+"/push", "application/json", bytes.NewReader(pushData))
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

func pullAndProcessMessage(serverURL, agentID string) error {
	resp, err := http.Get(fmt.Sprintf("%s/pull?agent_id=%s", serverURL, agentID))
	if err != nil {
		return fmt.Errorf("failed to pull message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		log.Println("No messages available")
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	var message struct {
		ID      int    `json:"id"`
		AgentID string `json:"agent_id"`
		Body    string `json:"body"`
	}
	if err := json.Unmarshal(body, &message); err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}

	log.Printf("Pulled message: %+v\n", message)

	// Simulate processing the message
	time.Sleep(2 * time.Second)
	log.Printf("Processed message: %s\n", message.Body)

	// Delete the message after processing
	return deleteMessage(serverURL, message.ID)
}

func deleteMessage(serverURL string, messageID int) error {
	deleteBody := map[string]int{"id": messageID}
	deleteData, _ := json.Marshal(deleteBody)

	req, err := http.NewRequest(http.MethodDelete, serverURL+"/delete", bytes.NewReader(deleteData))
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response from server: %s", string(responseBody))
	}

	log.Println("Message deleted successfully")
	return nil
}
