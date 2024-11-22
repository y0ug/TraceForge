package agent

import "fmt"

type ExamplePlugin struct{}

func NewExamplePlugin() (*ExamplePlugin, error) {
	return &ExamplePlugin{}, nil
}

func (p *ExamplePlugin) Name() string {
	return "example"
}

func (p *ExamplePlugin) Handle(task Task) (interface{}, error) {
	fmt.Printf("Handling task with ExamplePlugin: %+v\n", task)
	return nil, nil
}
