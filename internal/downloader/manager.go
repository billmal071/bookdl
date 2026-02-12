package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/billmal071/bookdl/internal/config"
	"github.com/billmal071/bookdl/internal/db"
)

const (
	// DefaultChunkSize is 5MB
	DefaultChunkSize = 5 * 1024 * 1024
)

// createProgressBar creates a styled progress bar with speed, ETA, and colors
func createProgressBar(total int64, description string) *progressbar.ProgressBar {
	return progressbar.NewOptions64(
		total,
		progressbar.OptionSetDescription(description),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]█[reset]",
			SaucerHead:    "[green]▓[reset]",
			SaucerPadding: "[dark_gray]░[reset]",
			BarStart:      "[dark_gray]│[reset]",
			BarEnd:        "[dark_gray]│[reset]",
		}),
		progressbar.OptionOnCompletion(func() {
			fmt.Println()
		}),
	)
}

// createChunkProgressBar creates a progress bar for chunked downloads showing chunk info
func createChunkProgressBar(total int64, description string, currentChunk, totalChunks int) *progressbar.ProgressBar {
	desc := fmt.Sprintf("%s [chunk %d/%d]", description, currentChunk, totalChunks)
	return progressbar.NewOptions64(
		total,
		progressbar.OptionSetDescription(desc),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(25),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[cyan]█[reset]",
			SaucerHead:    "[cyan]▓[reset]",
			SaucerPadding: "[dark_gray]░[reset]",
			BarStart:      "[dark_gray]│[reset]",
			BarEnd:        "[dark_gray]│[reset]",
		}),
	)
}

// DownloadResult contains the result of a download operation
type DownloadResult struct {
	Download *db.Download
	Error    error
}

// Manager handles download operations
type Manager struct {
	httpClient    *http.Client
	chunkSize     int64
	maxConcurrent int
	mu            sync.RWMutex
	active        map[int64]context.CancelFunc
}

// NewManager creates a new download manager
func NewManager() *Manager {
	cfg := config.Get()
	chunkSize := cfg.Downloads.ChunkSize
	if chunkSize == 0 {
		chunkSize = DefaultChunkSize
	}

	maxConcurrent := cfg.Downloads.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	}

	return &Manager{
		httpClient: &http.Client{
			Timeout: 0, // No timeout for downloads
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  true,
				MaxIdleConnsPerHost: 5,
			},
		},
		chunkSize:     chunkSize,
		maxConcurrent: maxConcurrent,
		active:        make(map[int64]context.CancelFunc),
	}
}

// GetMaxConcurrent returns the maximum concurrent downloads setting
func (m *Manager) GetMaxConcurrent() int {
	return m.maxConcurrent
}

// StartConcurrent starts multiple downloads concurrently with progress tracking
func (m *Manager) StartConcurrent(ctx context.Context, downloads []*db.Download, progressFn func(id int64, status string, progress float64)) []DownloadResult {
	results := make([]DownloadResult, len(downloads))

	// Semaphore for limiting concurrency
	sem := make(chan struct{}, m.maxConcurrent)
	var wg sync.WaitGroup
	var resultMu sync.Mutex

	for i, download := range downloads {
		wg.Add(1)
		go func(idx int, dl *db.Download) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Notify start
			if progressFn != nil {
				progressFn(dl.ID, "starting", 0)
			}

			// Perform download
			err := m.StartDownload(ctx, dl)

			// Store result
			resultMu.Lock()
			results[idx] = DownloadResult{Download: dl, Error: err}
			resultMu.Unlock()

			// Notify completion
			if progressFn != nil {
				if err != nil {
					progressFn(dl.ID, "failed", 0)
				} else {
					progressFn(dl.ID, "completed", 100)
				}
			}
		}(i, download)
	}

	wg.Wait()
	return results
}

