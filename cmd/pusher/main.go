package main

import (
	"TraceForge/internals/mq"
	"encoding/json"
	"flag"
	"fmt"
	"os"

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

	serverURL := flag.String("url", "http://127.0.0.1:8888", "The URL of the queue")
	agentUUID := flag.String("uuid", "", "The UUID of the agent")

	// Parse command-line arguments
	flag.Parse()
	if *agentUUID == "" {
		fmt.Println("Usage: go run main.go -uuid <UUID>")
		return
	}

	client := mq.NewClient(*serverURL)

	task := Task{
		TaskID:    uuid.NewString(),
		Operation: "process",
		Data: map[string]string{
			"msg": "test",
		},
	}

	value, err := json.Marshal(task)
	if err != nil {
		logger.WithError(err).Error("failed to marshal task")
		return
	}

	client.PushMessage(*agentUUID, string(value))
}
