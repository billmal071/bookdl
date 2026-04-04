package liber3

import "github.com/billmal071/bookdl/internal/config"

// NewClient creates a new Liber3 client
// For now, always uses the scraper as there's no known API
func NewClient() Client {
	cfg := config.Get()
	return NewScraperClient(cfg.Liber3.BaseURL)
}

// GetBaseURL returns the configured base URL for Liber3
func GetBaseURL() string {
	cfg := config.Get()
	if cfg.Liber3.BaseURL != "" {
		return cfg.Liber3.BaseURL
	}
	return "liber3.eth.limo"
}
