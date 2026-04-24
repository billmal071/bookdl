# audbookdl Source Clients Implementation Plan (Plan 2 of 8)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the four source clients (LibriVox, Internet Archive, Loyal Books, Open Library) and the multi-source search orchestrator that fans out queries in parallel.

**Architecture:** Each source implements `source.Source` interface from Plan 1. The search orchestrator uses `errgroup` to query all sources concurrently, merges results, and handles partial failures gracefully.

**Tech Stack:** Go 1.22+, net/http, encoding/json, encoding/xml, golang.org/x/sync/errgroup, github.com/PuerkitoBio/goquery (HTML scraping for Loyal Books)

**API Research Summary:**

- **LibriVox:** REST API at `librivox.org/api/feed/audiobooks`. Params: `title`, `author`, `limit`, `offset`, `format=json`, `extended=1`. Returns `books[]` with `id`, `title`, `authors[].first_name/last_name`, `totaltime`, `totaltimesecs`, `url_librivox`, `url_iarchive`, `language`, `description`, `sections[]` (chapters with `section_number`, `title`, `listen_url`, `playtime`).
- **Internet Archive:** Search at `archive.org/advancedsearch.php?q=...&output=json`. Metadata at `archive.org/metadata/{id}`. Files at `archive.org/download/{id}/{filename}`. Returns `response.docs[]` with `identifier`, `title`, `creator`, `description`, `date`, `downloads`. Metadata has `files[]` with `name`, `format`, `size`, `length`, `title`.
- **Loyal Books:** No API. Web scraping at `loyalbooks.com/book/{slug}`. RSS feeds at `loyalbooks.com/book/{slug}/feed`. Chapter MP3s at `www.archive.org/download/{id}/{file}.mp3`. HTML has chapter table with links.
- **Open Library:** Search at `openlibrary.org/search.json?q=...&fields=...`. Returns `docs[]` with `key`, `title`, `author_name[]`, `first_publish_year`, `ia[]` (Internet Archive identifiers). Bridges to IA for actual downloads.

---

### Task 1: HTTP Client Helper

**Files:**
- Create: `internal/httpclient/client.go`
- Test: `internal/httpclient/client_test.go`

- [ ] **Step 1: Write the test**

Create file `internal/httpclient/client_test.go`:

```go
package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient_Defaults(t *testing.T) {
	c := New()
	if c.httpClient.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", c.httpClient.Timeout)
	}
}

func TestNewClient_WithTimeout(t *testing.T) {
	c := New(WithTimeout(10 * time.Second))
	if c.httpClient.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", c.httpClient.Timeout)
	}
}

func TestGetJSON(t *testing.T) {
	type resp struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent header not set")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"test","age":42}`))
	}))
	defer server.Close()

	c := New()
	var got resp
	if err := c.GetJSON(context.Background(), server.URL, &got); err != nil {
		t.Fatalf("GetJSON() error: %v", err)
	}
	if got.Name != "test" || got.Age != 42 {
		t.Errorf("GetJSON() = %+v, want {test 42}", got)
	}
}

func TestGetJSON_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	c := New()
	var got struct{}
	if err := c.GetJSON(context.Background(), server.URL, &got); err == nil {
		t.Error("GetJSON() expected error for 404, got nil")
	}
}

func TestGetBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	}))
	defer server.Close()

	c := New()
	body, err := c.GetBody(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("GetBody() error: %v", err)
	}
	if string(body) != "hello world" {
		t.Errorf("GetBody() = %q, want %q", string(body), "hello world")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd ~/Documents/personal/audbookdl
CGO_ENABLED=0 /usr/local/go/bin/go test -v ./internal/httpclient/...
```

- [ ] **Step 3: Create `internal/httpclient/client.go`**

```go
package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps http.Client with common functionality.
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// Option configures the Client.
type Option func(*Client)

// WithTimeout sets the HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithUserAgent sets the User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.userAgent = ua
	}
}

// New creates a Client with sensible defaults.
func New(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// GetJSON fetches a URL and decodes the JSON response into dst.
func (c *Client) GetJSON(ctx context.Context, url string, dst interface{}) error {
	body, err := c.GetBody(ctx, url)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return fmt.Errorf("decode JSON from %s: %w", url, err)
	}
	return nil
}

// GetBody fetches a URL and returns the raw response body.
func (c *Client) GetBody(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body from %s: %w", url, err)
	}
	return body, nil
}
```

- [ ] **Step 4: Run tests, verify pass**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go test -v ./internal/httpclient/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/httpclient/
git commit -m "feat: add shared HTTP client helper with JSON and body fetching"
```

---

### Task 2: LibriVox Source Client

**Files:**
- Create: `internal/librivox/client.go`
- Create: `internal/librivox/parser.go`
- Test: `internal/librivox/client_test.go`

- [ ] **Step 1: Write the test**

Create file `internal/librivox/client_test.go`:

