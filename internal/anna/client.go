package anna

import (
	"github.com/williams/bookdl/internal/config"
)

// NewClient creates a new Anna's Archive client
// It uses the API client if an API key is configured, otherwise falls back to scraping
func NewClient() Client {
	cfg := config.Get()

	if cfg.Anna.APIKey != "" {
		return NewAPIClient(cfg.Anna.APIKey, cfg.Anna.BaseURL)
	}

	return NewScraperClient(cfg.Anna.BaseURL)
}

// GetBaseURL returns the configured base URL
func GetBaseURL() string {
	cfg := config.Get()
	if cfg.Anna.BaseURL != "" {
		return cfg.Anna.BaseURL
	}
	return "annas-archive.li"
}
