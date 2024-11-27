package sbapi

import (
	"TraceForge/internals/commons"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func (s *Server) GetFilesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	files, err := s.DB.GetFiles(ctx)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to query files")
	}
	commons.WriteSuccessResponse(w, "", files)
}

func (s *Server) GetPresignedURLHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	expiresIn := 15 * time.Minute

	// Generate UUID for upload ID
	uploadID := uuid.New().String()

	// Create S3 key
	s3Key := fmt.Sprintf("uploads/%s.bin", uploadID)

	fileURL, err := s.GeneratePresignedFileURLPut(ctx, s3Key, expiresIn)
	if err != nil {
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Store uploadID and s3Key in Redis
	err = s.RedisClient.Set(ctx, uploadID, s3Key, expiresIn).Err()
	if err != nil {
		s.Logger.WithError(err).Error("Failed to store upload ID in Redis")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := UploadResponse{
		UploadURL: fileURL,
		FileID:    uploadID,
		Key:       s3Key,
		ExpiresIn: int64(expiresIn.Seconds()),
	}

	s.Logger.WithFields(log.Fields{
		"upload_id": uploadID,
		"s3_key":    s3Key,
	}).Info("Generated presigned URL for file upload")

	commons.WriteSuccessResponse(w, "", response)
}

func (s *Server) CompleteUploadHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	uploadID := vars["file_id"]

	// Retrieve s3Key from Redis
	s3Key, err := s.RedisClient.Get(ctx, uploadID).Result()
	if err == redis.Nil {
		commons.WriteErrorResponse(w, "Upload ID not found or expired", http.StatusNotFound)
		return
	} else if err != nil {
		s.Logger.WithError(err).Error("Failed to get s3Key from Redis")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Fetch the file from S3
	resp, err := s.S3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		s.Logger.WithError(err).Error("Failed to get S3 object")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	// Initialize hash.Hash
	hash := sha256.New()

	// Stream the file and compute hash
	if _, err := io.Copy(hash, resp.Body); err != nil {
		s.Logger.WithError(err).Error("Failed to compute SHA256 hash")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get the final hash
	hashString := hex.EncodeToString(hash.Sum(nil))

	// Check if file with this hash already exists
	var existingID string
	err = s.DB.DB.QueryRowContext(ctx, "SELECT id FROM file_uploads WHERE sha256 = $1", hashString).
		Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		s.Logger.WithError(err).Error("Failed to query file by hash")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	fileID := ""
	msg := "File uploaded successfully"
	if existingID != "" {
		s.Logger.WithFields(log.Fields{
			"sha256": hashString,
			"id":     existingID,
		}).Info("Duplicate file detected")

		// Optionally, delete the uploaded duplicate file
		_, err = s.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.Config.S3BucketName),
			Key:    aws.String(s3Key),
		})
		if err != nil {
			s.Logger.WithError(err).Error("Failed to delete duplicate S3 object")
		}

		// Remove from Redis
		s.RedisClient.Del(ctx, uploadID)

		fileID = existingID
		msg = "File already exists"
	} else {

		// Rename the file in S3 to the SHA256 hash
		newS3Key := fmt.Sprintf("uploads/%s.bin", hashString)

		// Copy object to new key
		_, err = s.S3Client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(s.Config.S3BucketName),
			CopySource: aws.String(fmt.Sprintf("%s/%s", s.Config.S3BucketName, s3Key)),
			Key:        aws.String(newS3Key),
		})
		if err != nil {
			s.Logger.WithError(err).Error("Failed to copy S3 object")
			commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Delete the old object
		_, err = s.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.Config.S3BucketName),
			Key:    aws.String(s3Key),
		})
		if err != nil {
			s.Logger.WithError(err).Error("Failed to delete old S3 object")
			// Proceed anyway
		}

		// Insert new record into the database
		fileID = uuid.New().String()
		now := time.Now()
		stmt, err := s.DB.DB.PrepareContext(ctx, `
        INSERT INTO file_uploads (id, s3_key, created_at, updated_at, sha256)
        VALUES ($1, $2, $3, $3, $4)
    `)
		if err != nil {
			s.Logger.WithError(err).Error("Failed to prepare statement")
			commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		defer stmt.Close()

		_, err = stmt.ExecContext(ctx, fileID, newS3Key, now, hashString)
		if err != nil {
			s.Logger.WithError(err).Error("Failed to insert file record")
			commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Remove from Redis
		s.RedisClient.Del(ctx, uploadID)

		s.Logger.WithFields(log.Fields{
			"file_id": fileID,
			"s3_key":  newS3Key,
			"sha256":  hashString,
		}).Info("File uploaded and recorded successfully")
	}

	file, err := s.DB.GetFile(ctx, fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.Logger.WithError(err).Warn("File not found")
			commons.WriteErrorResponse(w, "File not found", http.StatusNotFound)
		} else {
			s.Logger.WithError(err).WithFields(log.Fields{"id": fileID}).Error("Failed to query file")
			commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	s.Logger.WithFields(log.Fields{"id": fileID}).Info("file info")
	commons.WriteSuccessResponse(w, msg, file)
}

func (s *Server) UpdateFileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	fileID := vars["file_id"]

	var params struct {
		Filename string `json:"filename"`
	}

	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		s.Logger.WithError(err).Error("Failed to decode request body")
		commons.WriteErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	stmt, err := s.DB.DB.PrepareContext(ctx, `
    UPDATE file_uploads
    SET is_uploaded = true, updated_at = CURRENT_TIMESTAMP
    WHERE id = $1
`)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to prepare statement")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.ExecContext(ctx, params.Filename, fileID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to update upload record")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Logger.WithFields(log.Fields{
		"file_id":  fileID,
		"filename": params.Filename,
	}).Info("Updated upload record")

	commons.WriteSuccessResponse(w, "Upload record updated", nil)
}