```go
package librivox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

const searchResponse = `{
	"books": [
		{
			"id": "1234",
			"title": "The Adventures of Sherlock Holmes",
			"description": "A collection of stories",
			"url_librivox": "https://librivox.org/the-adventures-of-sherlock-holmes",
			"language": "English",
			"copyright_year": "1892",
			"totaltime": "11:32:00",
			"totaltimesecs": 41520,
			"num_sections": "12",
			"authors": [
				{
					"id": "42",
					"first_name": "Arthur Conan",
					"last_name": "Doyle"
				}
			],
			"sections": [
				{
					"id": "1",
					"section_number": "1",
					"title": "A Scandal in Bohemia",
					"listen_url": "https://www.archive.org/download/adventures_holmes/adventuresofsherlockholmes_01_doyle_64kb.mp3",
					"language": "English",
					"playtime": "00:32:15",
					"readers": [
						{
							"display_name": "Mark Nelson"
						}
					]
				},
				{
					"id": "2",
					"section_number": "2",
					"title": "The Red-Headed League",
					"listen_url": "https://www.archive.org/download/adventures_holmes/adventuresofsherlockholmes_02_doyle_64kb.mp3",
					"language": "English",
					"playtime": "00:28:40",
					"readers": [
						{
							"display_name": "Mark Nelson"
						}
					]
				}
			]
		}
	]
}`

func TestSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("title") != "sherlock" {
			t.Errorf("title param = %q, want %q", q.Get("title"), "sherlock")
		}
		if q.Get("format") != "json" {
			t.Errorf("format param = %q, want %q", q.Get("format"), "json")
		}
		if q.Get("extended") != "1" {
			t.Errorf("extended param = %q, want %q", q.Get("extended"), "1")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchResponse))
	}))
	defer server.Close()

	client := NewClient(server.URL, httpclient.New())
	books, err := client.Search(context.Background(), "sherlock", source.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("Search() returned %d books, want 1", len(books))
	}

	book := books[0]
	if book.Title != "The Adventures of Sherlock Holmes" {
		t.Errorf("Title = %q, want %q", book.Title, "The Adventures of Sherlock Holmes")
	}
	if book.Author != "Arthur Conan Doyle" {
		t.Errorf("Author = %q, want %q", book.Author, "Arthur Conan Doyle")
	}
	if book.Source != "librivox" {
		t.Errorf("Source = %q, want %q", book.Source, "librivox")
	}
	if book.ChapterCount != 12 {
		t.Errorf("ChapterCount = %d, want 12", book.ChapterCount)
	}
}

func TestGetChapters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchResponse))
	}))
	defer server.Close()

	client := NewClient(server.URL, httpclient.New())
	chapters, err := client.GetChapters(context.Background(), "1234")
	if err != nil {
		t.Fatalf("GetChapters() error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("GetChapters() returned %d chapters, want 2", len(chapters))
	}

	ch := chapters[0]
	if ch.Title != "A Scandal in Bohemia" {
		t.Errorf("Title = %q, want %q", ch.Title, "A Scandal in Bohemia")
	}
	if ch.Index != 1 {
		t.Errorf("Index = %d, want 1", ch.Index)
	}
	if ch.DownloadURL == "" {
		t.Error("DownloadURL is empty")
	}
	if ch.Format != "mp3" {
		t.Errorf("Format = %q, want %q", ch.Format, "mp3")
	}
}

func TestName(t *testing.T) {
	client := NewClient("", httpclient.New())
	if client.Name() != "librivox" {
		t.Errorf("Name() = %q, want %q", client.Name(), "librivox")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Create `internal/librivox/parser.go` with API response types**

```go
package librivox

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/billmal071/audbookdl/internal/source"
)

// API response types
type apiResponse struct {
	Books []apiBook `json:"books"`
}

type apiBook struct {
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	URLLibrivox   string      `json:"url_librivox"`
	Language      string      `json:"language"`
	CopyrightYear string      `json:"copyright_year"`
	TotalTime     string      `json:"totaltime"`
	TotalTimeSecs int         `json:"totaltimesecs"`
	NumSections   string      `json:"num_sections"`
	Authors       []apiAuthor `json:"authors"`
	Sections      []apiSection `json:"sections"`
}

type apiAuthor struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type apiSection struct {
	ID            string      `json:"id"`
	SectionNumber string      `json:"section_number"`
	Title         string      `json:"title"`
	ListenURL     string      `json:"listen_url"`
	Language      string      `json:"language"`
	PlayTime      string      `json:"playtime"`
	Readers       []apiReader `json:"readers"`
}

type apiReader struct {
	DisplayName string `json:"display_name"`
}

func (b *apiBook) toAudiobook() *source.Audiobook {
	author := ""
	if len(b.Authors) > 0 {
		a := b.Authors[0]
		author = strings.TrimSpace(a.FirstName + " " + a.LastName)
	}

	narrator := ""
	if len(b.Sections) > 0 && len(b.Sections[0].Readers) > 0 {
		narrator = b.Sections[0].Readers[0].DisplayName
	}

	numSections, _ := strconv.Atoi(b.NumSections)

	return &source.Audiobook{
		ID:           b.ID,
		Title:        b.Title,
		Author:       author,
		Narrator:     narrator,
		Description:  b.Description,
		Language:     b.Language,
		Year:         b.CopyrightYear,
		Duration:     time.Duration(b.TotalTimeSecs) * time.Second,
		PageURL:      b.URLLibrivox,
		Format:       "mp3",
		ChapterCount: numSections,
		Source:       "librivox",
	}
}

func (s *apiSection) toChapter() *source.Chapter {
	idx, _ := strconv.Atoi(s.SectionNumber)
	return &source.Chapter{
		Index:       idx,
		Title:       s.Title,
		Duration:    parsePlaytime(s.PlayTime),
		DownloadURL: s.ListenURL,
		Format:      "mp3",
	}
}

// parsePlaytime parses "HH:MM:SS" into time.Duration.
func parsePlaytime(s string) time.Duration {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	sec, _ := strconv.Atoi(parts[2])
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
}

// buildSearchURL constructs the LibriVox API search URL.
func buildSearchURL(baseURL, query string, opts source.SearchOptions) string {
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}
	url := fmt.Sprintf("%s/api/feed/audiobooks/?title=%s&format=json&extended=1&limit=%d",
		baseURL, query, limit)
	if opts.Author != "" {
		url += "&author=" + opts.Author
	}
	if opts.Page > 0 {
		url += fmt.Sprintf("&offset=%d", opts.Page*limit)
	}
	return url
}

