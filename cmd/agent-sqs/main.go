package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
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
	AgentUuid    string
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

	flag.Parse()

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
		AgentUuid:    *agentUUID, // getEnv("AGENT_UUID"),
	}

	// Create an S3 client
	s3Client := s3.NewFromConfig(aws.Config{
		Region:       config.S3Region,
		BaseEndpoint: aws.String(config.S3Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(config.S3AccessKey, config.S3SecretKey, ""),
	})

	// Create an SQS client
	sqsClient := sqs.NewFromConfig(aws.Config{
		Region:       config.S3Region,
		BaseEndpoint: aws.String(config.SQSEndpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(config.SQSAccessKey, config.SQSSecretKey, ""),
	})

	pollSQS(context.Background(), sqsClient, s3Client, &config)

	log.Println("Stopping service.")
}

type Task struct {
	TaskID    string `json:"task_id"`
	Operation string `json:"operation"`
	Data      string `json:"data"`
}

func pollSQS(ctx context.Context, sqsClient *sqs.Client, s3Client *s3.Client, config *Config) error {
	log.Println("Polling SQS queue...")

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			resp, err := sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
				QueueUrl:            aws.String(config.SQSQueueURL),
				MaxNumberOfMessages: 10,
				WaitTimeSeconds:     10,
				MessageAttributeNames: []string{
					"All",
				},
			})
			if err != nil {
				log.Printf("failed to receive message, %v", err)
				continue
			}

			// Process each message
			for _, message := range resp.Messages {
				if message.MessageAttributes["agent_uuid"].StringValue != nil &&
					*message.MessageAttributes["agent_uuid"].StringValue == config.AgentUuid {

					// Parse the task
					var task Task
					if err := json.Unmarshal([]byte(*message.Body), &task); err != nil {
						log.Printf("failed to parse message body, %v", err)
						continue
					}

					// Process the task
					fmt.Printf("Agent %s processing task: %+v\n", config.AgentUuid, task)
					_, err = sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
						QueueUrl:      aws.String(config.SQSQueueURL),
						ReceiptHandle: message.ReceiptHandle,
					})
					if err != nil {
						return fmt.Errorf("error deleting message from SQS: %v", err)
					}
				}
			}
		}
	}
}