func (s *Server) GetFileDlHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	fileID := vars["file_id"]
	expiresIn := 15 * time.Minute

	file, err := s.DB.GetFile(ctx, fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.Logger.WithError(err).Warn("File not found")
			commons.WriteErrorResponse(w, "File not found", http.StatusNotFound)
		} else {
			s.Logger.WithError(err).WithFields(log.Fields{"id": fileID}).Error("Failed to query file")
			commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	s.Logger.WithFields(log.Fields{"id": fileID}).Info("file info")

	presignedURL, err := s.GeneratePresignedFileURLGet(ctx, file.S3Key, expiresIn)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to generate presigned URL")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
	}
	commons.WriteSuccessResponse(w, "", presignedURL)
}

func (s *Server) GetFileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	fileID := vars["file_id"]

	file, err := s.DB.GetFile(ctx, fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.Logger.WithError(err).Warn("File not found")
			commons.WriteErrorResponse(w, "File not found", http.StatusNotFound)
		} else {
			s.Logger.WithError(err).WithFields(log.Fields{"id": fileID}).Error("Failed to query file")
			commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	s.Logger.WithFields(log.Fields{"id": fileID}).Info("file info")
	commons.WriteSuccessResponse(w, "", file)
}

func (s *Server) DeleteFileHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	fileID := vars["file_id"]

	file, err := s.DB.GetFile(ctx, fileID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.Logger.WithError(err).Warn("File not found")
			commons.WriteErrorResponse(w, "File not found", http.StatusNotFound)
		} else {
			s.Logger.WithError(err).WithFields(log.Fields{"id": fileID}).Error("Failed to query file")
			commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Originaly I was using a SQL transaction here
	_, err = s.S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(file.S3Key),
	})
	if err != nil {
		s.Logger.WithError(err).Error("Failed to delete S3 object")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := s.DB.DeleteFile(ctx, fileID); err != nil {
		s.Logger.WithError(err).Error("Failed to delete file in DB")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Logger.WithFields(log.Fields{
		"file_id": fileID,
		"s3_key":  file.S3Key,
	}).Info("Deleted file ")

	commons.WriteSuccessResponse(w, "File deleted", nil)
}

func (s *Server) TasksHandler(w http.ResponseWriter, r *http.Request) {
	tasks := s.TaskManager.GetTasks()
	s.Logger.Info("tasks", tasks)
	commons.WriteSuccessResponse(w, "", tasks)
}

func (s *Server) RunTaskHandler(w http.ResponseWriter, r *http.Request) {
	taskName := mux.Vars(r)["task_name"]
	task := s.TaskManager.RunTask(taskName)
	if task == nil {
		commons.WriteErrorResponse(w, "Task not found", http.StatusNotFound)
		return
	}
	if !task.Enabled {
		commons.WriteErrorResponseData(w, "Task is disable", task, http.StatusConflict)
	}
	if task.Status == "running" {
		commons.WriteErrorResponseData(w, "Task is already running", task, http.StatusConflict)
		return
	}

	// Refresh task
	task, _ = s.TaskManager.GetTask(taskName)
	commons.WriteSuccessResponse(w, "Task started", task)
}

func (s *Server) CreateAnalysisTaskHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var params struct {
		AgentID uuid.UUID       `json:"agent_id"`
		Plugin  string          `json:"plugin"`
		FileID  uuid.UUID       `json:"file_id"`
		Args    json.RawMessage `json:"args"`
	}

	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		s.Logger.WithError(err).Error("Failed to decode request body")
		commons.WriteErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// jsonArgs, err := json.Marshal(params.Args)
	// if err != nil {
	// 	s.Logger.WithError(err).Error("Failed to encode analysis parameters")
	// 	commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
	// 	return
	// }

	taskID := uuid.New()
	now := time.Now()
	err := s.DB.CreateAnalysisTask(ctx, AnalysisTask{
		ID:        taskID,
		FileID:    params.FileID,
		AgentID:   params.AgentID,
		Plugin:    params.Plugin,
		Args:      params.Args,
		Status:    "pending",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		s.Logger.WithError(err).Error("Failed to create analysis task")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	commons.WriteSuccessResponse(w, "Analysis task created", map[string]interface{}{
		"task_id": taskID,
	})
}

// GetAnalysisTasksHandler retrieves analysis tasks
func (s *Server) GetAnalysisTasksHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tasks, err := s.DB.GetAnalysisTasks(ctx)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to get analysis tasks")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	commons.WriteSuccessResponse(w, "", tasks)
}

func (s *Server) GetAgentsHandler(w http.ResponseWriter, r *http.Request) {
	// ctx := r.Context()

	var agentsInfo []AgentInfo

	for _, agent := range s.AgentsConfig.Agents {

		agentInfo := AgentInfo{
			ID:      agent.ID,
			Name:    agent.Name,
			Plugins: agent.Plugins,
		}
		agentsInfo = append(agentsInfo, agentInfo)
	}

	if err := json.NewEncoder(w).Encode(agentsInfo); err != nil {
		s.Logger.WithError(err).Error("Failed to encode agents info")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
