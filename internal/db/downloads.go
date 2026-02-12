package db

import (
	"database/sql"
	"time"
)

// DownloadStatus represents the state of a download
type DownloadStatus string

const (
	StatusPending     DownloadStatus = "pending"
	StatusDownloading DownloadStatus = "downloading"
	StatusPaused      DownloadStatus = "paused"
	StatusCompleted   DownloadStatus = "completed"
	StatusFailed      DownloadStatus = "failed"
)

// Download represents a download record
type Download struct {
	ID             int64
	MD5Hash        string
	Title          string
	Authors        string
	Publisher      string
	Language       string
	Format         string
	FileSize       int64
	DownloadedSize int64
	SourceURL      string
	DownloadURL    string
	FilePath       string
	TempPath       string
	Status         DownloadStatus
	ErrorMessage   string
	RetryCount     int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CompletedAt    *time.Time
}

// Chunk represents a download chunk for resumable downloads
type Chunk struct {
	ID          int64
	DownloadID  int64
	ChunkIndex  int
	StartByte   int64
	EndByte     int64
	Downloaded  int64
	Status      string
}

// CreateDownload creates a new download record
func CreateDownload(d *Download) error {
	result, err := database.Exec(`
		INSERT INTO downloads (
			md5_hash, title, authors, publisher, language, format,
			file_size, source_url, download_url, file_path, temp_path, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.MD5Hash, d.Title, d.Authors, d.Publisher, d.Language, d.Format,
		d.FileSize, d.SourceURL, d.DownloadURL, d.FilePath, d.TempPath, d.Status,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	d.ID = id
	return nil
}

// GetDownload retrieves a download by ID
func GetDownload(id int64) (*Download, error) {
	d := &Download{}
	err := database.QueryRow(`
		SELECT id, md5_hash, title, authors, publisher, language, format,
			file_size, downloaded_size, source_url, download_url, file_path,
			temp_path, status, error_message, retry_count, created_at, updated_at, completed_at
		FROM downloads WHERE id = ?`, id).Scan(
		&d.ID, &d.MD5Hash, &d.Title, &d.Authors, &d.Publisher, &d.Language, &d.Format,
		&d.FileSize, &d.DownloadedSize, &d.SourceURL, &d.DownloadURL, &d.FilePath,
		&d.TempPath, &d.Status, &d.ErrorMessage, &d.RetryCount, &d.CreatedAt, &d.UpdatedAt, &d.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// GetDownloadByHash retrieves a download by MD5 hash
func GetDownloadByHash(hash string) (*Download, error) {
	d := &Download{}
	err := database.QueryRow(`
		SELECT id, md5_hash, title, authors, publisher, language, format,
			file_size, downloaded_size, source_url, download_url, file_path,
			temp_path, status, error_message, retry_count, created_at, updated_at, completed_at
		FROM downloads WHERE md5_hash = ?`, hash).Scan(
		&d.ID, &d.MD5Hash, &d.Title, &d.Authors, &d.Publisher, &d.Language, &d.Format,
		&d.FileSize, &d.DownloadedSize, &d.SourceURL, &d.DownloadURL, &d.FilePath,
		&d.TempPath, &d.Status, &d.ErrorMessage, &d.RetryCount, &d.CreatedAt, &d.UpdatedAt, &d.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	return d, nil
}

// ListDownloads retrieves downloads filtered by status
func ListDownloads(status DownloadStatus, showAll bool) ([]*Download, error) {
	var rows *sql.Rows
	var err error

	if status != "" {
		rows, err = database.Query(`
			SELECT id, md5_hash, title, authors, publisher, language, format,
				file_size, downloaded_size, source_url, download_url, file_path,
				temp_path, status, error_message, retry_count, created_at, updated_at, completed_at
			FROM downloads WHERE status = ?
			ORDER BY updated_at DESC`, status)
	} else if showAll {
		rows, err = database.Query(`
			SELECT id, md5_hash, title, authors, publisher, language, format,
				file_size, downloaded_size, source_url, download_url, file_path,
				temp_path, status, error_message, retry_count, created_at, updated_at, completed_at
			FROM downloads
			ORDER BY updated_at DESC`)
	} else {
		// By default, don't show completed downloads
		rows, err = database.Query(`
			SELECT id, md5_hash, title, authors, publisher, language, format,
				file_size, downloaded_size, source_url, download_url, file_path,
				temp_path, status, error_message, retry_count, created_at, updated_at, completed_at
			FROM downloads WHERE status != 'completed'
			ORDER BY updated_at DESC`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var downloads []*Download
	for rows.Next() {
		d := &Download{}
		err := rows.Scan(
			&d.ID, &d.MD5Hash, &d.Title, &d.Authors, &d.Publisher, &d.Language, &d.Format,
			&d.FileSize, &d.DownloadedSize, &d.SourceURL, &d.DownloadURL, &d.FilePath,
			&d.TempPath, &d.Status, &d.ErrorMessage, &d.RetryCount, &d.CreatedAt, &d.UpdatedAt, &d.CompletedAt,
		)
		if err != nil {
			return nil, err
		}
		downloads = append(downloads, d)
	}
	return downloads, rows.Err()
}

// UpdateStatus updates the download status
func UpdateStatus(id int64, status DownloadStatus, errMsg string) error {
	_, err := database.Exec(`
		UPDATE downloads SET status = ?, error_message = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, status, errMsg, id)
	return err
}

