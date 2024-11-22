package main

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hvapi/pkg/hvlib/utils"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

type HttpResp struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

type Server struct {
	*utils.Server
	Config   Config
	S3Client *s3.Client
	DB       *sql.DB
}

type Config struct {
	AuthToken      string
	HvApiAuthToken string
	HvApiUrl       string
	S3BucketName   string
	S3Region       string
	S3Endpoint     string
	S3AccessKey    string
	S3SecretKey    string
}

type UploadResponse struct {
	UploadURL string `json:"upload_url"`
	FileID    string `json:"file_id"`    // UUID
	Key       string `json:"key"`        // S3 key
	ExpiresIn int64  `json:"expires_in"` // Expiration in seconds
}

type FileInfo struct {
	ID         uuid.UUID `json:"id"`
	S3Key      string    `json:"s3_key"`
	Filename   string    `json:"filename,omitempty"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
	IsUploaded bool      `json:"is_uploaded"`
	Sha1       string    `json:"sha1,omitempty"`
	Sha256     string    `json:"sha256,omitempty"`
}

func getEnv(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		log.Fatalf("%s must be set", key)
	}
	return value
}

func main() {
	// Set up logging
	logger := log.New()
	// logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetFormatter(&log.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	if err := godotenv.Load(); err != nil {
		logrus.Warning("Error loading .env file")
	}

	config := Config{
		AuthToken:      getEnv("AUTH_TOKEN"),
		HvApiAuthToken: getEnv("HV_API_AUTH_TOKEN"),
		HvApiUrl:       getEnv("HV_API_URL"),
		S3BucketName:   getEnv("S3_BUCKET_NAME"),
		S3Region:       getEnv("S3_REGION"),
		S3Endpoint:     getEnv("S3_ENDPOINT"),
		S3AccessKey:    getEnv("S3_ACCESS_KEY"),
		S3SecretKey:    getEnv("S3_SECRET_KEY"),
	}

	s3Client := s3.NewFromConfig(aws.Config{
		Region:       config.S3Region,
		BaseEndpoint: aws.String(config.S3Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(config.S3AccessKey, config.S3SecretKey, ""),
	})

	// Initialize SQLite database
	dbPath := "./api.db"
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logger.Fatalf("Failed to connect to SQLite: %v", err)
	}
	defer db.Close()

	// Create messages table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS file_uploads (
        id UUID PRIMARY KEY,
        s3_key TEXT,
        filename TEXT DEFAULT "",
        created_at DATETIME ,
        expires_at DATETIME ,
        updated_at DATETIME ,
        is_uploaded BOOLEAN,
        sha1 TEXT DEFAULT "",
        sha256 TEXT DEFAULT ""
    );`)
	if err != nil {
		logger.Fatalf("Failed to create table: %v", err)
	}

	server := &Server{
		Server:   &utils.Server{Logger: logger},
		Config:   config,
		S3Client: s3Client,
		DB:       db,
	}

	// Create a new router
	router := mux.NewRouter()

	// Define routes
	router.HandleFunc("/hello", server.helloHandler).Methods("GET")
	router.HandleFunc("/upload/presign", server.getPresignedURLHandler).Methods("GET")
	router.HandleFunc("/upload/presign", server.getPresignedURLHandler).Methods("GET")
	router.HandleFunc("/upload/{file_id}/finish", server.finishUploadHandler).Methods("GET")
	router.HandleFunc("/files", server.getFilesHandler).Methods("GET")
	router.HandleFunc("/file/{file_id}", server.updateFileHandler).Methods("PUT")
	router.HandleFunc("/file/{file_id}", server.deleteFileHandler).Methods("DELETE")
	router.HandleFunc("/file/{file_id}", server.getFileHandler).Methods("GET")

	router.Use(server.RequestLoggingMiddleware())
	router.Use(server.AuthMiddleware)

	go server.startWorkerService()
	go server.startBackgroundTask()

	// Start the server
	port := 8081
	logger.Infof("Server listening on :%d", port)
	if err := http.ListenAndServe(
		fmt.Sprintf(":%d", port), router); err != nil {
		logger.Fatal(err)
	}
}

