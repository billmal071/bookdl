package search

import (
	"context"
	"sync"

	"github.com/billmal071/bookdl/internal/anna"
	"github.com/billmal071/bookdl/internal/liber3"
	"github.com/billmal071/bookdl/internal/zlibrary"
)

// Option defines which sources to search
type Option string

const (
	OptionAll      Option = "all"
	OptionAnna     Option = "anna"
	OptionZLibrary Option = "zlibrary"
	OptionLiber3   Option = "liber3"
)

// Searcher orchestrates searches across multiple sources
type Searcher struct {
	opt Option
}

// NewSearcher creates a new multi-source searcher
func NewSearcher(opt Option) *Searcher {
	return &Searcher{opt: opt}
}

// Search searches across configured sources and returns aggregated results
func (s *Searcher) Search(ctx context.Context, query string, limit int) ([]*anna.Book, error) {
	switch s.opt {
	case OptionAnna:
		return s.searchAnna(ctx, query, limit)
	case OptionZLibrary:
		return s.searchZLibrary(ctx, query, limit)
	case OptionLiber3:
		return s.searchLiber3(ctx, query, limit)
	case OptionAll:
		return s.searchBoth(ctx, query, limit)
	default:
		return s.searchBoth(ctx, query, limit)
	}
}

// searchZLibrary searches only Z-Library
func (s *Searcher) searchZLibrary(ctx context.Context, query string, limit int) ([]*anna.Book, error) {
	client := zlibrary.NewClient()
	books, err := client.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	// Convert zlibrary.Book to anna.Book and add source annotation
	var annaBooks []*anna.Book
	for _, book := range books {
		annaBook := &anna.Book{
			MD5Hash:     book.MD5Hash,
			Title:       book.Title,
			Authors:     book.Authors,
			Publisher:   book.Publisher,
			Year:        book.Year,
			Language:    book.Language,
			Format:      book.Format,
			Size:        book.Size,
			SizeBytes:   book.SizeBytes,
			PageURL:     book.PageURL,
			DownloadURL: book.DownloadURL,
			Source:      "zlibrary",
		}
		annaBooks = append(annaBooks, annaBook)
	}

	return annaBooks, nil
}

// searchLiber3 searches only Liber3
func (s *Searcher) searchLiber3(ctx context.Context, query string, limit int) ([]*anna.Book, error) {
	client := liber3.NewClient()
	books, err := client.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	// Convert liber3.Book to anna.Book and add source annotation
	var annaBooks []*anna.Book
	for _, book := range books {
		annaBook := &anna.Book{
			MD5Hash:   book.MD5Hash,
			Title:     book.Title,
			Authors:   book.Authors,
			Publisher: book.Publisher,
			Year:      book.Year,
			Language:  book.Language,
			Format:    book.Format,
			Size:      book.Size,
			SizeBytes: book.SizeBytes,
			PageURL:   book.PageURL,
			Source:    "liber3",
		}
		annaBooks = append(annaBooks, annaBook)
	}

	return annaBooks, nil
}

// searchAnna searches only Anna's Archive
func (s *Searcher) searchAnna(ctx context.Context, query string, limit int) ([]*anna.Book, error) {
	client := anna.NewClient()
	books, err := client.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	// Add source annotation
	for _, book := range books {
		book.Source = "anna"
	}

	return books, nil
}

// searchBoth searches both sources concurrently
func (s *Searcher) searchBoth(ctx context.Context, query string, limit int) ([]*anna.Book, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allBooks []*anna.Book
	var firstErr error

	// Search Anna's Archive
	wg.Add(1)
	go func() {
		defer wg.Done()
		annaBooks, err := s.searchAnna(ctx, query, limit)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		mu.Lock()
		allBooks = append(allBooks, annaBooks...)
		mu.Unlock()
	}()

	// Search Z-Library
	wg.Add(1)
	go func() {
		defer wg.Done()
		zlibraryBooks, err := s.searchZLibrary(ctx, query, limit)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		mu.Lock()
		allBooks = append(allBooks, zlibraryBooks...)
		mu.Unlock()
	}()

	// Search Liber3
	wg.Add(1)
	go func() {
		defer wg.Done()
		liber3Books, err := s.searchLiber3(ctx, query, limit)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		mu.Lock()
		allBooks = append(allBooks, liber3Books...)
		mu.Unlock()
	}()

	wg.Wait()

	if len(allBooks) == 0 && firstErr != nil {
		return nil, firstErr
	}

	return allBooks, nil
}

// SearchPage searches across sources with pagination support
func (s *Searcher) SearchPage(ctx context.Context, query string, limit int, page int) ([]*anna.Book, error) {
	// For now, delegate to Search (pagination not essential for multi-source support)
	return s.Search(ctx, query, limit)
}

// GetDownloadInfo retrieves download information for a book from the appropriate source
func (s *Searcher) GetDownloadInfo(ctx context.Context, md5Hash string, source string) (*anna.DownloadInfo, error) {
	// TODO: Route to correct source client
	client := anna.NewClient()
	return client.GetDownloadInfo(ctx, md5Hash)
}