// buildGetURL constructs the LibriVox API URL for a specific audiobook.
func buildGetURL(baseURL, bookID string) string {
	return fmt.Sprintf("%s/api/feed/audiobooks/?id=%s&format=json&extended=1", baseURL, bookID)
}
```

- [ ] **Step 4: Create `internal/librivox/client.go`**

```go
package librivox

import (
	"context"
	"fmt"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

// Client implements source.Source for LibriVox.
type Client struct {
	baseURL string
	http    *httpclient.Client
}

// NewClient creates a LibriVox client.
func NewClient(baseURL string, http *httpclient.Client) *Client {
	if baseURL == "" {
		baseURL = "https://librivox.org"
	}
	return &Client{baseURL: baseURL, http: http}
}

func (c *Client) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	url := buildSearchURL(c.baseURL, query, opts)

	var resp apiResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("librivox search: %w", err)
	}

	books := make([]*source.Audiobook, 0, len(resp.Books))
	for _, b := range resp.Books {
		books = append(books, b.toAudiobook())
	}
	return books, nil
}

func (c *Client) GetChapters(ctx context.Context, bookID string) ([]*source.Chapter, error) {
	url := buildGetURL(c.baseURL, bookID)

	var resp apiResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("librivox get chapters: %w", err)
	}

	if len(resp.Books) == 0 {
		return nil, fmt.Errorf("librivox: book %s not found", bookID)
	}

	book := resp.Books[0]
	chapters := make([]*source.Chapter, 0, len(book.Sections))
	for _, s := range book.Sections {
		chapters = append(chapters, s.toChapter())
	}
	return chapters, nil
}

func (c *Client) Name() string {
	return "librivox"
}
```

- [ ] **Step 5: Run tests, verify pass**

- [ ] **Step 6: Commit**

```bash
git add internal/librivox/
git commit -m "feat: add LibriVox source client with API search and chapter fetching"
```

---

### Task 3: Internet Archive Source Client

**Files:**
- Create: `internal/archive/client.go`
- Create: `internal/archive/parser.go`
- Test: `internal/archive/client_test.go`

- [ ] **Step 1: Write the test**

Create file `internal/archive/client_test.go`:

```go
package archive

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

const searchResp = `{
	"response": {
		"numFound": 1,
		"start": 0,
		"docs": [
			{
				"identifier": "adventures_sherlock_holmes_0711_librivox",
				"title": "The Adventures of Sherlock Holmes",
				"creator": "Arthur Conan Doyle",
				"description": "Twelve stories of mystery",
				"date": "2006-08-01T00:00:00Z",
				"downloads": 50000
			}
		]
	}
}`

const metadataResp = `{
	"metadata": {
		"identifier": "adventures_sherlock_holmes_0711_librivox",
		"title": "The Adventures of Sherlock Holmes",
		"creator": "Arthur Conan Doyle",
		"description": "Twelve stories",
		"date": "2006-08-01T00:00:00Z",
		"runtime": "11:32:00"
	},
	"files": [
		{
			"name": "adventuresofsherlockholmes_01_doyle_64kb.mp3",
			"format": "VBR MP3",
			"size": "18874368",
			"length": "1935.5",
			"title": "01 - A Scandal in Bohemia"
		},
		{
			"name": "adventuresofsherlockholmes_02_doyle_64kb.mp3",
			"format": "VBR MP3",
			"size": "17301504",
			"length": "1720.2",
			"title": "02 - The Red-Headed League"
		},
		{
			"name": "adventuresofsherlockholmes_01_doyle.ogg",
			"format": "Ogg Vorbis",
			"size": "12345678",
			"length": "1935.5",
			"title": "01 - A Scandal in Bohemia"
		},
		{
			"name": "__ia_thumb.jpg",
			"format": "Thumbnail",
			"size": "5000"
		}
	]
}`

func TestSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchResp))
	}))
	defer server.Close()

	client := NewClient(server.URL, httpclient.New())
	books, err := client.Search(context.Background(), "sherlock", source.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("Search() returned %d books, want 1", len(books))
	}

	book := books[0]
	if book.Title != "The Adventures of Sherlock Holmes" {
		t.Errorf("Title = %q", book.Title)
	}
	if book.Author != "Arthur Conan Doyle" {
		t.Errorf("Author = %q", book.Author)
	}
	if book.Source != "archive" {
		t.Errorf("Source = %q, want %q", book.Source, "archive")
	}
	if book.ID != "adventures_sherlock_holmes_0711_librivox" {
		t.Errorf("ID = %q", book.ID)
	}
}

func TestGetChapters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(metadataResp))
	}))
	defer server.Close()

	client := NewClient(server.URL, httpclient.New())
	chapters, err := client.GetChapters(context.Background(), "adventures_sherlock_holmes_0711_librivox")
	if err != nil {
		t.Fatalf("GetChapters() error: %v", err)
	}

	// Should only return MP3 files, not OGG or thumbnails
	if len(chapters) != 2 {
		t.Fatalf("GetChapters() returned %d chapters, want 2", len(chapters))
	}

	ch := chapters[0]
	if ch.Title != "01 - A Scandal in Bohemia" {
		t.Errorf("Title = %q", ch.Title)
	}
	if ch.Index != 1 {
		t.Errorf("Index = %d, want 1", ch.Index)
	}
	if ch.Format != "mp3" {
		t.Errorf("Format = %q, want mp3", ch.Format)
	}
	if ch.FileSize != 18874368 {
		t.Errorf("FileSize = %d, want 18874368", ch.FileSize)
	}
}

