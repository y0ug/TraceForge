package agent

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type ExecPlugin struct{}

type ExecPluginArgs struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

type ExecPluginResponse struct {
	Output  string `json:"output"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func NewExecPlugin() (*ExecPlugin, error) {
	return &ExecPlugin{}, nil
}

func (p *ExecPlugin) Name() string {
	return "exec"
}

func (p *ExecPlugin) Handle(task Task) (interface{}, error) {
	var args ExecPluginArgs
	if err := json.Unmarshal(task.Data, &args); err != nil {
		// return nil, fmt.Errorf("Failed to parse args: %w", err)
		return &ExecPluginResponse{
			Status:  "error",
			Message: fmt.Sprint("Failed to parse args: %w", err),
			Output:  "",
		}, fmt.Errorf("Failed to parse args: %w", err)
	}

	fmt.Printf("ExecPlugin: %+v\n", task)
	output, err := p.execCommand(&args)

	response := &ExecPluginResponse{
		Status:  "success",
		Message: "",
		Output:  string(output),
	}

	if err != nil {
		response.Status = "error"
		response.Message = fmt.Sprintf("%v", err)
		return response, fmt.Errorf("Failed to execute %v", err)
	}
	return response, nil
}

func (h *ExecPlugin) execCommand(args *ExecPluginArgs) ([]byte, error) {
	cmd := exec.Command(args.Name, args.Args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("failed to execute command %s %v", args.Name, args.Args)
	}
	fmt.Println(string(output))
	return output, nil
}
