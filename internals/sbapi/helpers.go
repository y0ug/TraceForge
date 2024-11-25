package sbapi

import (
	"TraceForge/internals/commons"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (s *Server) fileExistsInS3(ctx context.Context, key string) (bool, error) {
	head, err := s.S3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(key),
	})
	s.Logger.WithField("head", head).WithField("err", err).Info("Checking if file exists in S3")
	return err != nil, nil
}

func (s *Server) getProvidersFromHvapi() ([]string, error) {
	hvapiUrl := s.Config.HvApiUrl
	hvapiAuthToken := s.Config.HvApiAuthToken

	client := &http.Client{}
	req, err := http.NewRequest("GET", hvapiUrl+"/providers", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to hvapi /providers: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+hvapiAuthToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get providers from hvapi: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hvapi returned non-OK status: %d", resp.StatusCode)
	}

	var hvapiResp commons.HttpResp
	err = json.NewDecoder(resp.Body).Decode(&hvapiResp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode providers response: %w", err)
	}

	data, ok := hvapiResp.Data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid providers data format")
	}

	providers := make([]string, 0, len(data))
	for _, item := range data {
		provider, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("invalid provider name format")
		}
		providers = append(providers, provider)
	}

	return providers, nil
}

func (s *Server) getVmsFromHvapi(provider string) (interface{}, error) {
	hvapiUrl := s.Config.HvApiUrl
	hvapiAuthToken := s.Config.HvApiAuthToken

	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/%s", hvapiUrl, provider), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to hvapi for provider %s: %w", provider, err)
	}
	req.Header.Set("Authorization", "Bearer "+hvapiAuthToken)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get VMs for provider %s: %w", provider, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("hvapi returned non-OK status %d for provider %s", resp.StatusCode, provider)
	}

	var hvapiResp commons.HttpResp
	err = json.NewDecoder(resp.Body).Decode(&hvapiResp)
	if err != nil {
		return nil, fmt.Errorf("failed to decode VMs response for provider %s: %w", provider, err)
	}

	return hvapiResp.Data, nil
}

func structToMap(data interface{}) map[string]interface{} {
	var result map[string]interface{}
	temp, _ := json.Marshal(data)
	json.Unmarshal(temp, &result)
	return result
}
