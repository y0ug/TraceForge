package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type TinyTracerPlugin struct{}

type TinyTracerPluginArgs struct {
	URL string `json:"url"`
	Ext string `json:"ext"` // dll or exe
}

// TinyTracerPluginResponse defines the structure of the plugin's response
type TinyTracerPluginResponse struct {
	Output     string `json:"output"`
	TagContent string `json:"tag_content"`
	Status     string `json:"status"`
	Message    string `json:"message"`
}

// NewTinyTracerPlugin creates a new instance of TinyTracerPlugin
func NewTinyTracerPlugin() (*TinyTracerPlugin, error) {
	return &TinyTracerPlugin{}, nil
}

// Name returns the name of the plugin
func (p *TinyTracerPlugin) Name() string {
	return "tiny_tracer"
}

// Handle processes the task using TinyTracer
func (p *TinyTracerPlugin) Handle(task Task, sendStatusUpdate func(string)) (interface{}, error) {
	// Verify the existence of the TinyTracer directory
	tracerDir := `C:\pin\source\tools\tiny_tracer\`
	if _, err := os.Stat(tracerDir); os.IsNotExist(err) {
		sendStatusUpdate(fmt.Sprintf("Directory %s does not exist.", tracerDir))
		return &TinyTracerPluginResponse{
			Status:     "error",
			Message:    fmt.Sprintf("Directory %s does not exist.", tracerDir),
			Output:     "",
			TagContent: "",
		}, fmt.Errorf("directory %s does not exist", tracerDir)
	}

	// Parse task arguments
	var args TinyTracerPluginArgs
	if err := json.Unmarshal(task.Data, &args); err != nil {
		sendStatusUpdate(fmt.Sprintf("Failed to parse args: %v", err))
		return &TinyTracerPluginResponse{
			Status:     "error",
			Message:    fmt.Sprintf("Failed to parse args: %v", err),
			Output:     "",
			TagContent: "",
		}, fmt.Errorf("failed to parse args: %w", err)
	}

	fmt.Println(args)

	// Validate Ext
	if args.Ext != "dll" && args.Ext != "exe" {
		sendStatusUpdate("Invalid ext. Must be 'dll' or 'exe'.")
		return &TinyTracerPluginResponse{
			Status:     "error",
			Message:    "Invalid ext. Must be 'dll' or 'exe'.",
			Output:     "",
			TagContent: "",
		}, fmt.Errorf("invalid ext: %s", args.Ext)
	}

	// Download the file using helper function
	sendStatusUpdate("Downloading the file...")
	downloadedFilePath, err := downloadFileToTemp(args.URL, fmt.Sprintf("%s.%s", task.TaskID, args.Ext))
	if err != nil {
		sendStatusUpdate(fmt.Sprintf("Failed to download file: %v", err))
		return &TinyTracerPluginResponse{
			Status:     "error",
			Message:    fmt.Sprintf("Failed to download file: %v", err),
			Output:     "",
			TagContent: "",
		}, fmt.Errorf("failed to download file: %w", err)
	}
	defer os.Remove(downloadedFilePath) // Clean up the downloaded file

	sendStatusUpdate(fmt.Sprintf("File downloaded to %s", downloadedFilePath))

	// Define paths
	runMePath := filepath.Join(tracerDir, "install32_64", "run_me.bat")
	if _, err := os.Stat(runMePath); os.IsNotExist(err) {
		sendStatusUpdate(fmt.Sprintf("Script %s does not exist.", runMePath))
		return &TinyTracerPluginResponse{
			Status:     "error",
			Message:    fmt.Sprintf("Script %s does not exist.", runMePath),
			Output:     "",
			TagContent: "",
		}, fmt.Errorf("script %s does not exist", runMePath)
	}

	// Define the output .tag file path
	tagFilePath := fmt.Sprintf("%s.tag", downloadedFilePath)

	// Prepare the command
	cmd := exec.Command(runMePath, downloadedFilePath, args.Ext, tagFilePath)

	// Create buffers to capture stdout and stderr
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Capture stdout and stderr
	sendStatusUpdate("Executing run_me.bat...")

	// Start and wait for the command to finish with a timeout
	err = cmd.Start()
	if err != nil {
		sendStatusUpdate(fmt.Sprintf("Failed to start run_me.bat: %v", err))
		return &TinyTracerPluginResponse{
			Status:     "error",
			Message:    fmt.Sprintf("Failed to start run_me.bat: %v", err),
			Output:     stdoutBuf.String(),
			TagContent: "",
		}, fmt.Errorf("failed to start run_me.bat: %w", err)
	}

	timeoutDuration := 120 * time.Second // Increased timeout for potentially longer executions
	response := &TinyTracerPluginResponse{
		Status:     "success",
		Message:    "Task completed successfully.",
		Output:     "",
		TagContent: "",
	}
	done := make(chan error)
	go func() { done <- cmd.Wait() }()

	select {
	case <-time.After(timeoutDuration):
		cmd.Process.Kill()
		sendStatusUpdate("run_me.bat timed out and was killed.")
		response.Status = "error"
		response.Message = "run_me.bat timed out."

		//  fmt.Errorf("run_me.bat timed out")
	case err := <-done:
		if err != nil {
			sendStatusUpdate(fmt.Sprintf("run_me.bat execution failed: %v", err))
			response.Status = "error"
			response.Message = fmt.Sprintf("run_me.bat execution failed: %w", err)
			// return &TinyTracerPluginResponse{
			// 	Status:     "error",
			// 	Message:    fmt.Sprintf("run_me.bat execution failed: %v", err),
			// 	Output:     output,
			// 	TagContent: "",
			// }, fmt.Errorf("run_me.bat execution failed: %w", err)
		}
	}

	sendStatusUpdate("run_me.bat executed successfully.")

	// Read the .tag file
	tagContent, err := os.ReadFile(tagFilePath)
	if err != nil {
		sendStatusUpdate(fmt.Sprintf("Failed to read tag file: %v", err))
		response.Status = "error"
		response.Message = fmt.Sprintf("Failed to read tag file: %v", err)
		// fmt.Errorf("failed to read tag file: %w", err)
	}

	// Cleanup the tag file
	os.Remove(tagFilePath)

	sendStatusUpdate("Task completed successfully.")
	response.Output = stdoutBuf.String()
	response.TagContent = string(tagContent)
	return response, nil
}
