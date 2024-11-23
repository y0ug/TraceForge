package agent

import (
	"fmt"
	"time"
)

type ExamplePlugin struct{}

func NewExamplePlugin() (*ExamplePlugin, error) {
	return &ExamplePlugin{}, nil
}

func (p *ExamplePlugin) Name() string {
	return "example"
}

func (p *ExamplePlugin) Handle(task Task, sendStatusUpdate func(string)) (interface{}, error) {
	fmt.Printf("Handling task with ExamplePlugin: %+v\n", task)
	for i := 0; i < 10; i++ {
		sendStatusUpdate(fmt.Sprintf("ExamplePlugin: %d", i))
		time.Sleep(1 * time.Second)
	}
	// Implement any logic and send status updates as needed
	return nil, nil
}
