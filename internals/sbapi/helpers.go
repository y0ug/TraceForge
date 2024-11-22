package sbapi

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func (s *Server) fileExistsInS3(key string) (bool, error) {
	_, err := s.S3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *types.NoSuchKey
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