func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logrus.Warn("No Authorization header")
			writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check Bearer token format
		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			logrus.Warn("Invalid Authorization header format")
			writeErrorResponse(w, "Invalid Authorization header format", http.StatusUnauthorized)
			return
		}

		// Validate token
		if headerParts[1] != s.Config.AuthToken {
			logrus.Warn("Invalid token")
			writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Token is valid, proceed with the request
		next.ServeHTTP(w, r)
	})
}

// Handler for VM snapshots
func (s *Server) helloHandler(w http.ResponseWriter, r *http.Request) {
	writeSuccessResponse(w, "hello", nil)
}

func (s *Server) getFilesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.Query(`
        SELECT id, filename, s3_key, created_at, expires_at, updated_at, is_uploaded, sha1, sha256
        FROM file_uploads 
        ORDER BY created_at DESC
    `)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to query uploads")
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
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
			logrus.WithError(err).Error("Failed to scan row")
			continue
		}

		uploads = append(uploads, upload)

	}
	writeSuccessResponse(w, "", uploads)
}

func (s *Server) getPresignedURLHandler(w http.ResponseWriter, r *http.Request) {
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
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(fileID, s3Key)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to insert upload record")
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Generate presigned URL
	presignedReq, err := presignClient.PresignPutObject(context.Background(),
		putObjectInput,
		s3.WithPresignExpires(time.Minute*15)) // URL expires in 15 minutes
	if err != nil {
		s.Logger.WithError(err).Error("Failed to generate presigned URL")
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := UploadResponse{
		UploadURL: presignedReq.URL,
		FileID:    fileID,
		Key:       s3Key,
		ExpiresIn: int64(expiresIn),
	}

	// Log the file upload request
	s.Logger.WithFields(logrus.Fields{
		"file_id": fileID,
		"s3_key":  s3Key,
	}).Info("Generated presigned URL for file upload")

	// Write success response
	writeSuccessResponse(w, "", response)
}

func fileExistsInS3(s *Server, key string) (bool, error) {
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

func (s *Server) finishUploadHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["file_id"]

	var s3Key string
	err := s.DB.QueryRow("SELECT s3_key FROM file_uploads WHERE id = ?", fileID).Scan(&s3Key)
	if err != nil {
		writeErrorResponse(w, "File id not found", http.StatusNotFound)
		s.Logger.WithFields(logrus.Fields{
			"file_id": fileID,
			"s3_key":  s3Key,
		}).Info("Not found in DB")
		return
	}

	exist, err := fileExistsInS3(s, s3Key)
	if err != nil {
		s.Logger.WithFields(logrus.Fields{
			"file_id": fileID,
			"s3_key":  s3Key,
		}).WithError(err).Error("Not found in DB")
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !exist {
		s.Logger.WithFields(logrus.Fields{
			"file_id": fileID,
			"s3_key":  s3Key,
		}).Info("Not found in S3")
		writeErrorResponse(w, "File not found in the bucket", http.StatusNotFound)
		return
	}

	stmt, err := s.DB.Prepare(`
        UPDATE file_uploads
        SET is_uploaded = true,updated_at = datetime('now')
        WHERE id = ?
    `)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to prepare statement")
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(fileID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to update upload record")
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Logger.WithFields(logrus.Fields{
		"file_id": fileID,
	}).Info("user finish to upload")

	writeSuccessResponse(w, "", nil)
}

func (s *Server) updateFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["file_id"]

	var params struct {
		Filename string `json:"filename"`
	}

	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		s.Logger.WithError(err).Error("Failed to decode request body")
		writeErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	stmt, err := s.DB.Prepare(`
        UPDATE file_uploads
        SET filename = ?, updated_at = datetime('now')
        WHERE id = ?
    `)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to prepare statement")
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(params.Filename, fileID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to update upload record")
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Logger.WithFields(logrus.Fields{
		"file_id":  fileID,
		"filename": params.Filename,
	}).Info("Updated upload record")

	writeSuccessResponse(w, "Upload record updated", nil)
}

func (s *Server) getFileHandler(w http.ResponseWriter, r *http.Request) {
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
			writeErrorResponse(w, "File not found", http.StatusNotFound)
		} else {
			s.Logger.WithError(err).Error("Failed to query file")
			writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	s.Logger.WithFields(logrus.Fields{"id": upload.ID}).Info("file info")
	writeSuccessResponse(w, "", upload)
}

func (s *Server) deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["file_id"]

	var s3Key string
	err := s.DB.QueryRow("SELECT s3_key FROM file_uploads WHERE id = ?", fileID).Scan(&s3Key)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to get s3_key for file_id")
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	tx, err := s.DB.Begin()
	if err != nil {
		s.Logger.WithError(err).Error("Failed to begin transaction")
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec("DELETE FROM file_uploads WHERE id = ?", fileID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to delete record")
		tx.Rollback()
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = s.S3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(s.Config.S3BucketName),
		Key:    aws.String(s3Key),
	})
	if err != nil {
		s.Logger.WithError(err).Error("Failed to delete S3 object")
		tx.Rollback()
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		s.Logger.WithError(err).Error("Failed to commit transaction")
		writeErrorResponse(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	s.Logger.WithFields(logrus.Fields{
		"file_id": fileID,
		"s3_key":  s3Key,
	}).Info("Deleted file upload")

	writeSuccessResponse(w, "File upload deleted", nil)
}

func (s *Server) startWorkerService() {
	logger := logrus.New()
	logger.Infof("Worker service started")

	for {
		// Fetch files needing hash calculation
		rows, err := s.DB.Query(`
            SELECT id, s3_key
            FROM file_uploads
            WHERE sha256 == "" AND is_uploaded IS true
        `)
		if err != nil {
			logger.WithError(err).Error("Failed to query uploads needing hash calculation")
			time.Sleep(1 * time.Minute)
			continue
		}

		var files []FileInfo
		for rows.Next() {
			var file FileInfo
			if err := rows.Scan(&file.ID, &file.S3Key); err != nil {
				logger.WithError(err).Error("Failed to scan row")
				continue
			}
			files = append(files, file)
		}
		rows.Close()

		// Process each file
		for _, file := range files {
			if err := s.processFile(file); err != nil {
				logger.WithFields(logrus.Fields{
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
	stmt, err := s.DB.Prepare(`
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

	logrus.WithFields(logrus.Fields{
		"file_id": file.ID,
		"s3_key":  file.S3Key,
		"sha1":    hash1,
		"sha256":  hash256,
	}).Info("Updated file with hash")

	return nil
}

func (s *Server) startBackgroundTask() {
	for {
		s.cleanupExpiredEntries()
		time.Sleep(15 * time.Minute)
	}
}

func (s *Server) cleanupExpiredEntries() {
	rows, err := s.DB.Query(`
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
	tx, err := s.DB.Begin()
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

	s.Logger.WithFields(logrus.Fields{
		"id":     id,
		"s3_key": s3Key,
	}).Info("Deleted expired file upload")
}

// Helper: Write JSON response

func writeJSONResponse(w http.ResponseWriter, httpStatus int, data HttpResp) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	json.NewEncoder(w).Encode(data)
}

func writeSuccessResponse(w http.ResponseWriter, message string, data interface{}) {
	writeJSONResponse(w,
		http.StatusOK,
		HttpResp{Status: "success", Data: data, Message: message})
}

// Helper: Write error response
func writeErrorResponse(w http.ResponseWriter, message string, httpStatus int) {
	writeJSONResponse(w,
		httpStatus,
		HttpResp{Status: "error", Data: nil, Message: message})
}
