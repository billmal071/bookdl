package db

import (
	"database/sql"
	"os"
	"path/filepath"

	"github.com/billmal071/bookdl/internal/config"
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
    verified        INTEGER DEFAULT 0,
    priority        INTEGER DEFAULT 0,
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

CREATE TABLE IF NOT EXISTS bookmarks (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    md5_hash        TEXT UNIQUE NOT NULL,
    title           TEXT NOT NULL,
    authors         TEXT,
    publisher       TEXT,
    year            TEXT,
    language        TEXT,
    format          TEXT,
    size            TEXT,
    page_url        TEXT,
    notes           TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_bookmarks_hash ON bookmarks(md5_hash);

CREATE TABLE IF NOT EXISTS search_history (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    query           TEXT NOT NULL,
    result_count    INTEGER DEFAULT 0,
    filters         TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_search_history_query ON search_history(query);
CREATE INDEX IF NOT EXISTS idx_search_history_created ON search_history(created_at DESC);

CREATE TABLE IF NOT EXISTS search_cache (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    cache_key       TEXT UNIQUE NOT NULL,
    query           TEXT NOT NULL,
    filters         TEXT,
    results_json    TEXT NOT NULL,
    result_count    INTEGER DEFAULT 0,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at      DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_search_cache_key ON search_cache(cache_key);
CREATE INDEX IF NOT EXISTS idx_search_cache_expires ON search_cache(expires_at);
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

	// Run migrations
	if err := runMigrations(db); err != nil {
		return err
	}

	return nil
}

// runMigrations applies database migrations
func runMigrations(db *sql.DB) error {
	// Migration 1: Add verified column if it doesn't exist
	var verifiedCount int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('downloads') WHERE name='verified'").Scan(&verifiedCount)
	if err != nil {
		return err
	}

	if verifiedCount == 0 {
		_, err := db.Exec("ALTER TABLE downloads ADD COLUMN verified INTEGER DEFAULT 0")
		if err != nil {
			return err
		}
	}

	// Migration 2: Add priority column if it doesn't exist
	var priorityCount int
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('downloads') WHERE name='priority'").Scan(&priorityCount)
	if err != nil {
		return err
	}

	if priorityCount == 0 {
		// Add priority column to existing databases
		_, err := db.Exec("ALTER TABLE downloads ADD COLUMN priority INTEGER DEFAULT 0")
		if err != nil {
			return err
		}
		// Create index
		_, err = db.Exec("CREATE INDEX IF NOT EXISTS idx_downloads_priority ON downloads(priority DESC)")
		if err != nil {
			return err
		}
	}

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
