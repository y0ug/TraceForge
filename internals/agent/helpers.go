package agent

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

func NewTask(plugin string, data interface{}) (Task, error) {
	taskData, err := json.Marshal(data)
	if err != nil {
		return Task{}, fmt.Errorf("failed to marshal task data: %w", err)
	}
	return Task{
		TaskID: uuid.NewString(),
		Plugin: plugin,
		Data:   taskData,
	}, nil
}
