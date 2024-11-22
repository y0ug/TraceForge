package sbapi

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

func (s *Server) HasherTask() {
	s.Logger.Infof("Hasher service started")

	for {
		// Fetch files needing hash calculation
		rows, err := s.DB.DB.Query(`
            SELECT id, s3_key
            FROM file_uploads
            WHERE sha256 == "" AND is_uploaded IS true
        `)
		if err != nil {
			s.Logger.WithError(err).Error("Failed to query uploads needing hash calculation")
			time.Sleep(1 * time.Minute)
			continue
		}

		var files []FileInfo
		for rows.Next() {
			var file FileInfo
			if err := rows.Scan(&file.ID, &file.S3Key); err != nil {
				s.Logger.WithError(err).Error("Failed to scan row")
				continue
			}
			files = append(files, file)
		}
		rows.Close()

		// Process each file
		for _, file := range files {
			if err := s.processFile(file); err != nil {
				s.Logger.WithFields(log.Fields{
					"file_id": file.ID,
					"s3_key":  file.S3Key,
				}).WithError(err).Error("Failed to process file")
			}
		}

		time.Sleep(1 * time.Minute)
	}
}

func (s *Server) processFile(file FileInfo) error {
	// Fetch the file from S3
	resp, err := s.S3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(file.S3Key),
	})
	if err != nil {
		return fmt.Errorf("failed to get S3 object: %w", err)
	}
	defer resp.Body.Close()

	// Calculate the hash
	hash := sha256.New()
	if _, err := io.Copy(hash, resp.Body); err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}
	hash256 := hex.EncodeToString(hash.Sum(nil))

	hash = sha1.New()
	if _, err := io.Copy(hash, resp.Body); err != nil {
		return fmt.Errorf("failed to calculate hash: %w", err)
	}
	hash1 := hex.EncodeToString(hash.Sum(nil))

	// Update the database with the hash
	stmt, err := s.DB.DB.Prepare(`
        UPDATE file_uploads
        SET sha1= ?, sha256 = ?
        WHERE id = ?
    `)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	if _, err := stmt.Exec(hash1, hash256, file.ID); err != nil {
		return fmt.Errorf("failed to update upload record: %w", err)
	}

	s.Logger.WithFields(log.Fields{
		"file_id": file.ID,
		"s3_key":  file.S3Key,
		"sha1":    hash1,
		"sha256":  hash256,
	}).Info("Updated file with hash")

	return nil
}

func (s *Server) CleanupTask() {
	for {
		s.cleanupExpiredEntries()
		time.Sleep(15 * time.Minute)
	}
}

func (s *Server) cleanupExpiredEntries() {
	rows, err := s.DB.DB.Query(`
        SELECT id, s3_key FROM file_uploads
        WHERE expires_at <= datetime('now') AND is_uploaded IS false 
    `)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to query expired entries")
		return
	}
	defer rows.Close()

	var ids []uuid.UUID
	var s3Keys []string
	for rows.Next() {
		var id uuid.UUID
		var s3Key string
		if err := rows.Scan(&id, &s3Key); err != nil {
			s.Logger.WithError(err).Error("Failed to scan row")
			continue
		}
		ids = append(ids, id)
		s3Keys = append(s3Keys, s3Key)
	}

	for i, id := range ids {
		s.deleteFileUpload(id, s3Keys[i])
	}

	s.Logger.Info("Completed cleanup of expired entries")
}

func (s *Server) deleteFileUpload(id uuid.UUID, s3Key string) {
	tx, err := s.DB.DB.Begin()
	if err != nil {
		s.Logger.WithError(err).Error("Failed to begin transaction")
		return
	}

	_, err = tx.Exec("DELETE FROM file_uploads WHERE id = ?", id)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to delete record")
		tx.Rollback()
		return
	}

	_, err = s.S3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		s.Logger.WithError(err).Error("Failed to delete S3 object")
		tx.Rollback()
		return
	}

	if err := tx.Commit(); err != nil {
		s.Logger.WithError(err).Error("Failed to commit transaction")
		return
	}

	s.Logger.WithFields(log.Fields{
		"id":     id,
		"s3_key": s3Key,
	}).Info("Deleted expired file upload")
}
