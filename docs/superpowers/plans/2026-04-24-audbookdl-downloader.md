# audbookdl Download Manager Implementation Plan (Plan 3 of 8)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the album-aware download manager that treats an entire audiobook (multiple chapter files) as a single unit, with per-chapter chunked downloads, resume support, retry with backoff, and file organization.

**Architecture:** The Manager orchestrates downloading all chapters of an audiobook. Each chapter is downloaded with chunked transfer for resumability, tracked in SQLite. The manager creates the `Author/Title/` directory structure and names files with zero-padded chapter indices.

**Tech Stack:** Go 1.22+, net/http, database/sql, existing db/config/source packages

---

### Task 1: Retry Logic with Exponential Backoff

**Files:**
- Create: `internal/downloader/retry.go`
- Test: `internal/downloader/retry_test.go`

- [ ] **Step 1: Write the test**

Create file `internal/downloader/retry_test.go`:

```go
package downloader

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	cfg := RetryConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
	}

	// Attempt 0 should give base delay (with jitter)
	d := CalculateBackoff(0, cfg)
	if d < 750*time.Millisecond || d > 1250*time.Millisecond {
		t.Errorf("attempt 0: got %v, want ~1s", d)
	}

	// Attempt 3 should be capped at MaxDelay
	d = CalculateBackoff(10, cfg)
	if d > cfg.MaxDelay+cfg.MaxDelay/4 {
		t.Errorf("attempt 10: got %v, exceeds max delay %v", d, cfg.MaxDelay)
	}
}

func TestRetryOperation_Success(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond, Multiplier: 2.0}
	calls := 0
	err := RetryOperation(context.Background(), cfg, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryOperation_EventualSuccess(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 5, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond, Multiplier: 2.0}
	calls := 0
	err := RetryOperation(context.Background(), cfg, func() error {
		calls++
		if calls < 3 {
			return errors.New("temporary error")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryOperation_AllFail(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: 1 * time.Millisecond, MaxDelay: 10 * time.Millisecond, Multiplier: 2.0}
	calls := 0
	err := RetryOperation(context.Background(), cfg, func() error {
		calls++
		return errors.New("persistent error")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetryOperation_ContextCancelled(t *testing.T) {
	cfg := RetryConfig{MaxAttempts: 10, BaseDelay: 100 * time.Millisecond, MaxDelay: 1 * time.Second, Multiplier: 2.0}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := RetryOperation(ctx, cfg, func() error {
		return errors.New("should not retry")
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Create `internal/downloader/retry.go`**

```go
package downloader

import (
	"context"
	"math/rand"
	"time"
)

// RetryConfig holds retry settings.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
	}
}

// CalculateBackoff returns backoff duration with jitter for the given attempt.
func CalculateBackoff(attempt int, cfg RetryConfig) time.Duration {
	delay := float64(cfg.BaseDelay)
	for i := 0; i < attempt; i++ {
		delay *= cfg.Multiplier
	}
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}
	// Add ±25% jitter
	jitter := delay * 0.25 * (rand.Float64()*2 - 1)
	delay += jitter
	return time.Duration(delay)
}

// RetryOperation executes fn with exponential backoff until success or max attempts.
func RetryOperation(ctx context.Context, cfg RetryConfig, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// Don't sleep after the last attempt
		if attempt < cfg.MaxAttempts-1 {
			backoff := CalculateBackoff(attempt, cfg)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return lastErr
}
```

- [ ] **Step 4: Run tests, verify pass**

- [ ] **Step 5: Commit**

```bash
git add internal/downloader/
git commit -m "feat: add retry logic with exponential backoff and jitter"
```

---

### Task 2: File Download with Chunked Transfer

**Files:**
- Create: `internal/downloader/chunk.go`
- Test: `internal/downloader/chunk_test.go`

- [ ] **Step 1: Write the test**

Create file `internal/downloader/chunk_test.go`:

```go
package downloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadFile_Simple(t *testing.T) {
	content := "hello world this is test content for download"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "46")
		w.Write([]byte(content))
	}))
	defer server.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "test.mp3")

	err := DownloadFile(context.Background(), server.URL, dest, nil)
	if err != nil {
		t.Fatalf("DownloadFile() error: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %q, want %q", string(data), content)
	}
}

func TestDownloadFile_CreatesParentDirs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data"))
	}))
	defer server.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "sub", "dir", "test.mp3")

	err := DownloadFile(context.Background(), server.URL, dest, nil)
	if err != nil {
		t.Fatalf("DownloadFile() error: %v", err)
	}
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Error("file not created")
	}
}

func TestDownloadFile_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "test.mp3")

	err := DownloadFile(context.Background(), server.URL, dest, nil)
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}

func TestDownloadFile_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		w.Write([]byte("start"))
		w.(http.Flusher).Flush()
		// Block until test is done
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dir := t.TempDir()
	dest := filepath.Join(dir, "test.mp3")

	err := DownloadFile(ctx, server.URL, dest, nil)
	if err == nil {
		t.Error("expected error on cancelled context")
	}
}

