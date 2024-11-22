package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"hvapi/pkg/hvlib/utils"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

type ServerSQS struct {
	DB            *sql.DB
	*utils.Server // Embedding utils.Server
}

type Message struct {
	ID        int       `json:"id"`
	AgentID   string    `json:"agent_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	VisibleAt time.Time `json:"visible_at"`
}

func main() {
	// Set up logging
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	// Load configuration
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./sqs.db"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8888"
	}

	// Initialize SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logger.Fatalf("Failed to connect to SQLite: %v", err)
	}
	defer db.Close()

	// Create messages table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		agent_id TEXT NOT NULL,
		body TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		visible_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		logger.Fatalf("Failed to create table: %v", err)
	}

	// Initialize the server
	server := &ServerSQS{
		DB:     db,
		Server: &utils.Server{Logger: logger},
	}

	// Set up routes
	router := mux.NewRouter()
	router.HandleFunc("/push", server.PushMessage).Methods(http.MethodPost)
	router.HandleFunc("/pull", server.PullMessage).Methods(http.MethodGet)
	router.HandleFunc("/delete", server.DeleteMessage).Methods(http.MethodDelete)
	router.Use(server.RequestLoggingMiddleware())

	logger.Infof("Server listening on :%s", port)
	if err := http.ListenAndServe(
		fmt.Sprintf(":%d", port), router); err != nil {
		logrus.Fatal(err)
	}
}

// PushMessage handles adding messages to a specific agent's queue
func (s *ServerSQS) PushMessage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AgentID string `json:"agent_id"`
		Body    string `json:"body"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.Logger.WithError(err).Error("Invalid request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := s.DB.Exec(`INSERT INTO messages (agent_id, body) VALUES (?, ?)`, body.AgentID, body.Body)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to save message")
		http.Error(w, "Failed to save message", http.StatusInternalServerError)
		return
	}

	s.Logger.WithFields(logrus.Fields{
		"agent_id": body.AgentID,
		"body":     body.Body,
	}).Info("Message pushed successfully")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintln(w, "Message pushed successfully")
}

// PullMessage handles fetching the next message for an agent
func (s *ServerSQS) PullMessage(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		s.Logger.Warn("Missing agent_id parameter")
		http.Error(w, "Missing agent_id parameter", http.StatusBadRequest)
		return
	}

	var msg Message
	err := s.DB.QueryRow(`SELECT id, agent_id, body, created_at, visible_at FROM messages 
		WHERE agent_id = ? AND visible_at <= CURRENT_TIMESTAMP 
		ORDER BY created_at ASC LIMIT 1`, agentID).Scan(&msg.ID, &msg.AgentID, &msg.Body, &msg.CreatedAt, &msg.VisibleAt)
	if err == sql.ErrNoRows {
		s.Logger.WithField("agent_id", agentID).Info("No messages available")
		http.Error(w, "No messages available", http.StatusNotFound)
		return
	} else if err != nil {
		s.Logger.WithError(err).Error("Failed to retrieve message")
		http.Error(w, "Failed to retrieve message", http.StatusInternalServerError)
		return
	}

	// Set visibility timeout (e.g., 30 seconds)
	_, err = s.DB.Exec(`UPDATE messages SET visible_at = DATETIME('now', '+30 seconds') WHERE id = ?`, msg.ID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to update visibility timeout")
		http.Error(w, "Failed to update visibility timeout", http.StatusInternalServerError)
		return
	}

	s.Logger.WithFields(logrus.Fields{
		"agent_id": msg.AgentID,
		"message":  msg.Body,
	}).Info("Message pulled successfully")
	json.NewEncoder(w).Encode(msg)
}

// DeleteMessage handles deleting a processed message
func (s *ServerSQS) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.Logger.WithError(err).Error("Invalid request body")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := s.DB.Exec(`DELETE FROM messages WHERE id = ?`, body.ID)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to delete message")
		http.Error(w, "Failed to delete message", http.StatusInternalServerError)
		return
	}

	s.Logger.WithField("message_id", body.ID).Info("Message deleted successfully")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Message deleted successfully")
}