func TestGetChapters_FiltersMp3Only(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(metadataResp))
	}))
	defer server.Close()

	client := NewClient(server.URL, httpclient.New())
	chapters, _ := client.GetChapters(context.Background(), "test-id")

	for _, ch := range chapters {
		if ch.Format != "mp3" {
			t.Errorf("chapter %d has format %q, expected only mp3", ch.Index, ch.Format)
		}
	}
}

func TestName(t *testing.T) {
	client := NewClient("", httpclient.New())
	if client.Name() != "archive" {
		t.Errorf("Name() = %q, want %q", client.Name(), "archive")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Create `internal/archive/parser.go`**

```go
package archive

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/billmal071/audbookdl/internal/source"
)

// API response types for search
type searchResponse struct {
	Response struct {
		NumFound int         `json:"numFound"`
		Start    int         `json:"start"`
		Docs     []searchDoc `json:"docs"`
	} `json:"response"`
}

type searchDoc struct {
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Creator     string `json:"creator"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Downloads   int    `json:"downloads"`
}

// API response types for metadata
type metadataResponse struct {
	Metadata metadataInfo `json:"metadata"`
	Files    []fileInfo   `json:"files"`
}

type metadataInfo struct {
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Creator     string `json:"creator"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Runtime     string `json:"runtime"`
}

type fileInfo struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Size   string `json:"size"`
	Length string `json:"length"`
	Title  string `json:"title"`
}

func (d *searchDoc) toAudiobook() *source.Audiobook {
	year := ""
	if len(d.Date) >= 4 {
		year = d.Date[:4]
	}
	return &source.Audiobook{
		ID:          d.Identifier,
		Title:       d.Title,
		Author:      d.Creator,
		Description: d.Description,
		Year:        year,
		PageURL:     fmt.Sprintf("https://archive.org/details/%s", d.Identifier),
		Format:      "mp3",
		Source:      "archive",
	}
}

func (f *fileInfo) isAudioMP3() bool {
	return strings.Contains(strings.ToLower(f.Format), "mp3") &&
		strings.HasSuffix(strings.ToLower(f.Name), ".mp3")
}

func (f *fileInfo) toChapter(identifier string, index int) *source.Chapter {
	size, _ := strconv.ParseInt(f.Size, 10, 64)
	lengthSec, _ := strconv.ParseFloat(f.Length, 64)

	title := f.Title
	if title == "" {
		// Strip extension for title
		title = strings.TrimSuffix(f.Name, ".mp3")
	}

	return &source.Chapter{
		Index:       index,
		Title:       title,
		Duration:    time.Duration(math.Round(lengthSec)) * time.Second,
		DownloadURL: fmt.Sprintf("https://archive.org/download/%s/%s", identifier, f.Name),
		Format:      "mp3",
		FileSize:    size,
	}
}

func buildSearchURL(baseURL, query string, opts source.SearchOptions) string {
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}
	q := fmt.Sprintf("collection:(librivoxaudio OR audio_bookspoetry) AND (%s)", query)
	url := fmt.Sprintf("%s/advancedsearch.php?q=%s&output=json&rows=%d&fl[]=identifier,title,creator,description,date,downloads",
		baseURL, q, limit)
	if opts.Page > 0 {
		url += fmt.Sprintf("&page=%d", opts.Page+1)
	}
	return url
}

func buildMetadataURL(baseURL, identifier string) string {
	return fmt.Sprintf("%s/metadata/%s", baseURL, identifier)
}
```

- [ ] **Step 4: Create `internal/archive/client.go`**

```go
package archive

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

// Client implements source.Source for Internet Archive.
type Client struct {
	baseURL string
	http    *httpclient.Client
}

// NewClient creates an Internet Archive client.
func NewClient(baseURL string, http *httpclient.Client) *Client {
	if baseURL == "" {
		baseURL = "https://archive.org"
	}
	return &Client{baseURL: baseURL, http: http}
}

func (c *Client) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	url := buildSearchURL(c.baseURL, query, opts)

	var resp searchResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("archive search: %w", err)
	}

	books := make([]*source.Audiobook, 0, len(resp.Response.Docs))
	for _, d := range resp.Response.Docs {
		books = append(books, d.toAudiobook())
	}
	return books, nil
}

func (c *Client) GetChapters(ctx context.Context, bookID string) ([]*source.Chapter, error) {
	url := buildMetadataURL(c.baseURL, bookID)

	var resp metadataResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("archive metadata: %w", err)
	}

	// Filter to MP3 files only
	var chapters []*source.Chapter
	idx := 1
	for _, f := range resp.Files {
		if f.isAudioMP3() {
			chapters = append(chapters, f.toChapter(resp.Metadata.Identifier, idx))
			idx++
		}
	}

	// Sort by filename to ensure correct chapter order
	sort.Slice(chapters, func(i, j int) bool {
		return strings.ToLower(chapters[i].DownloadURL) < strings.ToLower(chapters[j].DownloadURL)
	})

	// Re-index after sorting
	for i := range chapters {
		chapters[i].Index = i + 1
	}

	return chapters, nil
}

func (c *Client) Name() string {
	return "archive"
}
```

- [ ] **Step 5: Run tests, verify pass**

- [ ] **Step 6: Commit**

```bash
git add internal/archive/
git commit -m "feat: add Internet Archive source client with search and metadata"
```

---

### Task 4: Loyal Books Source Client

**Files:**
- Create: `internal/loyalbooks/client.go`
- Create: `internal/loyalbooks/parser.go`
- Test: `internal/loyalbooks/client_test.go`

- [ ] **Step 1: Write the test**

Create file `internal/loyalbooks/client_test.go`:

```go
package loyalbooks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

const searchPageHTML = `<html><body>
<table class="layout2-blue">
<tr>
	<td class="layout2">
		<a href="/book/adventures-of-sherlock-holmes"><img src="/image/cover.jpg"/></a>
	</td>
	<td class="layout2">
		<a href="/book/adventures-of-sherlock-holmes">The Adventures of Sherlock Holmes</a>
		<br/>By: <a href="/author/Arthur-Conan-Doyle">Arthur Conan Doyle</a>
	</td>
</tr>
<tr>
	<td class="layout2">
		<a href="/book/study-in-scarlet"><img src="/image/cover2.jpg"/></a>
	</td>
	<td class="layout2">
		<a href="/book/study-in-scarlet">A Study in Scarlet</a>
		<br/>By: <a href="/author/Arthur-Conan-Doyle">Arthur Conan Doyle</a>
	</td>
</tr>
</table>
</body></html>`

const bookPageHTML = `<html><body>
<div class="book">
	<h1>The Adventures of Sherlock Holmes</h1>
	<span class="author">By: <a href="/author/Arthur-Conan-Doyle">Arthur Conan Doyle</a></span>
</div>
<table class="chapter-download">
<tr>
	<td>1</td>
	<td><a href="https://www.archive.org/download/adventures_holmes/chapter01.mp3">A Scandal in Bohemia</a></td>
	<td>32:15</td>
</tr>
<tr>
	<td>2</td>
	<td><a href="https://www.archive.org/download/adventures_holmes/chapter02.mp3">The Red-Headed League</a></td>
	<td>28:40</td>
</tr>
</table>
</body></html>`

func TestSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(searchPageHTML))
	}))
	defer server.Close()

	client := NewClient(server.URL, httpclient.New())
	books, err := client.Search(context.Background(), "sherlock", source.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(books) < 1 {
		t.Fatal("Search() returned no books")
	}

	book := books[0]
	if book.Title != "The Adventures of Sherlock Holmes" {
		t.Errorf("Title = %q", book.Title)
	}
	if book.Author != "Arthur Conan Doyle" {
		t.Errorf("Author = %q", book.Author)
	}
	if book.Source != "loyalbooks" {
		t.Errorf("Source = %q, want %q", book.Source, "loyalbooks")
	}
}

func TestGetChapters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(bookPageHTML))
	}))
	defer server.Close()

	client := NewClient(server.URL, httpclient.New())
	chapters, err := client.GetChapters(context.Background(), "adventures-of-sherlock-holmes")
	if err != nil {
		t.Fatalf("GetChapters() error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("GetChapters() returned %d chapters, want 2", len(chapters))
	}

	ch := chapters[0]
	if ch.Title != "A Scandal in Bohemia" {
		t.Errorf("Title = %q", ch.Title)
	}
	if ch.Index != 1 {
		t.Errorf("Index = %d, want 1", ch.Index)
	}
	if ch.DownloadURL == "" {
		t.Error("DownloadURL is empty")
	}
}

func TestName(t *testing.T) {
	client := NewClient("", httpclient.New())
	if client.Name() != "loyalbooks" {
		t.Errorf("Name() = %q, want %q", client.Name(), "loyalbooks")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Create `internal/loyalbooks/parser.go`**

```go
package loyalbooks

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/billmal071/audbookdl/internal/source"
)

type bookListing struct {
	Slug   string
	Title  string
	Author string
}

func parseSearchPage(doc *goquery.Document) []bookListing {
	var books []bookListing

	doc.Find("table.layout2-blue tr").Each(func(i int, s *goquery.Selection) {
		titleLink := s.Find("td.layout2 a[href^='/book/']").First()
		href, exists := titleLink.Attr("href")
		if !exists || href == "" {
			return
		}

		title := strings.TrimSpace(titleLink.Text())
		if title == "" {
			return
		}

		author := ""
		s.Find("td.layout2 a[href^='/author/']").Each(func(_ int, a *goquery.Selection) {
			author = strings.TrimSpace(a.Text())
		})

		slug := strings.TrimPrefix(href, "/book/")
		books = append(books, bookListing{
			Slug:   slug,
			Title:  title,
			Author: author,
		})
	})

	return books
}

func parseBookPage(doc *goquery.Document, slug, baseURL string) []*source.Chapter {
	var chapters []*source.Chapter

	doc.Find("table.chapter-download tr").Each(func(i int, s *goquery.Selection) {
		cells := s.Find("td")
		if cells.Length() < 2 {
			return
		}

		link := cells.Eq(1).Find("a")
		href, exists := link.Attr("href")
		if !exists {
			return
		}

		title := strings.TrimSpace(link.Text())
		idx := i + 1

		// Try to parse index from first cell
		idxText := strings.TrimSpace(cells.Eq(0).Text())
		if n, err := strconv.Atoi(idxText); err == nil {
			idx = n
		}

		// Parse duration from third cell if present
		var duration time.Duration
		if cells.Length() >= 3 {
			duration = parseDuration(strings.TrimSpace(cells.Eq(2).Text()))
		}

		chapters = append(chapters, &source.Chapter{
			Index:       idx,
			Title:       title,
			Duration:    duration,
			DownloadURL: href,
			Format:      "mp3",
		})
	})

	return chapters
}

func parseDuration(s string) time.Duration {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 2:
		m, _ := strconv.Atoi(parts[0])
		sec, _ := strconv.Atoi(parts[1])
		return time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
	case 3:
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		sec, _ := strconv.Atoi(parts[2])
		return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
	default:
		return 0
	}
}

