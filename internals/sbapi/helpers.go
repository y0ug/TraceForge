package sbapi

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

func (s *Server) fileExistsInS3(ctx context.Context, key string) (bool, error) {
	head, err := s.S3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(key),
	})
	s.Logger.WithField("head", head).WithField("err", err).Info("Checking if file exists in S3")
	return err != nil, nil
}

func (s *Server) getAgentConfigByID(agentID uuid.UUID) (*AgentConfig, error) {
	for _, agent := range s.AgentsConfig.Agents {
		if agent.ID == agentID.String() {
			return &agent, nil
		}
	}
	return nil, fmt.Errorf("agent with ID %s not found", agentID)
}

func (s *Server) GeneratePresignedFileURLGet(ctx context.Context, s3Key string, expiresIn time.Duration) (string, error) {
	// Create presigned URL
	presignClient := s3.NewPresignClient(s.S3Client)
	putObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(s3Key),
	}

	presignedReq, err := presignClient.PresignGetObject(ctx, putObjectInput, s3.WithPresignExpires(expiresIn))
	if err != nil {
		s.Logger.WithError(err).Error("Failed to generate presigned URL")
		return "", err
	}
	return presignedReq.URL, nil
}

func (s *Server) GeneratePresignedFileURLPut(ctx context.Context, s3Key string, expiresIn time.Duration) (string, error) {
	// Create presigned URL
	presignClient := s3.NewPresignClient(s.S3Client)
	putObjectInput := &s3.PutObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(s3Key),
	}

	presignedReq, err := presignClient.PresignPutObject(ctx, putObjectInput, s3.WithPresignExpires(expiresIn))
	if err != nil {
		s.Logger.WithError(err).Error("Failed to generate presigned URL")
		return "", err
	}
	return presignedReq.URL, nil
}

func (s *Server) acquireVMLock(vmName string, timeout time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("vm_lock:%s", vmName)
	success, err := s.RedisClient.SetNX(context.Background(), lockKey, "locked", timeout).Result()
	return success, err
}

func (s *Server) releaseVMLock(vmName string) {
	lockKey := fmt.Sprintf("vm_lock:%s", vmName)
	s.RedisClient.Del(context.Background(), lockKey)
}
