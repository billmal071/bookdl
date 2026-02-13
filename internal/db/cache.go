package db

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SearchCacheEntry represents a cached search result
type SearchCacheEntry struct {
	ID          int64
	CacheKey    string
	Query       string
	Filters     string
	ResultsJSON string
	ResultCount int
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// GenerateCacheKey generates a unique cache key from query and filters
func GenerateCacheKey(query string, filters map[string]string) string {
	data := query
	if filters != nil {
		filterJSON, _ := json.Marshal(filters)
		data += string(filterJSON)
	}
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:16]) // Use first 16 bytes for shorter key
}

// GetCachedSearch retrieves a cached search result if it hasn't expired
func GetCachedSearch(cacheKey string) (*SearchCacheEntry, error) {
	entry := &SearchCacheEntry{}
	err := database.QueryRow(`
		SELECT id, cache_key, query, filters, results_json, result_count, created_at, expires_at
		FROM search_cache
		WHERE cache_key = ? AND expires_at > CURRENT_TIMESTAMP`, cacheKey).Scan(
		&entry.ID, &entry.CacheKey, &entry.Query, &entry.Filters,
		&entry.ResultsJSON, &entry.ResultCount, &entry.CreatedAt, &entry.ExpiresAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// SaveCachedSearch saves a search result to cache
func SaveCachedSearch(cacheKey, query, filters string, resultsJSON string, resultCount int, ttl time.Duration) error {
	expiresAt := time.Now().Add(ttl)
	_, err := database.Exec(`
		INSERT OR REPLACE INTO search_cache (cache_key, query, filters, results_json, result_count, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		cacheKey, query, filters, resultsJSON, resultCount, expiresAt)
	return err
}

// CleanExpiredCache removes expired cache entries
func CleanExpiredCache() error {
	_, err := database.Exec(`DELETE FROM search_cache WHERE expires_at < CURRENT_TIMESTAMP`)
	return err
}

// ClearSearchCache clears all cached search results
func ClearSearchCache() error {
	_, err := database.Exec(`DELETE FROM search_cache`)
	return err
}

// GetCacheStats returns cache statistics
func GetCacheStats() (total int, expired int, err error) {
	err = database.QueryRow(`SELECT COUNT(*) FROM search_cache`).Scan(&total)
	if err != nil {
		return 0, 0, err
	}

	err = database.QueryRow(`SELECT COUNT(*) FROM search_cache WHERE expires_at < CURRENT_TIMESTAMP`).Scan(&expired)
	if err != nil {
		return total, 0, err
	}

	return total, expired, nil
}
