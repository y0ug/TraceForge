package sbapi

import (
	"context"
	"database/sql"

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