// StartDownload starts or resumes a download
func (m *Manager) StartDownload(ctx context.Context, download *db.Download) error {
	// Create cancellable context
	dlCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.active[download.ID] = cancel
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.active, download.ID)
		m.mu.Unlock()
	}()

	// Update status to downloading
	if err := db.UpdateStatus(download.ID, db.StatusDownloading, ""); err != nil {
		return err
	}

	// Check if server supports range requests
	supportsRange, totalSize, err := m.checkRangeSupport(dlCtx, download.DownloadURL)
	if err != nil {
		return fmt.Errorf("failed to check server capabilities: %w", err)
	}

	download.FileSize = totalSize

	if supportsRange && totalSize > m.chunkSize {
		return m.downloadChunked(dlCtx, download)
	}

	return m.downloadSimple(dlCtx, download)
}

// PauseDownload pauses an active download
func (m *Manager) PauseDownload(downloadID int64) error {
	m.mu.RLock()
	cancel, ok := m.active[downloadID]
	m.mu.RUnlock()

	if ok {
		cancel()
	}

	return db.UpdateStatus(downloadID, db.StatusPaused, "")
}

// checkRangeSupport checks if the server supports range requests
func (m *Manager) checkRangeSupport(ctx context.Context, url string) (bool, int64, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return false, 0, err
	}

	req.Header.Set("User-Agent", config.Get().Network.UserAgent)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		// HEAD might not be supported, try GET with Range
		return m.checkRangeSupportWithGet(ctx, url)
	}
	defer resp.Body.Close()

	acceptRanges := resp.Header.Get("Accept-Ranges")
	contentLength := resp.ContentLength

	return acceptRanges == "bytes", contentLength, nil
}

func (m *Manager) checkRangeSupportWithGet(ctx context.Context, url string) (bool, int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, 0, err
	}

	req.Header.Set("User-Agent", config.Get().Network.UserAgent)
	req.Header.Set("Range", "bytes=0-0")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPartialContent {
		// Parse Content-Range header
		contentRange := resp.Header.Get("Content-Range")
		var total int64
		fmt.Sscanf(contentRange, "bytes 0-0/%d", &total)
		return true, total, nil
	}

	return false, resp.ContentLength, nil
}

// ErrHTMLContent indicates the download returned HTML instead of a file
var ErrHTMLContent = fmt.Errorf("received HTML content instead of file")

// downloadSimple downloads without chunking
func (m *Manager) downloadSimple(ctx context.Context, download *db.Download) error {
	req, err := http.NewRequestWithContext(ctx, "GET", download.DownloadURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", config.Get().Network.UserAgent)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %s", resp.Status)
	}

	// Check content type - if it's HTML, this is likely an error page
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		return ErrHTMLContent
	}

	// Create temp file
	file, err := os.Create(download.TempPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create styled progress bar with speed and ETA
	bar := createProgressBar(resp.ContentLength, "Downloading")

	// Read the first 2KB to validate content (larger buffer catches more HTML errors)
	header := make([]byte, 2048)
	n, _ := io.ReadFull(resp.Body, header)
	if n > 0 {
		// Check for HTML content by looking at the beginning
		headerStr := strings.ToLower(string(header[:n]))
		if strings.Contains(headerStr, "<!doctype html") ||
			strings.Contains(headerStr, "<html") ||
			strings.Contains(headerStr, "<head") ||
			strings.Contains(headerStr, "<body") ||
			strings.Contains(headerStr, "<title>") ||
			strings.Contains(headerStr, "<!doctype") ||
			strings.Contains(headerStr, "<script") ||
			strings.Contains(headerStr, "cloudflare") ||
			strings.Contains(headerStr, "captcha") ||
			strings.Contains(headerStr, "access denied") ||
			strings.Contains(headerStr, "error 403") ||
			strings.Contains(headerStr, "error 404") {
			return ErrHTMLContent
		}

		// Write header to file
		if _, err := file.Write(header[:n]); err != nil {
			return err
		}
		bar.Add(n)
	}

	// Copy the rest with progress
	writer := io.MultiWriter(file, bar)
	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return err
	}

	// Move temp file to final location
	file.Close()
	return os.Rename(download.TempPath, download.FilePath)
}

