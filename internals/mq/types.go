package mq

import (
	"TraceForge/internals/commons"
	"database/sql"
	"time"

	"github.com/sirupsen/logrus"
)

type Client struct {
	serverURL string
	// agentID   string
	logger *logrus.Logger
}

type ServerSQS struct {
	DB              *sql.DB
	*commons.Server // Embedding utils.Server
}

type Message struct {
	ID        string    `json:"id"`
	QueueID   string    `json:"queue_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	VisibleAt time.Time `json:"visible_at"`
}

type MessageResponse struct {
	ID        string    `json:"id"`
	QueueID   string    `json:"queue_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
