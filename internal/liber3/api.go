package liber3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	apiBaseURL = "https://lgate.glitternode.ru/v1"
)

// ipfsGateways is the list of IPFS gateways to try, in order.
// Matches Liber3's Library3.supportedGateways.
var ipfsGateways = []string{
	"https://gateway-ipfs.st/ipfs",
	"https://cloudflare-ipfs.com/ipfs",
	"https://gateway.pinata.cloud/ipfs",
	"https://dweb.link/ipfs",
	"https://ipfs.fleek.co/ipfs",
	"https://gateway.ipfs.io/ipfs",
}

// APIClient accesses Liber3 via its backend JSON API.
type APIClient struct {
	http *http.Client
}

// NewAPIClient creates a new API client.
func NewAPIClient() *APIClient {
	return &APIClient{
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// apiSearchRequest is the POST body for /v1/searchV2.
type apiSearchRequest struct {
	Address string `json:"address"`
	Word    string `json:"word"`
}

// apiSearchResponse is the response from /v1/searchV2.
type apiSearchResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Book []apiBookItem `json:"book"`
	} `json:"data"`
}

// apiBookItem represents a book in the search API response.
type apiBookItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Publisher string `json:"publisher"`
	Year      string `json:"year"`
	Language  string `json:"language"`
	Extension string `json:"extension"`
	FileSize  string `json:"filesize"`
	IPFSCID   string `json:"ipfs_cid"`
	CoverURL  string `json:"coverurl"`
}

// Search searches for books via the Liber3 API.
func (c *APIClient) Search(ctx context.Context, query string, limit int) ([]*Book, error) {
	return c.SearchPage(ctx, query, limit, 1)
}

// SearchPage searches for books with pagination support.
func (c *APIClient) SearchPage(ctx context.Context, query string, limit int, page int) ([]*Book, error) {
	body, err := json.Marshal(apiSearchRequest{Word: query})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiBaseURL+"/searchV2", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var searchResp apiSearchResponse
	if err := json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	if searchResp.Code != 200 {
		return nil, fmt.Errorf("search API error: %s", searchResp.Message)
	}

	seen := make(map[string]bool)
	var books []*Book
	for _, item := range searchResp.Data.Book {
		if len(books) >= limit {
			break
		}
		id := strings.ToLower(strings.TrimSpace(item.ID))
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true

		title := strings.TrimSpace(item.Title)
		if title == "" {
			continue
		}

		var downloadURL string
		if item.IPFSCID != "" {
			downloadURL = fmt.Sprintf("%s/%s", ipfsGateways[0], item.IPFSCID)
		}

		books = append(books, &Book{
			MD5Hash:     id,
			Title:       title,
			Authors:     strings.TrimSpace(item.Author),
			Publisher:   strings.TrimSpace(item.Publisher),
			Year:        strings.TrimSpace(item.Year),
			Language:    strings.TrimSpace(item.Language),
			Format:      strings.ToUpper(strings.TrimSpace(item.Extension)),
			Size:        formatFileSize(item.FileSize),
			PageURL:     fmt.Sprintf("https://%s/#/book/%s", GetBaseURL(), id),
			DownloadURL: downloadURL,
			Source:      "liber3",
		})
	}

	if len(books) == 0 {
		return nil, ErrNoResults
	}

	return books, nil
}

// GetDownloadInfo retrieves download info for a book by its ID.
func (c *APIClient) GetDownloadInfo(ctx context.Context, md5Hash string) (*DownloadInfo, error) {
	body, err := json.Marshal(map[string]any{
		"book_ids": []string{md5Hash},
		"address":  "",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiBaseURL+"/book", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("book request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// The response is {"data":{"book":{"<id>":{"book":{...}}}}}
	var bookResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Book map[string]struct {
				Book apiBookItem `json:"book"`
			} `json:"book"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &bookResp); err != nil {
		return nil, fmt.Errorf("failed to parse book response: %w", err)
	}

	if bookResp.Code != 200 {
		return nil, fmt.Errorf("book API error: %s", bookResp.Message)
	}

	// Find the book in the response (case-insensitive key match)
	var item apiBookItem
	found := false
	for key, entry := range bookResp.Data.Book {
		if strings.EqualFold(key, md5Hash) {
			item = entry.Book
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("book not found in API response")
	}

	if item.IPFSCID == "" {
		return nil, fmt.Errorf("no IPFS CID available for this book")
	}

	info := &DownloadInfo{
		DirectURL: fmt.Sprintf("%s/%s", ipfsGateways[0], item.IPFSCID),
		Filename:  fmt.Sprintf("%s.%s", sanitizeTitle(item.Title), strings.ToLower(item.Extension)),
	}

	// Add all gateways as mirrors for fallback
	for _, gw := range ipfsGateways {
		info.MirrorURLs = append(info.MirrorURLs, fmt.Sprintf("%s/%s", gw, item.IPFSCID))
	}

	return info, nil
}

// formatFileSize converts a raw byte string or human-readable size to a display string.
func formatFileSize(s string) string {
	s = strings.TrimSpace(s)
	// If it already has a unit (e.g. "1.3 MB"), return as-is
	if strings.ContainsAny(s, "kKmMgG") {
		return s
	}
	// Try to parse as raw bytes
	bytes, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return s
	}
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.2f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func sanitizeTitle(s string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range invalid {
		s = strings.ReplaceAll(s, char, "_")
	}
	s = strings.TrimSpace(s)
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}