// UpdateProgress updates the download progress
func UpdateProgress(id int64, downloadedSize int64) error {
	_, err := database.Exec(`
		UPDATE downloads SET downloaded_size = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, downloadedSize, id)
	return err
}

// UpdateDownloadURL updates the download URL
func UpdateDownloadURL(id int64, url string) error {
	_, err := database.Exec(`
		UPDATE downloads SET download_url = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, url, id)
	return err
}

// MarkCompleted marks a download as completed
func MarkCompleted(id int64, filePath string) error {
	_, err := database.Exec(`
		UPDATE downloads SET
			status = 'completed',
			file_path = ?,
			temp_path = NULL,
			completed_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, filePath, id)
	return err
}

// IncrementRetry increments the retry count
func IncrementRetry(id int64) error {
	_, err := database.Exec(`
		UPDATE downloads SET retry_count = retry_count + 1, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, id)
	return err
}

// ResetDownload resets a download for restart
func ResetDownload(id int64) error {
	_, err := database.Exec(`
		UPDATE downloads SET
			downloaded_size = 0,
			retry_count = 0,
			status = 'pending',
			error_message = NULL,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, id)
	if err != nil {
		return err
	}

	// Delete chunks
	_, err = database.Exec(`DELETE FROM chunks WHERE download_id = ?`, id)
	return err
}

// DeleteDownload deletes a download record
func DeleteDownload(id int64) error {
	_, err := database.Exec(`DELETE FROM downloads WHERE id = ?`, id)
	return err
}

// CreateChunks creates chunk records for a download
func CreateChunks(downloadID int64, chunks []*Chunk) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO chunks (download_id, chunk_index, start_byte, end_byte, status)
		VALUES (?, ?, ?, ?, 'pending')`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range chunks {
		result, err := stmt.Exec(downloadID, c.ChunkIndex, c.StartByte, c.EndByte)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		c.ID = id
		c.DownloadID = downloadID
	}

	return tx.Commit()
}

// GetChunks retrieves chunks for a download
func GetChunks(downloadID int64) ([]*Chunk, error) {
	rows, err := database.Query(`
		SELECT id, download_id, chunk_index, start_byte, end_byte, downloaded, status
		FROM chunks WHERE download_id = ?
		ORDER BY chunk_index`, downloadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*Chunk
	for rows.Next() {
		c := &Chunk{}
		err := rows.Scan(&c.ID, &c.DownloadID, &c.ChunkIndex, &c.StartByte, &c.EndByte, &c.Downloaded, &c.Status)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// UpdateChunkProgress updates a chunk's progress
func UpdateChunkProgress(chunkID int64, downloaded int64) error {
	_, err := database.Exec(`
		UPDATE chunks SET downloaded = ? WHERE id = ?`, downloaded, chunkID)
	return err
}

// UpdateProgressAtomic updates both chunk and download progress in a single transaction
// This ensures consistency if the operation is interrupted (e.g., by pause)
func UpdateProgressAtomic(downloadID, chunkID, chunkDownloaded, totalDownloaded int64) error {
	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE chunks SET downloaded = ? WHERE id = ?`, chunkDownloaded, chunkID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`UPDATE downloads SET downloaded_size = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, totalDownloaded, downloadID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// MarkChunkCompleted marks a chunk as completed
func MarkChunkCompleted(chunkID int64) error {
	_, err := database.Exec(`
		UPDATE chunks SET status = 'completed' WHERE id = ?`, chunkID)
	return err
}

// DeleteChunks deletes all chunks for a download
func DeleteChunks(downloadID int64) error {
	_, err := database.Exec(`DELETE FROM chunks WHERE download_id = ?`, downloadID)
	return err
}

// GetIncompleteChunks retrieves incomplete chunks for a download
func GetIncompleteChunks(downloadID int64) ([]*Chunk, error) {
	rows, err := database.Query(`
		SELECT id, download_id, chunk_index, start_byte, end_byte, downloaded, status
		FROM chunks WHERE download_id = ? AND status != 'completed'
		ORDER BY chunk_index`, downloadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*Chunk
	for rows.Next() {
		c := &Chunk{}
		err := rows.Scan(&c.ID, &c.DownloadID, &c.ChunkIndex, &c.StartByte, &c.EndByte, &c.Downloaded, &c.Status)
		if err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}
