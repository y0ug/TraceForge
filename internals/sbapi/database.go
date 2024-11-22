package sbapi

import (
	"context"
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

// Calling db.Close() should be deferred by the caller
func NewDb(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// logger.Fatalf("Failed to connect to SQLite: %v", err)
	return &DB{DB: db}, nil
}

func (d *DB) CreateTables() error {
	// Create messages table
	_, err := d.DB.Exec(
		`CREATE TABLE IF NOT EXISTS file_uploads (
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
	return err
}

func (d *DB) GetFiles(ctx context.Context) ([]FileInfo, error) {
	rows, err := d.DB.QueryContext(ctx, `
        SELECT id, filename, s3_key, created_at, expires_at, updated_at, is_uploaded, sha1, sha256
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
			&file.ExpiresAt,
			&file.UpdatedAt,
			&file.IsUploaded,
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
        SELECT id, filename, s3_key, created_at, expires_at, updated_at, is_fileed ,sha1, sha256
        FROM file_uploads
        WHERE id = ?
    `, fileID).Scan(
		&file.ID,
		&file.Filename,
		&file.S3Key,
		&file.CreatedAt,
		&file.ExpiresAt,
		&file.UpdatedAt,
		&file.IsUploaded,
		&file.Sha1,
		&file.Sha256,
	)
	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (d *DB) DeleteFile(ctx context.Context, fileID string) error {
	_, err := d.DB.ExecContext(ctx, "DELETE FROM file_uploads WHERE id = ?", fileID)
	return err
}