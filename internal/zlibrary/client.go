package zlibrary

import "github.com/billmal071/bookdl/internal/config"

// NewClient creates a new Z-Library client
// For now, always uses the scraper as there's no known API
func NewClient() Client {
	cfg := config.Get()
	return NewScraperClient(cfg.ZLibrary.BaseURL)
}

// GetBaseURL returns the configured base URL for Z-Library
func GetBaseURL() string {
	cfg := config.Get()
	if cfg.ZLibrary.BaseURL != "" {
		return cfg.ZLibrary.BaseURL
	}
	return "z-library.sk"
}