func buildSearchURL(baseURL, query string) string {
	return fmt.Sprintf("%s/search?q=%s", baseURL, query)
}

func buildBookURL(baseURL, slug string) string {
	return fmt.Sprintf("%s/book/%s", baseURL, slug)
}
```

- [ ] **Step 4: Create `internal/loyalbooks/client.go`**

```go
package loyalbooks

import (
	"bytes"
	"context"
	"fmt"

	"github.com/PuerkitoBio/goquery"
	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

// Client implements source.Source for Loyal Books.
type Client struct {
	baseURL string
	http    *httpclient.Client
}

// NewClient creates a Loyal Books client.
func NewClient(baseURL string, http *httpclient.Client) *Client {
	if baseURL == "" {
		baseURL = "https://www.loyalbooks.com"
	}
	return &Client{baseURL: baseURL, http: http}
}

func (c *Client) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	url := buildSearchURL(c.baseURL, query)

	body, err := c.http.GetBody(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("loyalbooks search: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("loyalbooks parse: %w", err)
	}

	listings := parseSearchPage(doc)

	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}
	if len(listings) > limit {
		listings = listings[:limit]
	}

	books := make([]*source.Audiobook, 0, len(listings))
	for _, l := range listings {
		books = append(books, &source.Audiobook{
			ID:      l.Slug,
			Title:   l.Title,
			Author:  l.Author,
			PageURL: buildBookURL(c.baseURL, l.Slug),
			Format:  "mp3",
			Source:  "loyalbooks",
		})
	}
	return books, nil
}