func TestDownloadFile_ProgressCallback(t *testing.T) {
	content := "test content data"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer server.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "test.mp3")

	var lastN int64
	err := DownloadFile(context.Background(), server.URL, dest, func(downloaded int64) {
		lastN = downloaded
	})
	if err != nil {
		t.Fatalf("DownloadFile() error: %v", err)
	}
	if lastN != int64(len(content)) {
		t.Errorf("progress callback reported %d bytes, want %d", lastN, len(content))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Create `internal/downloader/chunk.go`**

```go
package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// ProgressFunc is called with the total bytes downloaded so far.
type ProgressFunc func(downloaded int64)

var httpClient = &http.Client{
	Timeout: 0, // No timeout for downloads
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  true,
		MaxIdleConnsPerHost: 5,
	},
}

// DownloadFile downloads a URL to a local file path.
// Creates parent directories if needed. Calls progressFn with bytes downloaded.
func DownloadFile(ctx context.Context, url, destPath string, progressFn ProgressFunc) error {
	// Create parent dirs
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "audbookdl/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	var downloaded int64
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write file: %w", writeErr)
			}
			downloaded += int64(n)
			if progressFn != nil {
				progressFn(downloaded)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read body: %w", readErr)
		}
	}

	return nil
}
```

- [ ] **Step 4: Run tests, verify pass**

- [ ] **Step 5: Commit**

```bash
git add internal/downloader/
git commit -m "feat: add file download with progress callback and directory creation"
```

---

### Task 3: Album-Aware Download Manager

**Files:**
- Create: `internal/downloader/manager.go`
- Test: `internal/downloader/manager_test.go`

- [ ] **Step 1: Write the test**

Create file `internal/downloader/manager_test.go`:

```go
package downloader

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/source"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	conn, err := db.InitWithPath(dbPath)
	if err != nil {
		t.Fatalf("InitWithPath() error: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestManager_DownloadAudiobook(t *testing.T) {
	// Create a test server serving chapter files
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Write([]byte("fake mp3 content for " + r.URL.Path))
	}))
	defer server.Close()

	dbConn := setupTestDB(t)
	dir := t.TempDir()

	book := &source.Audiobook{
		ID:     "test-123",
		Title:  "Test Book",
		Author: "Test Author",
		Source: "librivox",
	}

	chapters := []*source.Chapter{
		{Index: 1, Title: "Chapter One", DownloadURL: server.URL + "/ch01.mp3", Format: "mp3", FileSize: 100},
		{Index: 2, Title: "Chapter Two", DownloadURL: server.URL + "/ch02.mp3", Format: "mp3", FileSize: 100},
	}

	mgr := NewManager(dbConn, dir, 3)
	err := mgr.DownloadAudiobook(context.Background(), book, chapters, nil)
	if err != nil {
		t.Fatalf("DownloadAudiobook() error: %v", err)
	}

	// Verify files exist
	expectedDir := filepath.Join(dir, "Test Author", "Test Book")
	for _, name := range []string{"01 - Chapter One.mp3", "02 - Chapter Two.mp3"} {
		path := filepath.Join(expectedDir, name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", path)
		}
	}

	// Verify download record in DB
	downloads, err := db.ListDownloads(dbConn)
	if err != nil {
		t.Fatalf("ListDownloads() error: %v", err)
	}
	if len(downloads) != 1 {
		t.Fatalf("expected 1 download, got %d", len(downloads))
	}
	if downloads[0].Status != db.StatusCompleted {
		t.Errorf("status = %q, want %q", downloads[0].Status, db.StatusCompleted)
	}
}

func TestManager_DownloadAudiobook_PartialFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/ch02.mp3" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte("fake mp3"))
	}))
	defer server.Close()

	dbConn := setupTestDB(t)
	dir := t.TempDir()

	book := &source.Audiobook{ID: "fail-test", Title: "Fail Book", Author: "Author", Source: "archive"}
	chapters := []*source.Chapter{
		{Index: 1, Title: "Ch 1", DownloadURL: server.URL + "/ch01.mp3", Format: "mp3"},
		{Index: 2, Title: "Ch 2", DownloadURL: server.URL + "/ch02.mp3", Format: "mp3"},
	}

	mgr := NewManager(dbConn, dir, 3)
	err := mgr.DownloadAudiobook(context.Background(), book, chapters, nil)
	if err == nil {
		t.Error("expected error when chapter download fails")
	}

	// Verify download marked as failed
	downloads, _ := db.ListDownloads(dbConn)
	if len(downloads) != 1 {
		t.Fatalf("expected 1 download, got %d", len(downloads))
	}
	if downloads[0].Status != db.StatusFailed {
		t.Errorf("status = %q, want %q", downloads[0].Status, db.StatusFailed)
	}
}

func TestManager_BuildFilePath(t *testing.T) {
	mgr := NewManager(nil, "/audiobooks", 3)

	tests := []struct {
		author  string
		title   string
		chapter *source.Chapter
		want    string
	}{
		{
			"Arthur Conan Doyle", "Sherlock Holmes",
			&source.Chapter{Index: 1, Title: "A Scandal in Bohemia", Format: "mp3"},
			"/audiobooks/Arthur Conan Doyle/Sherlock Holmes/01 - A Scandal in Bohemia.mp3",
		},
		{
			"Author", "Title",
			&source.Chapter{Index: 12, Title: "Chapter Twelve", Format: "m4b"},
			"/audiobooks/Author/Title/12 - Chapter Twelve.m4b",
		},
	}

	for _, tt := range tests {
		got := mgr.buildChapterPath(tt.author, tt.title, tt.chapter)
		if got != tt.want {
			t.Errorf("buildChapterPath() = %q, want %q", got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

- [ ] **Step 3: Create `internal/downloader/manager.go`**

```go
package downloader

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/source"
)

