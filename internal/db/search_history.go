package db

import (
	"encoding/json"
	"time"
)

// SearchHistory represents a saved search query
type SearchHistory struct {
	ID          int64
	Query       string
	ResultCount int
	Filters     SearchFilters
	CreatedAt   time.Time
}

// SearchFilters stores the filters used in a search
type SearchFilters struct {
	Format   string `json:"format,omitempty"`
	Language string `json:"language,omitempty"`
	Year     string `json:"year,omitempty"`
	MaxSize  string `json:"max_size,omitempty"`
}

// AddSearchHistory adds a search to history
func AddSearchHistory(query string, resultCount int, filters SearchFilters) error {
	filtersJSON, err := json.Marshal(filters)
	if err != nil {
		filtersJSON = []byte("{}")
	}

	_, err = database.Exec(`
		INSERT INTO search_history (query, result_count, filters)
		VALUES (?, ?, ?)`,
		query, resultCount, string(filtersJSON),
	)
	return err
}

// GetSearchHistory retrieves recent search history
func GetSearchHistory(limit int) ([]*SearchHistory, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := database.Query(`
		SELECT id, query, result_count, filters, created_at
		FROM search_history
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []*SearchHistory
	for rows.Next() {
		h := &SearchHistory{}
		var filtersJSON string
		err := rows.Scan(&h.ID, &h.Query, &h.ResultCount, &filtersJSON, &h.CreatedAt)
		if err != nil {
			return nil, err
		}

		// Parse filters JSON
		if filtersJSON != "" {
			json.Unmarshal([]byte(filtersJSON), &h.Filters)
		}

		history = append(history, h)
	}
	return history, rows.Err()
}

// GetUniqueSearchHistory retrieves unique recent searches (no duplicates)
func GetUniqueSearchHistory(limit int) ([]*SearchHistory, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := database.Query(`
		SELECT id, query, result_count, filters, created_at
		FROM search_history
		WHERE id IN (
			SELECT MAX(id) FROM search_history GROUP BY query
		)
		ORDER BY created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []*SearchHistory
	for rows.Next() {
		h := &SearchHistory{}
		var filtersJSON string
		err := rows.Scan(&h.ID, &h.Query, &h.ResultCount, &filtersJSON, &h.CreatedAt)
		if err != nil {
			return nil, err
		}

		if filtersJSON != "" {
			json.Unmarshal([]byte(filtersJSON), &h.Filters)
		}

		history = append(history, h)
	}
	return history, rows.Err()
}

// ClearSearchHistory removes all search history
func ClearSearchHistory() error {
	_, err := database.Exec(`DELETE FROM search_history`)
	return err
}

// DeleteSearchHistoryOlderThan removes history older than the given duration
func DeleteSearchHistoryOlderThan(d time.Duration) error {
	cutoff := time.Now().Add(-d)
	_, err := database.Exec(`DELETE FROM search_history WHERE created_at < ?`, cutoff)
	return err
}