func (c *Client) GetChapters(ctx context.Context, bookID string) ([]*source.Chapter, error) {
	url := buildBookURL(c.baseURL, bookID)

	body, err := c.http.GetBody(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("loyalbooks book page: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("loyalbooks parse: %w", err)
	}

	chapters := parseBookPage(doc, bookID, c.baseURL)
	if len(chapters) == 0 {
		return nil, fmt.Errorf("loyalbooks: no chapters found for %s", bookID)
	}

	return chapters, nil
}

func (c *Client) Name() string {
	return "loyalbooks"
}
```

- [ ] **Step 5: Fetch goquery dependency and run tests**

```bash
cd ~/Documents/personal/audbookdl
/usr/local/go/bin/go get github.com/PuerkitoBio/goquery
CGO_ENABLED=0 /usr/local/go/bin/go mod tidy
CGO_ENABLED=0 /usr/local/go/bin/go test -v ./internal/loyalbooks/...
```

- [ ] **Step 6: Commit**

```bash
git add internal/loyalbooks/ go.mod go.sum
git commit -m "feat: add Loyal Books source client with HTML scraping"
```

---

### Task 5: Open Library Source Client

**Files:**
- Create: `internal/openlibrary/client.go`
- Create: `internal/openlibrary/parser.go`
- Test: `internal/openlibrary/client_test.go`

- [ ] **Step 1: Write the test**

Create file `internal/openlibrary/client_test.go`:

```go
package openlibrary

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

const searchResp = `{
	"numFound": 2,
	"start": 0,
	"docs": [
		{
			"key": "/works/OL262758W",
			"title": "The Adventures of Sherlock Holmes",
			"author_name": ["Arthur Conan Doyle"],
			"first_publish_year": 1892,
			"ia": ["adventures_sherlock_holmes_0711_librivox", "adventuresofsherlo0000doyl"]
		},
		{
			"key": "/works/OL262421W",
			"title": "A Study in Scarlet",
			"author_name": ["Arthur Conan Doyle"],
			"first_publish_year": 1887,
			"ia": ["study_in_scarlet_librivox"]
		}
	]
}`

// We also need a metadata endpoint for when GetChapters delegates to IA
const iaMetadataResp = `{
	"metadata": {
		"identifier": "adventures_sherlock_holmes_0711_librivox",
		"title": "The Adventures of Sherlock Holmes",
		"creator": "Arthur Conan Doyle"
	},
	"files": [
		{
			"name": "chapter01.mp3",
			"format": "VBR MP3",
			"size": "18874368",
			"length": "1935.5",
			"title": "01 - A Scandal in Bohemia"
		}
	]
}`

func TestSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(searchResp))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, httpclient.New())
	books, err := client.Search(context.Background(), "sherlock", source.SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("Search() returned %d books, want 2", len(books))
	}

	book := books[0]
	if book.Title != "The Adventures of Sherlock Holmes" {
		t.Errorf("Title = %q", book.Title)
	}
	if book.Author != "Arthur Conan Doyle" {
		t.Errorf("Author = %q", book.Author)
	}
	if book.Year != "1892" {
		t.Errorf("Year = %q, want 1892", book.Year)
	}
	if book.Source != "openlibrary" {
		t.Errorf("Source = %q", book.Source)
	}
	// ID should be the first IA identifier
	if book.ID != "adventures_sherlock_holmes_0711_librivox" {
		t.Errorf("ID = %q", book.ID)
	}
}

func TestGetChapters_DelegatesToIA(t *testing.T) {
	iaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(iaMetadataResp))
	}))
	defer iaServer.Close()

	client := NewClient("", iaServer.URL, httpclient.New())
	chapters, err := client.GetChapters(context.Background(), "adventures_sherlock_holmes_0711_librivox")
	if err != nil {
		t.Fatalf("GetChapters() error: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("GetChapters() returned %d, want 1", len(chapters))
	}
	if chapters[0].Format != "mp3" {
		t.Errorf("Format = %q, want mp3", chapters[0].Format)
	}
}

