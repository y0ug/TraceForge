package sbapi

import (
	"TraceForge/internals/agent"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (s *Server) WrapStartAgentTaskWorker(agentID string) func() error {
	return func() error {
		return s.StartAgentTaskWorker(agentID)
	}
}

// Should had a way to stop the task with the taskManager
// TODO: acquireVMLock should be called if we start mutiple backend
// or the task should run on only one backend
func (s *Server) StartAgentTaskWorker(agentID string) error {
	ctx := context.Background()
	s.Logger.Infof("Starting task worker for agent %s", agentID)

	parsedUUID, err := uuid.Parse(agentID)
	if err != nil {
		err := fmt.Errorf("Error parsing UUID: %v\n", err)
		s.Logger.WithError(err)
		return err
	}
	// Initialize HvClient for this agent
	agentConfig, err := s.getAgentConfigByID(parsedUUID)
	if err != nil {
		s.Logger.WithError(err).Errorf("Failed to get agent config for agent %s", agentID)
		return err
	}

	hvClient := NewHvClient(agentConfig.HvapiConfig.URL, agentConfig.HvapiConfig.AuthToken)

	// We are stopping the VM to ensure a clean start
	_, err = hvClient.StopVM(ctx, agentConfig.Provider, agentConfig.Name)
	if err != nil {
		if hvErr, ok := err.(*HvError); ok {
			s.Logger.Errorf("HV API Error during StopVM - %s: %s", hvErr.Status, hvErr.Message)
		} else {
			s.Logger.WithError(err).Error("Failed to stop VM")
		}
	}

	for {
		task, err := s.DB.GetNextPendingAnalysisTaskForAgent(ctx, agentID)
		if err != nil {
			s.Logger.WithError(err).Errorf("Failed to get pending task for agent %s", agentID)
			time.Sleep(5 * time.Second)
			continue
		}
		if task == nil {
			// Sleep and try again
			time.Sleep(5 * time.Second)
			continue
		}

		s.Logger.Infof("Processing analysis task %s for agent %s", task.ID, agentID)
		s.handleAnalysisTask(*task, hvClient)
		time.Sleep(1 * time.Second)
	}
}

func (s *Server) handleAnalysisTask(task AnalysisTask, hvClient *HvClient) {
	ctx := context.Background()

	agentConfig, err := s.getAgentConfigByID(task.AgentID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to get agent config")
		s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed")
		return
	}

	// Update task status to 'running'
	err = s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "running")
	if err != nil {
		s.Logger.WithError(err).Error("Failed to update task status to running")
		return
	}

	// Use HvClient to revert VM
	_, err = hvClient.RevertVM(ctx, agentConfig.Provider, agentConfig.Name)
	if err != nil {
		if hvErr, ok := err.(*HvError); ok {
			s.Logger.Errorf("HV API Error during RevertVM - %s: %s", hvErr.Status, hvErr.Message)
		} else {
			s.Logger.WithError(err).Error("Failed to revert VM")
		}
		s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed")
		return
	}

	// Use HvClient to start VM
	_, err = hvClient.StartVM(ctx, agentConfig.Provider, agentConfig.Name)
	if err != nil {
		if hvErr, ok := err.(*HvError); ok {
			s.Logger.Errorf("HV API Error during StartVM - %s: %s", hvErr.Status, hvErr.Message)
		} else {
			s.Logger.WithError(err).Error("Failed to start VM")
		}
		s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed")
		return
	}

	// Defer stopping the VM using HvClient
	// defer func() {
	// 	resp, err := hvClient.StopVM(ctx, agentConfig.Provider, agentConfig.Name)
	// 	if err != nil {
	// 		if hvErr, ok := err.(*HvError); ok {
	// 			s.Logger.Errorf("HV API Error during StopVM - %s: %s", hvErr.Status, hvErr.Message)
	// 		} else {
	// 			s.Logger.WithError(err).Error("Failed to stop VM")
	// 		}
	// 		s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed")
	// 		return
	// 	}
	// 	if resp.Status != "success" {
	// 		s.Logger.Errorf("Failed to stop VM: %s", resp.Message)
	// 		s.DB.UpdateAnalysisTaskStatus(ctx, task.ID, "failed")
	// 		return
	// 	}
	// }()

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
	fileURL, err := s.GeneratePresignedFileURLGet(ctx, file.S3Key, expiresIn)
	if err != nil {
		return fmt.Errorf("failed to generate file URL: %w", err)
	}

	s.Logger.Debugf("url %s", fileURL)
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
	s.Logger.Debugf("jsonArgs: %s", jsonArgs)

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
