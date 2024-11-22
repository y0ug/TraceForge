package sbapi

import "database/sql"

func CreateTables(db *sql.DB) error {
	// Create messages table
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS file_uploads (
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