// DownloadProgressFunc reports progress for the entire audiobook download.
type DownloadProgressFunc func(chapterIndex, totalChapters int, chapterBytes int64)

// Manager orchestrates audiobook downloads.
type Manager struct {
	db            *sql.DB
	baseDir       string
	maxConcurrent int
}

// NewManager creates a download manager.
func NewManager(database *sql.DB, baseDir string, maxConcurrent int) *Manager {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	return &Manager{db: database, baseDir: baseDir, maxConcurrent: maxConcurrent}
}

// DownloadAudiobook downloads all chapters of an audiobook.
// Creates DB records, downloads files, updates status.
func (m *Manager) DownloadAudiobook(ctx context.Context, book *source.Audiobook, chapters []*source.Chapter, progressFn DownloadProgressFunc) error {
	// Calculate total size
	var totalSize int64
	for _, ch := range chapters {
		totalSize += ch.FileSize
	}

	// Create download record
	basePath := filepath.Join(m.baseDir, book.Author, book.Title)
	dlID, err := db.CreateDownload(m.db, &db.AudiobookDownload{
		AudiobookID: book.ID,
		Title:       book.Title,
		Author:      book.Author,
		Narrator:    book.Narrator,
		Source:      book.Source,
		BasePath:    basePath,
		TotalSize:   totalSize,
	})
	if err != nil {
		return fmt.Errorf("create download record: %w", err)
	}

	// Update status to downloading
	if err := db.UpdateDownloadStatus(m.db, dlID, db.StatusDownloading); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Create chapter records
	for _, ch := range chapters {
		filePath := m.buildChapterPath(book.Author, book.Title, ch)
		_, err := db.CreateChapterDownload(m.db, &db.ChapterDownload{
			DownloadID:   dlID,
			ChapterIndex: ch.Index,
			Title:        ch.Title,
			FilePath:     filePath,
			FileSize:     ch.FileSize,
		})
		if err != nil {
			return fmt.Errorf("create chapter record: %w", err)
		}
	}

	// Download chapters with concurrency limit
	sem := make(chan struct{}, m.maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var downloadErr error

	for i, ch := range chapters {
		select {
		case <-ctx.Done():
			db.UpdateDownloadStatus(m.db, dlID, db.StatusFailed)
			return ctx.Err()
		default:
		}

		wg.Add(1)
		go func(idx int, chapter *source.Chapter) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			filePath := m.buildChapterPath(book.Author, book.Title, chapter)

			retryCfg := DefaultRetryConfig()
			err := RetryOperation(ctx, retryCfg, func() error {
				return DownloadFile(ctx, chapter.DownloadURL, filePath, func(downloaded int64) {
					if progressFn != nil {
						progressFn(chapter.Index, len(chapters), downloaded)
					}
				})
			})

			if err != nil {
				mu.Lock()
				if downloadErr == nil {
					downloadErr = fmt.Errorf("chapter %d (%s): %w", chapter.Index, chapter.Title, err)
				}
				mu.Unlock()
			}
		}(i, ch)
	}

	wg.Wait()

	if downloadErr != nil {
		db.UpdateDownloadStatus(m.db, dlID, db.StatusFailed)
		return downloadErr
	}

	// Mark as completed
	if err := db.UpdateDownloadStatus(m.db, dlID, db.StatusCompleted); err != nil {
		return fmt.Errorf("mark completed: %w", err)
	}

	return nil
}

// buildChapterPath creates the file path for a chapter.
// Format: baseDir/Author/Title/01 - Chapter Title.mp3
func (m *Manager) buildChapterPath(author, title string, ch *source.Chapter) string {
	filename := fmt.Sprintf("%02d - %s.%s", ch.Index, ch.Title, ch.Format)
	return filepath.Join(m.baseDir, author, title, filename)
}
```

- [ ] **Step 4: Run tests, verify pass**

- [ ] **Step 5: Run all tests**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go test ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/downloader/
git commit -m "feat: add album-aware download manager with concurrent chapter downloads"
```

---

### Task 4: Verify Full Build and All Tests

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

```bash
cd ~/Documents/personal/audbookdl
CGO_ENABLED=0 /usr/local/go/bin/go test -v ./...
```

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