func TestName(t *testing.T) {
	client := NewClient("", "", httpclient.New())
	if client.Name() != "openlibrary" {
		t.Errorf("Name() = %q", client.Name())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Create `internal/openlibrary/parser.go`**

```go
package openlibrary

import (
	"fmt"
	"strconv"

	"github.com/billmal071/audbookdl/internal/source"
)

type searchResponse struct {
	NumFound int         `json:"numFound"`
	Start    int         `json:"start"`
	Docs     []searchDoc `json:"docs"`
}

type searchDoc struct {
	Key              string   `json:"key"`
	Title            string   `json:"title"`
	AuthorName       []string `json:"author_name"`
	FirstPublishYear int      `json:"first_publish_year"`
	IA               []string `json:"ia"`
}

func (d *searchDoc) toAudiobook() *source.Audiobook {
	author := ""
	if len(d.AuthorName) > 0 {
		author = d.AuthorName[0]
	}

	year := ""
	if d.FirstPublishYear > 0 {
		year = strconv.Itoa(d.FirstPublishYear)
	}

	// Use the first IA identifier as the ID (for GetChapters delegation)
	id := d.Key
	if len(d.IA) > 0 {
		id = d.IA[0]
	}

	pageURL := fmt.Sprintf("https://openlibrary.org%s", d.Key)

	return &source.Audiobook{
		ID:      id,
		Title:   d.Title,
		Author:  author,
		Year:    year,
		PageURL: pageURL,
		Format:  "mp3",
		Source:  "openlibrary",
	}
}

func buildSearchURL(baseURL, query string, opts source.SearchOptions) string {
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}
	url := fmt.Sprintf("%s/search.json?q=%s&fields=key,title,author_name,first_publish_year,ia&limit=%d",
		baseURL, query, limit)
	if opts.Page > 0 {
		url += fmt.Sprintf("&offset=%d", opts.Page*limit)
	}
	return url
}
```

- [ ] **Step 4: Create `internal/openlibrary/client.go`**

```go
package openlibrary

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
)

// Client implements source.Source for Open Library.
// It delegates to Internet Archive for actual chapter downloads.
type Client struct {
	baseURL   string
	iaBaseURL string
	http      *httpclient.Client
}

// NewClient creates an Open Library client.
// iaBaseURL is the Internet Archive base URL for fetching chapter metadata.
func NewClient(baseURL, iaBaseURL string, http *httpclient.Client) *Client {
	if baseURL == "" {
		baseURL = "https://openlibrary.org"
	}
	if iaBaseURL == "" {
		iaBaseURL = "https://archive.org"
	}
	return &Client{baseURL: baseURL, iaBaseURL: iaBaseURL, http: http}
}

func (c *Client) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	url := buildSearchURL(c.baseURL, query, opts)

	var resp searchResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("openlibrary search: %w", err)
	}

	books := make([]*source.Audiobook, 0, len(resp.Docs))
	for _, d := range resp.Docs {
		// Only include results that have Internet Archive identifiers
		if len(d.IA) > 0 {
			books = append(books, d.toAudiobook())
		}
	}
	return books, nil
}

// GetChapters delegates to Internet Archive metadata API using the IA identifier.
func (c *Client) GetChapters(ctx context.Context, bookID string) ([]*source.Chapter, error) {
	url := fmt.Sprintf("%s/metadata/%s", c.iaBaseURL, bookID)

	var resp iaMetadataResponse
	if err := c.http.GetJSON(ctx, url, &resp); err != nil {
		return nil, fmt.Errorf("openlibrary get chapters (via IA): %w", err)
	}

	var chapters []*source.Chapter
	idx := 1
	for _, f := range resp.Files {
		if f.isAudioMP3() {
			chapters = append(chapters, f.toChapter(bookID, idx))
			idx++
		}
	}

	sort.Slice(chapters, func(i, j int) bool {
		return strings.ToLower(chapters[i].DownloadURL) < strings.ToLower(chapters[j].DownloadURL)
	})
	for i := range chapters {
		chapters[i].Index = i + 1
	}

	return chapters, nil
}

func (c *Client) Name() string {
	return "openlibrary"
}

// IA metadata types (reused from archive package pattern, but local to avoid circular deps)
type iaMetadataResponse struct {
	Metadata struct {
		Identifier string `json:"identifier"`
	} `json:"metadata"`
	Files []iaFileInfo `json:"files"`
}

type iaFileInfo struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Size   string `json:"size"`
	Length string `json:"length"`
	Title  string `json:"title"`
}

func (f *iaFileInfo) isAudioMP3() bool {
	return strings.Contains(strings.ToLower(f.Format), "mp3") &&
		strings.HasSuffix(strings.ToLower(f.Name), ".mp3")
}

func (f *iaFileInfo) toChapter(identifier string, index int) *source.Chapter {
	size, _ := strconv.ParseInt(f.Size, 10, 64)
	lengthSec, _ := strconv.ParseFloat(f.Length, 64)

	title := f.Title
	if title == "" {
		title = strings.TrimSuffix(f.Name, ".mp3")
	}

	return &source.Chapter{
		Index:       index,
		Title:       title,
		Duration:    time.Duration(math.Round(lengthSec)) * time.Second,
		DownloadURL: fmt.Sprintf("https://archive.org/download/%s/%s", identifier, f.Name),
		Format:      "mp3",
		FileSize:    size,
	}
}
```

- [ ] **Step 5: Run tests, verify pass**

- [ ] **Step 6: Commit**

```bash
git add internal/openlibrary/
git commit -m "feat: add Open Library source client with IA delegation for chapters"
```

---

### Task 6: Multi-Source Search Orchestrator

**Files:**
- Create: `internal/search/searcher.go`
- Test: `internal/search/searcher_test.go`

- [ ] **Step 1: Write the test**

Create file `internal/search/searcher_test.go`:

```go
package search

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/billmal071/audbookdl/internal/source"
)

// mockSource implements source.Source for testing.
type mockSource struct {
	name     string
	books    []*source.Audiobook
	chapters []*source.Chapter
	err      error
	delay    time.Duration
}

func (m *mockSource) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.books, m.err
}

func (m *mockSource) GetChapters(ctx context.Context, bookID string) ([]*source.Chapter, error) {
	return m.chapters, m.err
}

func (m *mockSource) Name() string { return m.name }

func TestSearcher_MultipleSources(t *testing.T) {
	s := New(
		&mockSource{
			name:  "source1",
			books: []*source.Audiobook{{ID: "1", Title: "Book A", Source: "source1"}},
		},
		&mockSource{
			name:  "source2",
			books: []*source.Audiobook{{ID: "2", Title: "Book B", Source: "source2"}},
		},
	)

	books, err := s.Search(context.Background(), "test", source.SearchOptions{})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(books) != 2 {
		t.Errorf("Search() returned %d books, want 2", len(books))
	}
}

func TestSearcher_PartialFailure(t *testing.T) {
	s := New(
		&mockSource{
			name:  "good",
			books: []*source.Audiobook{{ID: "1", Title: "Book A", Source: "good"}},
		},
		&mockSource{
			name: "bad",
			err:  errors.New("connection refused"),
		},
	)

	books, err := s.Search(context.Background(), "test", source.SearchOptions{})
	if err != nil {
		t.Fatalf("Search() error: %v (should succeed with partial results)", err)
	}
	if len(books) != 1 {
		t.Errorf("Search() returned %d books, want 1", len(books))
	}
}

func TestSearcher_AllFail(t *testing.T) {
	s := New(
		&mockSource{name: "bad1", err: errors.New("fail1")},
		&mockSource{name: "bad2", err: errors.New("fail2")},
	)

	books, err := s.Search(context.Background(), "test", source.SearchOptions{})
	if err == nil {
		t.Error("Search() expected error when all sources fail")
	}
	if len(books) != 0 {
		t.Errorf("Search() returned %d books, want 0", len(books))
	}
}

func TestSearcher_Empty(t *testing.T) {
	s := New()
	books, err := s.Search(context.Background(), "test", source.SearchOptions{})
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(books) != 0 {
		t.Errorf("Search() returned %d books, want 0", len(books))
	}
}

func TestSearcher_ContextCancellation(t *testing.T) {
	s := New(
		&mockSource{
			name:  "slow",
			books: []*source.Audiobook{{ID: "1", Title: "Slow Book"}},
			delay: 5 * time.Second,
		},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := s.Search(ctx, "test", source.SearchOptions{})
	if err == nil {
		t.Error("Search() expected error on context cancellation")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Create `internal/search/searcher.go`**

```go
package search

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/billmal071/audbookdl/internal/source"
)

// Searcher orchestrates searches across multiple sources concurrently.
type Searcher struct {
	sources []source.Source
}

// New creates a Searcher with the given sources.
func New(sources ...source.Source) *Searcher {
	return &Searcher{sources: sources}
}

// Search queries all sources concurrently and merges results.
// Returns partial results if some sources fail. Returns error only if ALL sources fail.
func (s *Searcher) Search(ctx context.Context, query string, opts source.SearchOptions) ([]*source.Audiobook, error) {
	if len(s.sources) == 0 {
		return nil, nil
	}

	type result struct {
		books []*source.Audiobook
		err   error
	}

	results := make([]result, len(s.sources))
	var wg sync.WaitGroup

	for i, src := range s.sources {
		wg.Add(1)
		go func(idx int, src source.Source) {
			defer wg.Done()
			books, err := src.Search(ctx, query, opts)
			results[idx] = result{books: books, err: err}
		}(i, src)
	}

	wg.Wait()

	var allBooks []*source.Audiobook
	var errs []string
	for i, r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", s.sources[i].Name(), r.err))
			continue
		}
		allBooks = append(allBooks, r.books...)
	}

	// If all sources failed, return an error
	if len(errs) == len(s.sources) {
		return nil, fmt.Errorf("all sources failed: %s", strings.Join(errs, "; "))
	}

	return allBooks, nil
}

