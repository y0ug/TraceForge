package sbapi

import (
	"TraceForge/internals/commons"
	"context"
	"database/sql"
	"encoding/json"
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
	rows, err := s.DB.Query(`
        SELECT id, filename, s3_key, created_at, expires_at, updated_at, is_uploaded, sha1, sha256
        FROM file_uploads 
        ORDER BY created_at DESC
    `)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to query uploads")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var uploads []FileInfo
	for rows.Next() {
		var upload FileInfo
		if err := rows.Scan(&upload.ID,
			&upload.Filename,
			&upload.S3Key,
			&upload.CreatedAt,
			&upload.ExpiresAt,
			&upload.UpdatedAt,
			&upload.IsUploaded,
			&upload.Sha1,
			&upload.Sha256,
		); err != nil {
			s.Logger.WithError(err).Error("Failed to scan row")
			continue
		}

		uploads = append(uploads, upload)

	}
	commons.WriteSuccessResponse(w, "", uploads)
}

func (s *Server) GetPresignedURLHandler(w http.ResponseWriter, r *http.Request) {
	// Generate UUID for the file
	fileID := uuid.New().String()

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

	expiresIn := 15 * 60
	stmt, err := s.DB.Prepare(fmt.Sprintf(`
        INSERT INTO file_uploads (id, s3_key, created_at, expires_at, updated_at, is_uploaded)
        VALUES (?, ?, datetime('now'), datetime('now', '+%d seconds'), datetime('now'), false)
    `, expiresIn))
	if err != nil {
		s.Logger.WithError(err).Error("Failed to prepare statement")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(fileID, s3Key)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to insert upload record")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Generate presigned URL
	presignedReq, err := presignClient.PresignPutObject(context.Background(),
		putObjectInput,
		s3.WithPresignExpires(time.Minute*15)) // URL expires in 15 minutes
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
	vars := mux.Vars(r)
	fileID := vars["file_id"]

	var s3Key string
	err := s.DB.QueryRow("SELECT s3_key FROM file_uploads WHERE id = ?", fileID).Scan(&s3Key)
	if err != nil {
		commons.WriteErrorResponse(w, "File id not found", http.StatusNotFound)
		s.Logger.WithFields(log.Fields{
			"file_id": fileID,
			"s3_key":  s3Key,
		}).Info("Not found in DB")
		return
	}

	exist, err := s.fileExistsInS3(s3Key)
	if err != nil {
		s.Logger.WithFields(log.Fields{
			"file_id": fileID,
			"s3_key":  s3Key,
		}).WithError(err).Error("Not found in DB")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !exist {
		s.Logger.WithFields(log.Fields{
			"file_id": fileID,
			"s3_key":  s3Key,
		}).Info("Not found in S3")
		commons.WriteErrorResponse(w, "File not found in the bucket", http.StatusNotFound)
		return
	}

	stmt, err := s.DB.Prepare(`
        UPDATE file_uploads
        SET is_uploaded = true,updated_at = datetime('now')
        WHERE id = ?
    `)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to prepare statement")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(fileID)
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

	stmt, err := s.DB.Prepare(`
        UPDATE file_uploads
        SET filename = ?, updated_at = datetime('now')
        WHERE id = ?
    `)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to prepare statement")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(params.Filename, fileID)
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

func (s *Server) GetFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["file_id"]

	var upload FileInfo
	err := s.DB.QueryRow(`
        SELECT id, filename, s3_key, created_at, expires_at, updated_at, is_uploaded ,sha1, sha256
        FROM file_uploads
        WHERE id = ?
    `, fileID).Scan(
		&upload.ID,
		&upload.Filename,
		&upload.S3Key,
		&upload.CreatedAt,
		&upload.ExpiresAt,
		&upload.UpdatedAt,
		&upload.IsUploaded,
		&upload.Sha1,
		&upload.Sha256,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			s.Logger.WithError(err).Warn("File not found")
			commons.WriteErrorResponse(w, "File not found", http.StatusNotFound)
		} else {
			s.Logger.WithError(err).Error("Failed to query file")
			commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	s.Logger.WithFields(log.Fields{"id": upload.ID}).Info("file info")
	commons.WriteSuccessResponse(w, "", upload)
}

func (s *Server) DeleteFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["file_id"]

	var s3Key string
	err := s.DB.QueryRow("SELECT s3_key FROM file_uploads WHERE id = ?", fileID).Scan(&s3Key)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to get s3_key for file_id")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	tx, err := s.DB.Begin()
	if err != nil {
		s.Logger.WithError(err).Error("Failed to begin transaction")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec("DELETE FROM file_uploads WHERE id = ?", fileID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to delete record")
		tx.Rollback()
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = s.S3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		s.Logger.WithError(err).Error("Failed to delete S3 object")
		tx.Rollback()
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		s.Logger.WithError(err).Error("Failed to commit transaction")
		commons.WriteErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Logger.WithFields(log.Fields{
		"file_id": fileID,
		"s3_key":  s3Key,
	}).Info("Deleted file upload")

	commons.WriteSuccessResponse(w, "File upload deleted", nil)
}
