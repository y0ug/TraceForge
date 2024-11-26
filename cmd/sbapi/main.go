package main

import (
	"TraceForge/internals/commons"
	"TraceForge/internals/mq"
	"TraceForge/internals/sbapi"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Set up logging
	logger := log.New()
	// logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(log.InfoLevel)

	if err := godotenv.Load(); err != nil {
		log.Warning("Error loading .env file")
	}

	dbConnStr := os.Getenv("DB_URI")
	if dbConnStr == "" {
		logger.Fatal("DB_URI environment variable is required")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		logger.Fatal("REDIS_URL environment variable is required")
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.WithError(err).Fatal("Failed to parse REDIS_URL")
	}

	redisClient := redis.NewClient(opt)
	config := sbapi.Config{
		AuthToken:    commons.GetEnv("AUTH_TOKEN"),
		S3BucketName: commons.GetEnv("S3_BUCKET_NAME"),
		S3Region:     commons.GetEnv("S3_REGION"),
		S3Endpoint:   commons.GetEnv("S3_ENDPOINT"),
		S3AccessKey:  commons.GetEnv("S3_ACCESS_KEY"),
		S3SecretKey:  commons.GetEnv("S3_SECRET_KEY"),
		MqURL:        commons.GetEnv("MQ_URL"),
	}

	s3Client := s3.NewFromConfig(aws.Config{
		Region:       config.S3Region,
		BaseEndpoint: aws.String(config.S3Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(config.S3AccessKey, config.S3SecretKey, ""),
	})

	// Initialize SQLite database
	db, err := sbapi.NewDb(dbConnStr)
	if err != nil {
		logger.WithError(err).Fatal("Failed NewDB")
	}
	defer db.DB.Close()

	err = db.CreateTables()
	if err != nil {
		logger.WithError(err).Fatal("Failed to create tables")
	}

	taskManager := sbapi.NewTaskManager()
	taskManager.Start()
	defer taskManager.Stop()

	agentsConfig, err := sbapi.LoadAgentsConfig("agents.toml")
	if err != nil {
		logger.WithError(err).Fatal("Failed to load agents config")
	}
	// Now you can access your configuration
	for name, hvapi := range agentsConfig.Hvapi {
		logger.Infof("HvAPI config  %s: %s\n", name, hvapi.URL)
	}

	for _, agent := range agentsConfig.Agents {
		logger.Infof("Agent: %s: %+v", agent.Name, agent)
	}

	mqClient := mq.NewClient(config.MqURL)
	server := &sbapi.Server{
		Server:       &commons.Server{Logger: logger},
		Config:       config,
		S3Client:     s3Client,
		DB:           db,
		RedisClient:  redisClient,
		TaskManager:  taskManager,
		AgentsConfig: agentsConfig,
		MQClient:     mqClient,
	}

	_, err = taskManager.AddTask("CleanupTask", "* * * * *", server.CleanOrphanFiles)
	if err != nil {
		logger.WithError(err).Fatal("Failed to add CleanupTask")
	}

	for _, agent := range agentsConfig.Agents {
		name := fmt.Sprintf("AgentTaskWorker-%s", agent.ID)
		go server.StartAgentTaskWorker(agent.ID)
		_, err = taskManager.AddTask(name, "", server.WrapStartAgentTaskWorker(agent.ID))
		if err != nil {
			logger.WithError(err).Fatalf("Failed to add %s", name)
		}
	}

	// Create a new router
	router := mux.NewRouter()

	// Define routes
	router.HandleFunc("/upload/presign", server.GetPresignedURLHandler).Methods("GET")
	router.HandleFunc("/upload/presign", server.GetPresignedURLHandler).Methods("GET")
	router.HandleFunc("/upload/{file_id}/complete", server.CompleteUploadHandler).Methods("GET")
	router.HandleFunc("/files", server.GetFilesHandler).Methods("GET")
	router.HandleFunc("/file/{file_id}", server.UpdateFileHandler).Methods("PUT")
	router.HandleFunc("/file/{file_id}", server.DeleteFileHandler).Methods("DELETE")
	router.HandleFunc("/file/{file_id}", server.GetFileHandler).Methods("GET")
	router.HandleFunc("/file/{file_id}/dl", server.GetFileDlHandler).Methods("GET")

	router.HandleFunc("/tasks", server.TasksHandler).Methods("GET")
	router.HandleFunc("/tasks/{task_name}/run", server.RunTaskHandler).Methods("GET")

	router.HandleFunc("/analysis_tasks", server.CreateAnalysisTaskHandler).Methods("POST")
	router.HandleFunc("/analysis_tasks", server.GetAnalysisTasksHandler).Methods("GET")

	router.HandleFunc("/agents", server.GetAgentsHandler).Methods("GET")
	router.Use(server.LoggingMiddleware())
	router.Use(server.AuthMiddleware)

	// go server.CleanupTask()
	// go server.HasherTask()

	// Start the server
	listenOn := fmt.Sprintf(":%s", port)
	logger.Infof("Server listening on %s", listenOn)
	if err := http.ListenAndServe(
		listenOn, router); err != nil {
		logger.Fatal(err)
	}
}
