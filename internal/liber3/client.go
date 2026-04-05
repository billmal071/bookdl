package liber3

import "github.com/billmal071/bookdl/internal/config"

// NewClient creates a new Liber3 client.
// Uses the API client for reliable search and download via IPFS.
func NewClient() Client {
	return NewAPIClient()
}

// GetBaseURL returns the configured base URL for Liber3
func GetBaseURL() string {
	cfg := config.Get()
	if cfg.Liber3.BaseURL != "" {
		return cfg.Liber3.BaseURL
	}
	return "liber3.eth.limo"
}
