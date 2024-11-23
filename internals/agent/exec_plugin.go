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

func (p *ExecPlugin) Handle(task Task, sendStatusUpdate func(string)) (interface{}, error) {
	var args ExecPluginArgs
	if err := json.Unmarshal(task.Data, &args); err != nil {
		sendStatusUpdate(fmt.Sprintf("Failed to parse args: %v", err))
		return &ExecPluginResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to parse args: %v", err),
			Output:  "",
		}, fmt.Errorf("Failed to parse args: %w", err)
	}

	sendStatusUpdate(fmt.Sprintf("Executing command: %s %v", args.Name, args.Args))
	output, err := p.execCommand(&args)
	if err != nil {
		sendStatusUpdate(fmt.Sprintf("Command execution failed: %v", err))
	} else {
		sendStatusUpdate("Command execution succeeded: \n" + string(output))
	}

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