// downloadChunked downloads with chunking for resumability
func (m *Manager) downloadChunked(ctx context.Context, download *db.Download) error {
	// Get or create chunks
	chunks, err := db.GetChunks(download.ID)
	if err != nil || len(chunks) == 0 {
		chunks = m.createChunks(download)
		if err := db.CreateChunks(download.ID, chunks); err != nil {
			return fmt.Errorf("failed to create chunks: %w", err)
		}
	}

	// Open or create temp file
	file, err := os.OpenFile(download.TempPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Pre-allocate file
	if err := file.Truncate(download.FileSize); err != nil {
		return err
	}

	// Calculate already downloaded and count incomplete chunks
	var downloaded int64
	var incompleteChunks int
	for _, chunk := range chunks {
		if chunk.Status == "completed" {
			downloaded += (chunk.EndByte - chunk.StartByte + 1)
		} else {
			downloaded += chunk.Downloaded
			incompleteChunks++
		}
	}

	// Create styled progress bar with speed and ETA
	bar := createProgressBar(download.FileSize, fmt.Sprintf("Downloading (%d chunks)", len(chunks)))

	// Set initial progress
	bar.Set64(downloaded)

	// Download each incomplete chunk
	chunkNum := 0
	for _, chunk := range chunks {
		if chunk.Status == "completed" {
			continue
		}
		chunkNum++

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Update description to show current chunk
		bar.Describe(fmt.Sprintf("Chunk %d/%d", len(chunks)-incompleteChunks+chunkNum, len(chunks)))

		if err := m.downloadChunk(ctx, download, chunk, file, bar); err != nil {
			return err
		}
	}

	// Move temp file to final location
	file.Close()
	return os.Rename(download.TempPath, download.FilePath)
}

// createChunks creates chunk definitions for a download
func (m *Manager) createChunks(download *db.Download) []*db.Chunk {
	var chunks []*db.Chunk
	numChunks := (download.FileSize + m.chunkSize - 1) / m.chunkSize

	for i := int64(0); i < numChunks; i++ {
		start := i * m.chunkSize
		end := start + m.chunkSize - 1
		if end >= download.FileSize {
			end = download.FileSize - 1
		}

		chunks = append(chunks, &db.Chunk{
			ChunkIndex: int(i),
			StartByte:  start,
			EndByte:    end,
			Status:     "pending",
		})
	}

	return chunks
}

// downloadChunk downloads a single chunk
func (m *Manager) downloadChunk(ctx context.Context, download *db.Download, chunk *db.Chunk, file *os.File, bar *progressbar.ProgressBar) error {
	// Calculate resume position
	startPos := chunk.StartByte + chunk.Downloaded

	var resp *http.Response
	retryCfg := DefaultRetryConfig()

	// Retry with exponential backoff
	err := RetryOperation(ctx, retryCfg, func() (int, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", download.DownloadURL, nil)
		if err != nil {
			return 0, err
		}

		req.Header.Set("User-Agent", config.Get().Network.UserAgent)
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", startPos, chunk.EndByte))

		var reqErr error
		resp, reqErr = m.httpClient.Do(req)
		if reqErr != nil {
			return 0, reqErr
		}

		if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
			statusCode := resp.StatusCode
			resp.Body.Close()
			return statusCode, fmt.Errorf("server returned %d", statusCode)
		}

		return resp.StatusCode, nil
	})

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Seek to correct position in file
	if _, err := file.Seek(startPos, io.SeekStart); err != nil {
		return err
	}

	// Read and write in small buffers for better progress tracking
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		select {
		case <-ctx.Done():
			// Save progress before returning
			db.UpdateChunkProgress(chunk.ID, chunk.Downloaded)
			return ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return writeErr
			}
			chunk.Downloaded += int64(n)
			bar.Add(n)

			// Periodically save progress (every 256KB to minimize data loss on crash)
			if chunk.Downloaded%(256*1024) == 0 {
				db.UpdateProgressAtomic(download.ID, chunk.ID, chunk.Downloaded, download.DownloadedSize+chunk.Downloaded)
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			db.UpdateChunkProgress(chunk.ID, chunk.Downloaded)
			return err
		}
	}

	// Mark chunk completed
	return db.MarkChunkCompleted(chunk.ID)
}
