package agent

import "fmt"

type ExamplePlugin struct{}

func NewExamplePlugin() *ExamplePlugin {
	return &ExamplePlugin{}
}

func (p *ExamplePlugin) Handle(task Task) error {
	fmt.Printf("Handling task with ExamplePlugin: %+v\n", task)
	return nil
}
