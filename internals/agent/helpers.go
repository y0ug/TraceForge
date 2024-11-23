package agent

import (
	"encoding/json"
	"fmt"
)

func NewTask(taskID, websocketURL, plugin string, data interface{}) (Task, error) {
	taskData, err := json.Marshal(data)
	if err != nil {
		return Task{}, fmt.Errorf("failed to marshal task data: %w", err)
	}
	return Task{
		TaskID:       taskID,
		Plugin:       plugin,
		Data:         taskData,
		WebSocketURL: websocketURL,
	}, nil
}
