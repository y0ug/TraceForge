package agent

import "encoding/json"

type Plugin interface {
	Handle(task Task) (interface{}, error)
	Name() string
}

type Task struct {
	TaskID string `json:"task_id"`
	Plugin string `json:"plugin"`
	// Data   interface{} `json:"data"`
	Data json.RawMessage `json:"data"`
}
