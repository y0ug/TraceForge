package sbapi

import (
	"TraceForge/internals/agent"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Should had a way to stop the task with the taskManager
func (s *Server) StartAnalysisTaskProcessor() error {
	for {
		s.processPendingAnalysisTasks()
		time.Sleep(5 * time.Second) // Adjust the interval as needed
	}
}

func (s *Server) processPendingAnalysisTasks() {
	ctx := context.Background()
	tasks, err := s.DB.GetPendingAnalysisTasks(ctx)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to get pending analysis tasks")
		return
	}
	for _, task := range tasks {
		s.Logger.Infof("Processing analysis task %s", task.ID)
		go s.handleAnalysisTask(task)
	}
}

func (s *Server) handleAnalysisTask(task AnalysisTask) {
	ctx := context.Background()

	agentConfig, err := s.getAgentConfigByID(task.AgentID)
	if err != nil {
		// Handle error...
		return
	}
	vmName := agentConfig.Name

	// Try to acquire VM lock
	acquired, err := s.acquireVMLock(vmName, 30*time.Minute)
	if err != nil || !acquired {
		s.Logger.WithError(err).Errorf("Failed to acquire lock for VM %s", vmName)
		// Optionally reschedule or fail the task
		return
	}
	defer s.releaseVMLock(vmName)

	// Update task status to 'running'
	err = s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "running")
	if err != nil {
		s.Logger.WithError(err).Error("Failed to update task status to running")
		return
	}

	// Get agent configuration
	agentConfig, err = s.getAgentConfigByID(task.AgentID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to get agent config")
		s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed")
		return
	}

	// Revert VM using the HVAPI server associated with the agent
	if err := s.RevertVM(agentConfig); err != nil {
		s.Logger.WithError(err).Error("Failed to revert VM")
		s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed")
		return
	}

	if err := s.StartVM(agentConfig); err != nil {
		s.Logger.WithError(err).Error("Failed to start VM")
		s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed")
		return
	}

	// stop VM should append on error too
	// defer func() {
	// 	if err := s.StopVM(agentConfig); err != nil {
	// 		s.Logger.WithError(err).Error("Failed to stop VM")
	// 		s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed")
	// 		return
	// 	}
	// }()

	// Wait for the agent to become available
	// if err := s.WaitForAgent(task.AgentID, 5*time.Minute); err != nil {
	// 	s.Logger.WithError(err).Error("Agent did not become available")
	// 	s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed", nil)
	// 	return
	// }
	//
	// Send task to agent
	if err := s.SendTaskToAgent(task); err != nil {
		s.Logger.WithError(err).Error("Failed to send task to agent")
		s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed")
		return
	}

	// Wait for agent to complete task and retrieve result
	result, err := s.WaitForTaskResult(task.ID, 10*time.Minute)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to get task result")
		s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed")
		return
	}

	err = s.DB.UpdateAnalysisTaskResults(ctx, task.ID, result)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to update task result")
		return
	}

	// Update task status to 'completed' with result
	err = s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "completed")
	if err != nil {
		s.Logger.WithError(err).Error("Failed to update task status to completed")
		return
	}

	s.Logger.Infof("Analysis task %s completed", task.ID)
}

func (s *Server) RevertVM(agentConfig *AgentConfig) error {
	hvapiConfig := agentConfig.HvapiConfig
	vmName := agentConfig.Name

	endpoint := fmt.Sprintf("%s/%s/%s/revert", hvapiConfig.URL, agentConfig.Provider, vmName)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+hvapiConfig.AuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// TODO read error message from JSON
		return fmt.Errorf("failed to revert VM, status code: %d", resp.StatusCode)
	}
	return nil
}

func (s *Server) StartVM(agentConfig *AgentConfig) error {
	hvapiConfig := agentConfig.HvapiConfig
	vmName := agentConfig.Name

	endpoint := fmt.Sprintf("%s/%s/%s/start", hvapiConfig.URL, agentConfig.Provider, vmName)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+hvapiConfig.AuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to revert VM, status code: %d", resp.StatusCode)
	}
	return nil
}

func (s *Server) StopVM(agentConfig *AgentConfig) error {
	hvapiConfig := agentConfig.HvapiConfig
	vmName := agentConfig.Name

	endpoint := fmt.Sprintf("%s/%s/%s/stop", hvapiConfig.URL, agentConfig.Provider, vmName)
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+hvapiConfig.AuthToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to revert VM, status code: %d", resp.StatusCode)
	}
	return nil
}

// SendTaskToAgent sends the analysis task to the specified agent
func (s *Server) SendTaskToAgent(task AnalysisTask) error {
	ctx := context.Background()
	expiresIn := time.Minute * 15

	// Get file information
	file, err := s.DB.GetFile(ctx, task.FileID.String())
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	// Generate presigned URL for the file (valid for a reasonable duration)
	fileURL, err := s.GeneratePresignedFileURL(ctx, file.S3Key, expiresIn)
	if err != nil {
		return fmt.Errorf("failed to generate file URL: %w", err)
	}

	s.Logger.Infof("url %s", fileURL)
	args := map[string]interface{}{
		"url": fileURL,
	}

	// Merge taskArgs into args
	taskArgs := map[string]interface{}{}
	err = json.Unmarshal(task.Args, &taskArgs)
	if err != nil {
		return fmt.Errorf("failed to unmarshal task args: %w", err)
	}
	for key, value := range taskArgs {
		args[key] = value
	}

	// Marshal args into json
	jsonArgs, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal task args: %w", err)
	}
	s.Logger.Infof("jsonArgs: %s", jsonArgs)

	// Prepare task message for the agent
	// TODO fileURL is in Data
	taskMessage := agent.Task{
		TaskID: task.ID.String(),
		Plugin: task.Plugin,
		Data:   jsonArgs,
	}

	messageBody, err := json.Marshal(taskMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal task message: %w", err)
	}

	// Send the message to the agent's queue
	err = s.MQClient.PushMessage(task.AgentID.String(), string(messageBody))
	if err != nil {
		return fmt.Errorf("failed to push message to agent: %w", err)
	}

	return nil
}

func (s *Server) WaitForTaskResult(taskID uuid.UUID, timeout time.Duration) (interface{}, error) {
	start := time.Now()
	for {
		if time.Since(start) > timeout {
			return nil, fmt.Errorf("timed out waiting for task result")
		}

		// Pull message from the queue where agent responses are sent
		msg, err := s.MQClient.PullMessage(taskID.String())
		if err != nil {
			s.Logger.WithError(err).Error("Failed to pull result message")
			time.Sleep(5 * time.Second)
			continue
		}
		if msg != nil {
			var result interface{}
			if err := json.Unmarshal([]byte(msg.Body), &result); err != nil {
				s.Logger.WithError(err).Error("Failed to parse result message")
				return nil, err
			}
			// Delete the message
			s.MQClient.DeleteMessage(msg.ID)
			return result, nil
		}
		time.Sleep(5 * time.Second)
	}
}
