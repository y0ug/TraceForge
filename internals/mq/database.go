package mq

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
)

// Calling db.Close() should be deferred by the caller
func NewDb(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	return db, err
}

func CreateTables(db *sql.DB) error {
	// Create messages table
	_, err := db.Exec(
		`CREATE TABLE IF NOT EXISTS messages (
		id UUID PRIMARY KEY ,
		agent_id TEXT NOT NULL,
		body TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		visible_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	return err
}

func DeleteMessage(ctx context.Context, db *sql.DB, messageID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM messages WHERE id = ?`, messageID)
	return err
}

func CreateMessage(ctx context.Context, db *sql.DB, agentID string, body string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO messages (id, agent_id, body) VALUES (?, ?, ?)`, uuid.NewString(), agentID, body)
	return err
}

func GetMessage(ctx context.Context, db *sql.DB, agentID string) (*Message, error) {
	msg := &Message{} // Initialize the pointer

	err := db.QueryRowContext(ctx, `
		SELECT id, agent_id, body, created_at, visible_at 
		FROM messages 
		WHERE agent_id = ? AND visible_at <= CURRENT_TIMESTAMP 
		ORDER BY created_at ASC LIMIT 1`, agentID).
		Scan(&msg.ID, &msg.QueueID, &msg.Body, &msg.CreatedAt, &msg.VisibleAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// No message found
			return nil, nil
		}
		// Other errors
		return nil, err
	}

	return msg, nil
}

func SetMessageVisibility(ctx context.Context, db *sql.DB, messageID string) error {
	_, err := db.ExecContext(ctx, `UPDATE messages SET visible_at = DATETIME('now', '+30 seconds') WHERE id = ?`, messageID)
	return err
}
