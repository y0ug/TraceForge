package sbapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

// Calling db.Close() should be deferred by the caller
func NewDb(connStr string) (*DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	return &DB{DB: db}, nil
}

func (d *DB) CreateTables() error {
	// Create messages table
	_, err := d.DB.Exec(`
    CREATE TABLE IF NOT EXISTS file_uploads (
        id UUID PRIMARY KEY,
        s3_key TEXT,
        filename TEXT DEFAULT '',
        created_at TIMESTAMP,
        updated_at TIMESTAMP,
        sha1 TEXT DEFAULT '',
        sha256 TEXT DEFAULT ''
    );
    CREATE TABLE IF NOT EXISTS analysis_tasks (
      id UUID PRIMARY KEY,
      file_id UUID NOT NULL REFERENCES file_uploads(id),
      agent_id UUID NOT NULL,
      plugin TEXT NOT NULL,
      status TEXT NOT NULL DEFAULT 'pending',
      args JSONB,
      result JSONB,
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  `)
	return err
}

func (d *DB) GetFiles(ctx context.Context) ([]FileInfo, error) {
	rows, err := d.DB.QueryContext(ctx, `
        SELECT id, filename, s3_key, created_at,  updated_at, sha1, sha256
        FROM file_uploads 
        ORDER BY created_at DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []FileInfo
	for rows.Next() {
		var file FileInfo
		if err := rows.Scan(&file.ID,
			&file.Filename,
			&file.S3Key,
			&file.CreatedAt,
			&file.UpdatedAt,
			&file.Sha1,
			&file.Sha256,
		); err != nil {
			return nil, err
		}

		files = append(files, file)
	}
	return files, nil
}

func (d *DB) GetFile(ctx context.Context, fileID string) (*FileInfo, error) {
	var file FileInfo
	err := d.DB.QueryRowContext(ctx, `
        SELECT id, filename, s3_key, created_at, updated_at, sha1, sha256
        FROM file_uploads
        WHERE id = $1 
    `, fileID).Scan(
		&file.ID,
		&file.Filename,
		&file.S3Key,
		&file.CreatedAt,
		&file.UpdatedAt,
		&file.Sha1,
		&file.Sha256,
	)
	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (d *DB) DeleteFile(ctx context.Context, fileID string) error {
	_, err := d.DB.ExecContext(ctx, "DELETE FROM file_uploads WHERE id = $1", fileID)
	return err
}

func (d *DB) GetAllS3Keys(ctx context.Context) ([]string, error) {
	rows, err := d.DB.QueryContext(ctx, `SELECT s3_key FROM file_uploads`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var s3Keys []string
	for rows.Next() {
		var s3Key string
		if err := rows.Scan(&s3Key); err != nil {
			return nil, err
		}
		s3Keys = append(s3Keys, s3Key)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return s3Keys, nil
}

// CreateAnalysisTask inserts a new analysis task into the database
func (d *DB) CreateAnalysisTask(ctx context.Context, task AnalysisTask) error {
	_, err := d.DB.ExecContext(ctx, `
        INSERT INTO analysis_tasks (id, file_id, agent_id, plugin, status, args, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    `, task.ID, task.FileID, task.AgentID, task.Plugin, task.Status, task.Args, task.CreatedAt, task.UpdatedAt)
	return err
}

// GetAnalysisTasks retrieves all analysis tasks
func (d *DB) GetAnalysisTasks(ctx context.Context) ([]AnalysisTask, error) {
	rows, err := d.DB.QueryContext(ctx, `
        SELECT id, file_id, agent_id, plugin, status, args, result, created_at, updated_at
        FROM analysis_tasks
        ORDER BY created_at DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []AnalysisTask
	for rows.Next() {
		var task AnalysisTask
		var result sql.NullString
		err := rows.Scan(&task.ID, &task.FileID, &task.AgentID, &task.Plugin, &task.Status, &task.Args, &result, &task.CreatedAt, &task.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if result.Valid {
			task.Result = json.RawMessage(result.String)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// UpdateAnalysisTaskStatus updates the status and result of an analysis task
func (d *DB) UpdateAnalysisTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error {
	_, err := d.DB.ExecContext(ctx, `
        UPDATE analysis_tasks
        SET  status = $1, updated_at = $2
        WHERE id = $3
    `, status, time.Now(), taskID)
	return err
}

func (d *DB) UpdateAnalysisTaskResults(ctx context.Context, taskID uuid.UUID, result interface{}) error {
	var resultJSON []byte
	var err error
	if result != nil {
		resultJSON, err = json.Marshal(result)
		if err != nil {
			return err
		}
	}

	_, err = d.DB.ExecContext(ctx, `
        UPDATE analysis_tasks
        SET result = $1, updated_at = $2
        WHERE id = $3
    `, resultJSON, time.Now(), taskID)
	return err
}

func (d *DB) GetNextPendingAnalysisTaskForAgent(ctx context.Context, agentID string) (*AnalysisTask, error) {
	row := d.DB.QueryRowContext(ctx, `
        SELECT id, file_id, agent_id, plugin, status, args, result, created_at, updated_at
        FROM analysis_tasks
        WHERE status = 'pending' AND agent_id = $1
        ORDER BY created_at ASC
        LIMIT 1
    `, agentID)

	var task AnalysisTask
	var result sql.NullString
	err := row.Scan(&task.ID, &task.FileID, &task.AgentID, &task.Plugin, &task.Status, &task.Args, &result, &task.CreatedAt, &task.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if result.Valid {
		task.Result = json.RawMessage(result.String)
	}
	return &task, nil
}

// GetPendingAnalysisTasks retrieves analysis tasks with 'pending' status
func (d *DB) GetPendingAnalysisTasks(ctx context.Context) ([]AnalysisTask, error) {
	rows, err := d.DB.QueryContext(ctx, `
        SELECT id, file_id, agent_id, plugin, status, args, result, created_at, updated_at
        FROM analysis_tasks
        WHERE status = 'pending'
        ORDER BY created_at ASC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []AnalysisTask
	for rows.Next() {
		var task AnalysisTask
		var result sql.NullString
		err := rows.Scan(&task.ID, &task.FileID, &task.AgentID, &task.Plugin, &task.Status, &task.Args, &result, &task.CreatedAt, &task.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if result.Valid {
			task.Result = json.RawMessage(result.String)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}
