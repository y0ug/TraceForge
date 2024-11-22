package agent

type Plugin interface {
	Handle(task Task) error
}

type Task struct {
	TaskID string      `json:"task_id"`
	Plugin string      `json:"plugin"`
	Data   interface{} `json:"data"`
}
