package main

import (
	"TraceForge/internals/agent"
	"TraceForge/internals/mq"
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
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

	pluginManager := agent.NewPluginManager()

	pluginFactories := map[string]func() (agent.Plugin, error){
		"example": func() (agent.Plugin, error) {
			return agent.NewExamplePlugin()
		},
		"exec": func() (agent.Plugin, error) {
			return agent.NewExecPlugin()
		},
		"dlexec": func() (agent.Plugin, error) {
			return agent.NewDlExecPlugin()
		},
	}

	for name, factory := range pluginFactories {
		pluginManager.RegisterPluginFactory(name, factory)
	}

	// List of plugin names to load
	pluginNames := []string{"example", "exec", "dlexec"}

	if err := pluginManager.LoadPlugins(pluginNames); err != nil {
		logger.WithError(err).Fatal("Failed to load plugins")
	}

	handleMessage(logger, client, *agentUUID, pluginManager)
loop:
	for {
		select {
		case <-stop:
			logger.Info("Exiting client.")
			break loop
		case <-timeout(4 * time.Second): // Timeout-driven loop
			handleMessage(logger, client, *agentUUID, pluginManager)
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
func handleMessage(logger *logrus.Logger, client *mq.Client, agentID string, pluginManager *agent.PluginManager) {
	msg, err := client.PullMessage(agentID)
	if err != nil {
		logger.WithError(err).Error("Failed to pull message")
		return
	}

	if msg == nil {
		logger.Info("No messages available")
		return
	}
	var task agent.Task
	if err := json.Unmarshal([]byte(msg.Body), &task); err != nil {
		logger.WithError(err).Error("failed to parse message body")
	}

	plugin, exists := pluginManager.GetPlugin(task.Plugin)
	if !exists {
		logger.WithField("plugin", task.Plugin).Error("No plugin found")
		// We still delete the task if no plugin found
	} else {
		resp, err := plugin.Handle(task)
		if err != nil {
			logger.WithError(err).Error("failed to handle task")
		}
		if resp != nil {
			value, err := json.Marshal(resp)
			if err != nil {
				logger.WithError(err).Error("failed to marshal response")
				return
			}
			client.PushMessage(task.TaskID, string(value))
		}
	}

	// Process the task
	logger.Printf("Agent %s processing task: %+v\n", agentID, task)
	err = client.DeleteMessage(msg.ID)
	if err != nil {
		logger.WithError(err).Error("Failed to delete message")
		return
	}
}
