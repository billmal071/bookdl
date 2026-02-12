package db

import (
	"database/sql"
	"os"
	"path/filepath"

	"github.com/williams/bookdl/internal/config"
	_ "modernc.org/sqlite"
)

var database *sql.DB

const schema = `
CREATE TABLE IF NOT EXISTS downloads (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    md5_hash        TEXT UNIQUE NOT NULL,
    title           TEXT NOT NULL,
    authors         TEXT,
    publisher       TEXT,
    language        TEXT,
    format          TEXT NOT NULL,
    file_size       INTEGER,
    downloaded_size INTEGER DEFAULT 0,
    source_url      TEXT NOT NULL,
    download_url    TEXT,
    file_path       TEXT,
    temp_path       TEXT,
    status          TEXT DEFAULT 'pending',
    error_message   TEXT,
    retry_count     INTEGER DEFAULT 0,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at    DATETIME
);

CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
CREATE INDEX IF NOT EXISTS idx_downloads_hash ON downloads(md5_hash);

CREATE TABLE IF NOT EXISTS chunks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    download_id     INTEGER NOT NULL,
    chunk_index     INTEGER NOT NULL,
    start_byte      INTEGER NOT NULL,
    end_byte        INTEGER NOT NULL,
    downloaded      INTEGER DEFAULT 0,
    status          TEXT DEFAULT 'pending',
    FOREIGN KEY (download_id) REFERENCES downloads(id) ON DELETE CASCADE,
    UNIQUE(download_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_chunks_download ON chunks(download_id);
`

// Init initializes the database connection and schema
func Init() error {
	dbPath := config.GetDBPath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}

	// Create schema
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	database = db
	return nil
}

// DB returns the database connection
func DB() *sql.DB {
	return database
}

// Close closes the database connection
func Close() error {
	if database != nil {
		return database.Close()
	}
	return nil
}
