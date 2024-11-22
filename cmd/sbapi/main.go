package main

import (
	"TraceForge/internals/commons"
	"TraceForge/internals/sbapi"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
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

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./api.db"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	config := sbapi.Config{
		AuthToken:      commons.GetEnv("AUTH_TOKEN"),
		HvApiAuthToken: commons.GetEnv("HV_API_AUTH_TOKEN"),
		HvApiUrl:       commons.GetEnv("HV_API_URL"),
		S3BucketName:   commons.GetEnv("S3_BUCKET_NAME"),
		S3Region:       commons.GetEnv("S3_REGION"),
		S3Endpoint:     commons.GetEnv("S3_ENDPOINT"),
		S3AccessKey:    commons.GetEnv("S3_ACCESS_KEY"),
		S3SecretKey:    commons.GetEnv("S3_SECRET_KEY"),
	}

	s3Client := s3.NewFromConfig(aws.Config{
		Region:       config.S3Region,
		BaseEndpoint: aws.String(config.S3Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(config.S3AccessKey, config.S3SecretKey, ""),
	})

	// Initialize SQLite database
	db, err := sbapi.NewDb(dbPath)
	if err != nil {
		logger.WithError(err).Fatal("Failed NewDB")
	}
	defer db.DB.Close()

	err = db.CreateTables()
	if err != nil {
		logger.WithError(err).Fatal("Failed to create tables")
	}

	server := &sbapi.Server{
		Server:   &commons.Server{Logger: logger},
		Config:   config,
		S3Client: s3Client,
		DB:       db,
	}

	// Create a new router
	router := mux.NewRouter()

	// Define routes
	router.HandleFunc("/upload/presign", server.GetPresignedURLHandler).Methods("GET")
	router.HandleFunc("/upload/presign", server.GetPresignedURLHandler).Methods("GET")
	router.HandleFunc("/upload/{file_id}/finish", server.FinishUploadHandler).Methods("GET")
	router.HandleFunc("/files", server.GetFilesHandler).Methods("GET")
	router.HandleFunc("/file/{file_id}", server.UpdateFileHandler).Methods("PUT")
	router.HandleFunc("/file/{file_id}", server.DeleteFileHandler).Methods("DELETE")
	router.HandleFunc("/file/{file_id}", server.GetFileHandler).Methods("GET")
	router.HandleFunc("/file/{file_id}/dl", server.GetFileDlHandler).Methods("GET")

	router.Use(server.LoggingMiddleware())
	router.Use(server.AuthMiddleware)

	go server.CleanupTask()
	go server.HasherTask()

	// Start the server
	listenOn := fmt.Sprintf(":%s", port)
	logger.Infof("Server listening on %s", listenOn)
	if err := http.ListenAndServe(
		listenOn, router); err != nil {
		logger.Fatal(err)
	}
}
