package sbapi

import (
	"TraceForge/internals/commons"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	// Generate UUID for the file
	fileID := uuid.New().String()
	ctx := r.Context()

	// Create S3 key - you might want to organize files in folders
	s3Key := fmt.Sprintf("uploads/%s.bin", fileID)

	// Create presign client
	presignClient := s3.NewPresignClient(s.S3Client)

	// Create put object input
	putObjectInput := &s3.PutObjectInput{
		Bucket:      aws.String(s.Config.S3BucketName),
		Key:         aws.String(s3Key),
		ContentType: aws.String("application/octet-stream"),
	}

	expiresIn := time.Minute * 15
	expiresAt := time.Now().Add(expiresIn)
	stmt, err := s.DB.DB.PrepareContext(ctx, `
    INSERT INTO file_uploads (id, s3_key, created_at, expires_at, updated_at, is_uploaded)
    VALUES ($1, $2, CURRENT_TIMESTAMP, $3, CURRENT_TIMESTAMP, false)
`)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to prepare statement")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(fileID, s3Key, expiresAt)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to insert upload record")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Generate presigned URL
	presignedReq, err := presignClient.PresignPutObject(context.Background(),
		putObjectInput,
		s3.WithPresignExpires(expiresIn)) // URL expires in 15 minutes
	if err != nil {
		s.Logger.WithError(err).Error("Failed to generate presigned URL")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := UploadResponse{
		UploadURL: presignedReq.URL,
		FileID:    fileID,
		Key:       s3Key,
		ExpiresIn: int64(expiresIn),
	}

	// Log the file upload request
	s.Logger.WithFields(log.Fields{
		"file_id": fileID,
		"s3_key":  s3Key,
	}).Info("Generated presigned URL for file upload")

	// Write success response
	commons.WriteSuccessResponse(w, "", response)
}

func (s *Server) FinishUploadHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	fileID := vars["file_id"]

	var s3Key string
	file, err := s.DB.GetFile(ctx, fileID)
	if err != nil {
		commons.WriteErrorResponse(w, "File id not found", http.StatusNotFound)
		s.Logger.WithFields(log.Fields{
			"file_id": fileID,
			"s3_key":  s3Key,
		}).Info("Not found in DB")
		return
	}

	exist, err := s.fileExistsInS3(file.S3Key)
	if err != nil {
		s.Logger.WithFields(log.Fields{
			"file_id": fileID,
			"s3_key":  file.S3Key,
		}).WithError(err).Error("Not found in DB")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !exist {
		s.Logger.WithFields(log.Fields{
			"file_id": fileID,
			"s3_key":  file.S3Key,
		}).Info("Not found in S3")
		commons.WriteErrorResponse(w, "File not found in the bucket", http.StatusNotFound)
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

	_, err = stmt.ExecContext(ctx, fileID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to update upload record")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Logger.WithFields(log.Fields{
		"file_id": fileID,
	}).Info("user finish to upload")

	commons.WriteSuccessResponse(w, "", nil)
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

	presigner := s3.NewPresignClient(s.S3Client)
	req := &s3.GetObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(file.S3Key),
	}
	presignedURL, err := presigner.PresignGetObject(ctx, req, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})
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
