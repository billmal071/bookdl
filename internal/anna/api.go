package anna

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// APIClient uses the Anna's Archive API with an API key
type APIClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// NewAPIClient creates a new API client
func NewAPIClient(apiKey, baseURL string) *APIClient {
	if baseURL == "" {
		baseURL = "annas-archive.li"
	}
	return &APIClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search searches for books using the API
func (c *APIClient) Search(ctx context.Context, query string, limit int) ([]*Book, error) {
	return c.SearchPage(ctx, query, limit, 1)
}

// SearchPage searches for books with pagination using the API
func (c *APIClient) SearchPage(ctx context.Context, query string, limit int, page int) ([]*Book, error) {
	// The API search endpoint with pagination
	offset := (page - 1) * limit
	url := fmt.Sprintf("https://%s/dyn/api/fast_download.json?q=%s&limit=%d&offset=%d&key=%s",
		c.baseURL, query, limit, offset, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Books []*Book `json:"books"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Books, nil
}

// GetDownloadInfo retrieves download information for a book
func (c *APIClient) GetDownloadInfo(ctx context.Context, md5Hash string) (*DownloadInfo, error) {
	url := fmt.Sprintf("https://%s/dyn/api/fast_download.json?md5=%s&key=%s",
		c.baseURL, md5Hash, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		DownloadLinks []string `json:"download_links"`
		Filename      string   `json:"filename"`
		FileSize      int64    `json:"filesize"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	info := &DownloadInfo{
		Filename:   result.Filename,
		FileSize:   result.FileSize,
		MirrorURLs: result.DownloadLinks,
	}
	if len(result.DownloadLinks) > 0 {
		info.DirectURL = result.DownloadLinks[0]
	}

	return info, nil
}
