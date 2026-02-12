package db

import (
	"database/sql"
	"time"
)

// Bookmark represents a saved book for later
type Bookmark struct {
	ID        int64
	MD5Hash   string
	Title     string
	Authors   string
	Publisher string
	Year      string
	Language  string
	Format    string
	Size      string
	PageURL   string
	Notes     string
	CreatedAt time.Time
}

// CreateBookmark creates a new bookmark
func CreateBookmark(b *Bookmark) error {
	result, err := database.Exec(`
		INSERT INTO bookmarks (
			md5_hash, title, authors, publisher, year, language, format, size, page_url, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.MD5Hash, b.Title, b.Authors, b.Publisher, b.Year, b.Language, b.Format, b.Size, b.PageURL, b.Notes,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	b.ID = id
	return nil
}

// GetBookmark retrieves a bookmark by ID
func GetBookmark(id int64) (*Bookmark, error) {
	b := &Bookmark{}
	err := database.QueryRow(`
		SELECT id, md5_hash, title, authors, publisher, year, language, format, size, page_url, notes, created_at
		FROM bookmarks WHERE id = ?`, id).Scan(
		&b.ID, &b.MD5Hash, &b.Title, &b.Authors, &b.Publisher, &b.Year, &b.Language, &b.Format, &b.Size, &b.PageURL, &b.Notes, &b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GetBookmarkByHash retrieves a bookmark by MD5 hash
func GetBookmarkByHash(hash string) (*Bookmark, error) {
	b := &Bookmark{}
	err := database.QueryRow(`
		SELECT id, md5_hash, title, authors, publisher, year, language, format, size, page_url, notes, created_at
		FROM bookmarks WHERE md5_hash = ?`, hash).Scan(
		&b.ID, &b.MD5Hash, &b.Title, &b.Authors, &b.Publisher, &b.Year, &b.Language, &b.Format, &b.Size, &b.PageURL, &b.Notes, &b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// ListBookmarks retrieves all bookmarks
func ListBookmarks() ([]*Bookmark, error) {
	rows, err := database.Query(`
		SELECT id, md5_hash, title, authors, publisher, year, language, format, size, page_url, notes, created_at
		FROM bookmarks
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []*Bookmark
	for rows.Next() {
		b := &Bookmark{}
		err := rows.Scan(
			&b.ID, &b.MD5Hash, &b.Title, &b.Authors, &b.Publisher, &b.Year, &b.Language, &b.Format, &b.Size, &b.PageURL, &b.Notes, &b.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		bookmarks = append(bookmarks, b)
	}
	return bookmarks, rows.Err()
}

// DeleteBookmark deletes a bookmark by ID
func DeleteBookmark(id int64) error {
	_, err := database.Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
	return err
}

// DeleteBookmarkByHash deletes a bookmark by MD5 hash
func DeleteBookmarkByHash(hash string) error {
	_, err := database.Exec(`DELETE FROM bookmarks WHERE md5_hash = ?`, hash)
	return err
}

// BookmarkExists checks if a bookmark exists for the given hash
func BookmarkExists(hash string) bool {
	var count int
	err := database.QueryRow(`SELECT COUNT(*) FROM bookmarks WHERE md5_hash = ?`, hash).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return false
	}
	return count > 0
}

// UpdateBookmarkNotes updates the notes for a bookmark
func UpdateBookmarkNotes(id int64, notes string) error {
	_, err := database.Exec(`UPDATE bookmarks SET notes = ? WHERE id = ?`, notes, id)
	return err
}
