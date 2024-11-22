package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/joho/godotenv"
)

type Config struct {
	S3BucketName string
	S3Region     string
	S3Endpoint   string
	S3AccessKey  string
	S3SecretKey  string
	SQSQueueURL  string
	SQSEndpoint  string
	SQSAccessKey string
	SQSSecretKey string
}

type Task struct {
	TaskID    string `json:"task_id"`
	Operation string `json:"operation"`
	Data      string `json:"data"`
}

func getEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("%s must be set", key)
	}
	return value
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	agentUUID := flag.String("uuid", "", "The UUID of the agent")
	value := flag.String("value", "", "The value to process")

	// Parse command-line arguments
	flag.Parse()
	if *agentUUID == "" || *value == "" {
		fmt.Println("Usage: go run main.go -uuid <UUID> -value <VALUE>")
		return
	}
	config := Config{
		S3BucketName: getEnv("S3_BUCKET_NAME"),
		S3Region:     getEnv("S3_REGION"),
		S3Endpoint:   getEnv("S3_ENDPOINT"),
		S3AccessKey:  getEnv("S3_ACCESS_KEY"),
		S3SecretKey:  getEnv("S3_SECRET_KEY"),
		SQSQueueURL:  getEnv("SQS_QUEUE_URL"),
		SQSEndpoint:  getEnv("SQS_ENDPOINT"),
		SQSAccessKey: getEnv("SQS_ACCESS_KEY"),
		SQSSecretKey: getEnv("SQS_SECRET_KEY"),
	}

	// Create an S3 client
	// s3Client := s3.NewFromConfig(aws.Config{
	// 	Region:       config.S3Region,
	// 	BaseEndpoint: aws.String(config.S3Endpoint),
	// 	Credentials:  credentials.NewStaticCredentialsProvider(config.S3AccessKey, config.S3SecretKey, ""),
	// })

	// Create an SQS client
	sqsClient := sqs.NewFromConfig(aws.Config{
		Region:       config.S3Region,
		BaseEndpoint: aws.String(config.SQSEndpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(config.SQSAccessKey, config.SQSSecretKey, ""),
	})

	ctx := context.Background()

	taskBody := fmt.Sprintf(`{"task_id":"%s", "operation":"process_data", "data":"some_data"}`, *value)

	input := &sqs.SendMessageInput{
		QueueUrl:    &config.SQSQueueURL,
		MessageBody: &taskBody,
		MessageAttributes: map[string]types.MessageAttributeValue{
			"agent_uuid": {
				DataType:    aws.String("String"),
				StringValue: aws.String(*agentUUID),
			},
		},
	}
	_, err = sqsClient.SendMessage(ctx, input)
	if err != nil {
		log.Fatalf("failed to send message to SQS, %v", err)
	}
}
