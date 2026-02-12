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
	// MaxRetries for failed requests
	MaxRetries = 3
)

// Manager handles download operations
type Manager struct {
	httpClient *http.Client
	chunkSize  int64
	mu         sync.RWMutex
	active     map[int64]context.CancelFunc
}

// NewManager creates a new download manager
func NewManager() *Manager {
	cfg := config.Get()
	chunkSize := cfg.Downloads.ChunkSize
	if chunkSize == 0 {
		chunkSize = DefaultChunkSize
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
		chunkSize: chunkSize,
		active:    make(map[int64]context.CancelFunc),
	}
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

	// Create progress bar
	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription("Downloading"),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	// Read the first few bytes to validate content
	header := make([]byte, 512)
	n, _ := io.ReadFull(resp.Body, header)
	if n > 0 {
		// Check for HTML content by looking at the beginning
		headerStr := strings.ToLower(string(header[:n]))
		if strings.Contains(headerStr, "<!doctype html") ||
			strings.Contains(headerStr, "<html") ||
			strings.Contains(headerStr, "<head") {
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

	fmt.Println() // New line after progress bar

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

	// Calculate already downloaded
	var downloaded int64
	for _, chunk := range chunks {
		if chunk.Status == "completed" {
			downloaded += (chunk.EndByte - chunk.StartByte + 1)
		} else {
			downloaded += chunk.Downloaded
		}
	}

	// Create progress bar
	bar := progressbar.NewOptions64(
		download.FileSize,
		progressbar.OptionSetDescription("Downloading"),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	// Set initial progress
	bar.Set64(downloaded)

	// Download each incomplete chunk
	for _, chunk := range chunks {
		if chunk.Status == "completed" {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := m.downloadChunk(ctx, download, chunk, file, bar); err != nil {
			return err
		}
	}

	fmt.Println() // New line after progress bar

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

	req, err := http.NewRequestWithContext(ctx, "GET", download.DownloadURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", config.Get().Network.UserAgent)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", startPos, chunk.EndByte))

	var resp *http.Response
	var lastErr error

	// Retry logic
	for attempt := 0; attempt < MaxRetries; attempt++ {
		resp, err = m.httpClient.Do(req)
		if err == nil && (resp.StatusCode == http.StatusPartialContent || resp.StatusCode == http.StatusOK) {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		lastErr = err
		if err == nil {
			lastErr = fmt.Errorf("server returned %d", resp.StatusCode)
		}
		time.Sleep(time.Second * time.Duration(attempt+1))
	}

	if lastErr != nil {
		return lastErr
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

			// Periodically save progress (every 1MB)
			if chunk.Downloaded%(1024*1024) == 0 {
				db.UpdateChunkProgress(chunk.ID, chunk.Downloaded)
				db.UpdateProgress(download.ID, download.DownloadedSize+chunk.Downloaded)
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
