package main

import (
	"TraceForge/internals/commons"
	mq "TraceForge/internals/mq"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

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
		dbPath = "./mq.db"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8888"
	}

	// Initialize SQLite database
	db, err := mq.NewDb(dbPath)
	if err != nil {
		logger.WithError(err).Fatalf("Failed to connect to SQLite")
	}
	defer db.Close()

	// Create messages table
	err = mq.CreateTables(db)
	if err != nil {
		logger.WithError(err).Fatalf("Failed to create table")
	}

	// Initialize the server
	server := &mq.ServerSQS{
		DB:     db,
		Server: &commons.Server{Logger: logger},
	}

	// Set up routes
	router := mux.NewRouter()
	router.HandleFunc("/push", server.PushMessageHandler).Methods(http.MethodPost)
	router.HandleFunc("/pull/{agent_id}", server.PullMessageHandler).Methods(http.MethodGet)
	router.HandleFunc("/delete/{message_id}", server.DeleteMessageHandler).Methods(http.MethodDelete)
	router.Use(server.LoggingMiddleware())

	listenOn := fmt.Sprintf(":%s", port)
	logger.Infof("Server listening on %s", listenOn)
	if err := http.ListenAndServe(
		listenOn, router); err != nil {
		logger.Fatal(err)
	}
}