// Sources returns the list of registered sources.
func (s *Searcher) Sources() []source.Source {
	return s.sources
}
```

- [ ] **Step 4: Fetch errgroup (not needed — we used sync.WaitGroup which is simpler and sufficient)**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go mod tidy
```

- [ ] **Step 5: Run tests, verify pass**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go test -v ./internal/search/...
```

- [ ] **Step 6: Run all tests**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go test -v ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/search/
git commit -m "feat: add multi-source search orchestrator with concurrent fan-out"
```

---

### Task 7: Verify Full Build and All Tests

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

```bash
cd ~/Documents/personal/audbookdl
CGO_ENABLED=0 /usr/local/go/bin/go test -v ./...
```

Expected: All tests pass across httpclient, librivox, archive, loyalbooks, openlibrary, search, source, config, db.

- [ ] **Step 2: Build and verify**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go build -o ./build/audbookdl ./cmd/audbookdl
./build/audbookdl version
```

- [ ] **Step 3: Format and vet**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go fmt ./...
CGO_ENABLED=0 /usr/local/go/bin/go vet ./...
```

- [ ] **Step 4: Verify new file structure**

New packages added:
```
internal/httpclient/client.go
internal/httpclient/client_test.go
internal/librivox/client.go
internal/librivox/parser.go
internal/librivox/client_test.go
internal/archive/client.go
internal/archive/parser.go
internal/archive/client_test.go
internal/loyalbooks/client.go
internal/loyalbooks/parser.go
internal/loyalbooks/client_test.go
internal/openlibrary/client.go
internal/openlibrary/parser.go
internal/openlibrary/client_test.go
internal/search/searcher.go
internal/search/searcher_test.go
```
