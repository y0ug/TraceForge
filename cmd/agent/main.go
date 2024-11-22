package main

import (
	"TraceForge/internals/mq"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type Task struct {
	TaskID    string      `json:"task_id"`
	Operation string      `json:"operation"`
	Data      interface{} `json:"data"`
}

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)
	agentUUID := flag.String("uuid", uuid.NewString(), "The UUID of the agent")
	serverURL := flag.String("url", "http://127.0.0.1:8888", "The URL of the queue")

	flag.Parse()

	// Set up channel to handle Ctrl+C
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	logger.
		WithFields(logrus.Fields{"agent_uuid": *agentUUID, "server_url": *serverURL}).
		Info("Starting client. Press Ctrl+C to exit.")

	client := mq.NewClient(*serverURL)

	handleMessage(logger, client, *agentUUID)
loop:
	for {
		select {
		case <-stop:
			logger.Info("Exiting client.")
			break loop
		case <-timeout(4 * time.Second): // Timeout-driven loop
			handleMessage(logger, client, *agentUUID)
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
func handleMessage(logger *logrus.Logger, client *mq.Client, agentID string) {
	msg, err := client.PullMessage(agentID)
	if err != nil {
		logger.WithError(err).Error("Failed to pull message")
		return
	}

	if msg == nil {
		logger.Info("No messages available")
		return
	}
	var task Task
	if err := json.Unmarshal([]byte(msg.Body), &task); err != nil {
		logger.WithError(err).Error("failed to parse message body")
	}

	// Process the task
	fmt.Printf("Agent %s processing task: %+v\n", agentID, task)
	err = client.DeleteMessage(msg.ID)
	if err != nil {
		logger.WithError(err).Error("Failed to delete message")
		return
	}
}
