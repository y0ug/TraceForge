package sbapi

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	log "github.com/sirupsen/logrus"
)

func (s *Server) CleanOrphanFiles() error {
	ctx := context.Background()

	// Step 1: Retrieve all S3 keys from the database
	dbKeys, err := s.DB.GetAllS3Keys(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve S3 keys from database: %w", err)
	}

	dbKeysSet := make(map[string]struct{}, len(dbKeys))
	for _, key := range dbKeys {
		dbKeysSet[key] = struct{}{}
	}

	// Step 2: List all objects in the S3 bucket
	s3KeysSet := make(map[string]struct{})
	var continuationToken *string

	for {
		listInput := &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.Config.S3BucketName),
			ContinuationToken: continuationToken,
		}

		result, err := s.S3Client.ListObjectsV2(ctx, listInput)
		if err != nil {
			return fmt.Errorf("failed to list objects in S3: %w", err)
		}

		// Adjust for the expired time
		// For case when the client doesn't have call /upload/{upload_id}/complete
		cutoffTime := time.Now().Add(-1 * time.Minute)
		// In the loop where you collect S3 keys
		for _, item := range result.Contents {
			if item.LastModified.Before(cutoffTime) {
				s3KeysSet[*item.Key] = struct{}{}
			} else {
				s.Logger.WithFields(log.Fields{
					"key":           *item.Key,
					"last_modified": item.LastModified,
				}).Info("Skipping recent object")
			}
		}

		if *result.IsTruncated {
			continuationToken = result.NextContinuationToken
		} else {
			break
		}
	}

	// Step 3: Identify orphaned files
	var orphanKeys []string
	for key := range s3KeysSet {
		if _, exists := dbKeysSet[key]; !exists {
			orphanKeys = append(orphanKeys, key)
		}
	}

	// Step 4: Delete orphaned files
	if len(orphanKeys) == 0 {
		return nil
	}

	const batchSize = 1000 // S3 DeleteObjects allows up to 1000 objects per request
	for i := 0; i < len(orphanKeys); i += batchSize {
		end := i + batchSize
		if end > len(orphanKeys) {
			end = len(orphanKeys)
		}

		objectsToDelete := make([]types.ObjectIdentifier, 0, end-i)
		for _, key := range orphanKeys[i:end] {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{Key: aws.String(key)})
		}

		deleteInput := &s3.DeleteObjectsInput{
			Bucket: aws.String(s.Config.S3BucketName),
			Delete: &types.Delete{
				Objects: objectsToDelete,
				Quiet:   aws.Bool(true),
			},
		}

		_, err := s.S3Client.DeleteObjects(ctx, deleteInput)
		if err != nil {
			s.Logger.WithError(err).Error("Failed to delete a batch of orphaned files")
			// Decide whether to continue or return the error based on your preference
			continue
		}

		s.Logger.Infof("Deleted %d orphaned files", len(objectsToDelete))
	}
	return nil
}
