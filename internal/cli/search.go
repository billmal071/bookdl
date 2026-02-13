package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/billmal071/bookdl/internal/anna"
	"github.com/billmal071/bookdl/internal/config"
	"github.com/billmal071/bookdl/internal/db"
	"github.com/billmal071/bookdl/internal/tui"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for books",
	Long: `Search Anna's Archive for books matching the query.

By default, shows an interactive selector to choose from the results.
Use -d/--download to immediately start downloading the selected book.
Use -q/--queue for multi-select mode to add multiple books to the download queue.

Examples:
  bookdl search "clean code"
  bookdl search -n 5 "golang programming"
  bookdl search -f epub "design patterns"
  bookdl search -l english "machine learning"
  bookdl search --year 2020-2024 "python"
  bookdl search --max-size 10MB "algorithms"
  bookdl search -d "pragmatic programmer"
  bookdl search -q "programming books"     # Multi-select to queue
  bookdl search --history                  # Show search history`,
	Args: cobra.ArbitraryArgs,
	RunE: runSearch,
}

// filterOptions holds all search filter settings
type filterOptions struct {
	format   string
	language string
	year     string
	maxSize  string
}

func init() {
	searchCmd.Flags().IntP("limit", "n", 5, "number of results to show")
	searchCmd.Flags().StringP("format", "f", "", "filter by format (epub, pdf, mobi, djvu)")
	searchCmd.Flags().StringP("language", "l", "", "filter by language (english, spanish, etc.)")
	searchCmd.Flags().String("year", "", "filter by year (2020) or year range (2020-2024)")
	searchCmd.Flags().String("max-size", "", "filter by maximum file size (e.g., 10MB, 1GB)")
	searchCmd.Flags().BoolP("download", "d", false, "immediately download selected book")
	searchCmd.Flags().BoolP("queue", "q", false, "multi-select mode: add multiple books to download queue")
	searchCmd.Flags().Bool("no-interactive", false, "disable interactive mode, just print results")
	searchCmd.Flags().Bool("history", false, "show search history")
}

func runSearch(cmd *cobra.Command, args []string) error {
	showHistory, _ := cmd.Flags().GetBool("history")

	// Show search history if requested
	if showHistory {
		return showSearchHistory()
	}

	// Require query if not showing history
	if len(args) == 0 {
		return fmt.Errorf("search query required")
	}

	query := strings.Join(args, " ")
	limit, _ := cmd.Flags().GetInt("limit")
	autoDownload, _ := cmd.Flags().GetBool("download")
	queueMode, _ := cmd.Flags().GetBool("queue")
	noInteractive, _ := cmd.Flags().GetBool("no-interactive")

	// Collect filter options
	filters := filterOptions{
		format:   getString(cmd, "format"),
		language: getString(cmd, "language"),
		year:     getString(cmd, "year"),
		maxSize:  getString(cmd, "max-size"),
	}

	// Show search info with active filters
	Printf("Searching for: %s\n", query)
	if filters.hasAny() {
		Printf("Filters: %s\n", filters.String())
	}

	// Create client and search
	client := anna.NewClient()

	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	// Get extra results for filtering (more if filters are active)
	searchLimit := limit * 3
	if filters.hasAny() {
		searchLimit = limit * 5 // Get more results when filtering
	}
	if searchLimit < 20 {
		searchLimit = 20
	}

	var books []*anna.Book

	// Try to get from cache if enabled
	cfg := config.Get()
	if cfg.Cache.Enabled {
		filterMap := filters.toMap()
		cacheKey := db.GenerateCacheKey(query, filterMap)

		cached, err := db.GetCachedSearch(cacheKey)
		if err == nil && cached != nil {
			// Cache hit
			if err := json.Unmarshal([]byte(cached.ResultsJSON), &books); err == nil {
				Printf("Using cached results (%d found)\n", len(books))
			} else {
				// Cache corrupted, fetch fresh
				books = nil
			}
		}

		// Clean expired cache entries periodically
		go db.CleanExpiredCache()
	}

	// If not in cache, fetch from API
	if books == nil {
		var err error
		books, err = client.Search(ctx, query, searchLimit)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}

		// Save to cache if enabled
		if cfg.Cache.Enabled {
			filterMap := filters.toMap()
			cacheKey := db.GenerateCacheKey(query, filterMap)
			if resultsJSON, err := json.Marshal(books); err == nil {
				filtersJSON, _ := json.Marshal(filterMap)
				db.SaveCachedSearch(cacheKey, query, string(filtersJSON), string(resultsJSON), len(books), cfg.Cache.TTL)
			}
		}
	}

	// Apply all filters
	books = applyFilters(books, filters)

	// Limit results
	if len(books) > limit {
		books = books[:limit]
	}

	if len(books) == 0 {
		fmt.Println("No books found matching your query.")
		return nil
	}

	Printf("Found %d result(s)\n\n", len(books))

	// Save search to history
	saveSearchHistory(query, len(books), filters)

	// Non-interactive mode: just print results
	if noInteractive {
		printBooks(books)
		return nil
	}

	// Create load more function for pagination
	currentPage := 1
	loadMore := func() ([]*anna.Book, error) {
		currentPage++
		newCtx, newCancel := context.WithTimeout(cmd.Context(), 60*time.Second)
		defer newCancel()

		moreBooks, err := client.SearchPage(newCtx, query, searchLimit, currentPage)
		if err != nil {
			return nil, err
		}

		// Apply all filters
		moreBooks = applyFilters(moreBooks, filters)

		// Limit results
		if len(moreBooks) > limit {
			moreBooks = moreBooks[:limit]
		}

		return moreBooks, nil
	}

	// Queue mode: multi-select
	if queueMode {
		selectedBooks, err := tui.RunMultiSelector(books, loadMore)
		if err != nil {
			return fmt.Errorf("selection failed: %w", err)
		}

		if len(selectedBooks) == 0 {
			return nil // User cancelled
		}

		fmt.Println()

		// Add selected books to queue
		added := 0
		for _, book := range selectedBooks {
			if err := addToQueue(book); err != nil {
				Errorf("failed to queue %s: %v", book.Title, err)
			} else {
				added++
				fmt.Printf("Queued: %s\n", book.Title)
			}
		}

		if added > 0 {
			Successf("Added %d book(s) to the download queue.", added)
			fmt.Println("Run 'bookdl queue' to view the queue or 'bookdl resume all' to start downloading.")
		}
		return nil
	}

	// Interactive selection with load more support (single select)
	selected, err := tui.RunSelectorWithLoadMore(books, loadMore)
	if err != nil {
		return fmt.Errorf("selection failed: %w", err)
	}

	if selected == nil {
		return nil // User cancelled
	}

	fmt.Println()

	if autoDownload {
		return startBookDownload(cmd.Context(), selected)
	}

	// Print selected book info
	fmt.Printf("Selected: %s\n", selected.Title)
	fmt.Printf("MD5: %s\n", selected.MD5Hash)
	fmt.Printf("\nTo download, run:\n")
	fmt.Printf("  bookdl download %s\n", selected.MD5Hash)

	return nil
}

// addToQueue adds a book to the download queue as a pending download
func addToQueue(book *anna.Book) error {
	// Check if already in queue
	existing, err := db.GetDownloadByHash(book.MD5Hash)
	if err == nil && existing != nil {
		if existing.Status == db.StatusPending {
			return fmt.Errorf("already in queue")
		}
		if existing.Status == db.StatusCompleted {
			return fmt.Errorf("already downloaded")
		}
	}

	// Create a pending download
	download := &db.Download{
		MD5Hash:   book.MD5Hash,
		Title:     book.Title,
		Authors:   book.Authors,
		Publisher: book.Publisher,
		Language:  book.Language,
		Format:    book.Format,
		FileSize:  book.SizeBytes,
		SourceURL: book.PageURL,
		Status:    db.StatusPending,
	}

	return db.CreateDownload(download)
}

// getString safely gets a string flag value
func getString(cmd *cobra.Command, name string) string {
	val, _ := cmd.Flags().GetString(name)
	return val
}

// hasAny returns true if any filter is set
func (f filterOptions) hasAny() bool {
	return f.format != "" || f.language != "" || f.year != "" || f.maxSize != ""
}

// String returns a human-readable representation of active filters
func (f filterOptions) String() string {
	var parts []string
	if f.format != "" {
		parts = append(parts, fmt.Sprintf("format=%s", f.format))
	}
	if f.language != "" {
		parts = append(parts, fmt.Sprintf("language=%s", f.language))
	}
	if f.year != "" {
		parts = append(parts, fmt.Sprintf("year=%s", f.year))
	}
	if f.maxSize != "" {
		parts = append(parts, fmt.Sprintf("max-size=%s", f.maxSize))
	}
	return strings.Join(parts, ", ")
}

// toMap converts filter options to a map for caching
func (f filterOptions) toMap() map[string]string {
	m := make(map[string]string)
	if f.format != "" {
		m["format"] = f.format
	}
	if f.language != "" {
		m["language"] = f.language
	}
	if f.year != "" {
		m["year"] = f.year
	}
	if f.maxSize != "" {
		m["max-size"] = f.maxSize
	}
	return m
}

// applyFilters applies all filters to the book list
func applyFilters(books []*anna.Book, filters filterOptions) []*anna.Book {
	if !filters.hasAny() {
		return books
	}

	var filtered []*anna.Book
	for _, book := range books {
		if filters.format != "" && !matchesFormat(book, filters.format) {
			continue
		}
		if filters.language != "" && !matchesLanguage(book, filters.language) {
			continue
		}
		if filters.year != "" && !matchesYear(book, filters.year) {
			continue
		}
		if filters.maxSize != "" && !matchesMaxSize(book, filters.maxSize) {
			continue
		}
		filtered = append(filtered, book)
	}
	return filtered
}

// matchesFormat checks if a book matches the format filter
func matchesFormat(book *anna.Book, format string) bool {
	return strings.EqualFold(book.Format, format)
}

// matchesLanguage checks if a book matches the language filter
func matchesLanguage(book *anna.Book, language string) bool {
	return strings.EqualFold(book.Language, language)
}

// matchesYear checks if a book matches the year filter
// Supports single year (2020) or range (2020-2024)
func matchesYear(book *anna.Book, yearFilter string) bool {
	if book.Year == "" {
		return false
	}

	// Extract numeric year from book
	bookYear := extractYear(book.Year)
	if bookYear == 0 {
		return false
	}

	// Check for year range (e.g., "2020-2024")
	if strings.Contains(yearFilter, "-") {
		parts := strings.Split(yearFilter, "-")
		if len(parts) != 2 {
			return false
		}
		startYear, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		endYear, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil {
			return false
		}
		return bookYear >= startYear && bookYear <= endYear
	}

	// Single year match
	filterYear, err := strconv.Atoi(strings.TrimSpace(yearFilter))
	if err != nil {
		return false
	}
	return bookYear == filterYear
}

// extractYear extracts a 4-digit year from a string
func extractYear(s string) int {
	re := regexp.MustCompile(`\b(19|20)\d{2}\b`)
	match := re.FindString(s)
	if match == "" {
		return 0
	}
	year, _ := strconv.Atoi(match)
	return year
}

// matchesMaxSize checks if a book is within the max size limit
func matchesMaxSize(book *anna.Book, maxSize string) bool {
	if book.Size == "" && book.SizeBytes == 0 {
		return true // Allow books with unknown size
	}

	maxBytes := parseSize(maxSize)
	if maxBytes == 0 {
		return true // Invalid max size, don't filter
	}

	// Use SizeBytes if available, otherwise parse Size string
	var bookBytes int64
	if book.SizeBytes > 0 {
		bookBytes = book.SizeBytes
	} else {
		bookBytes = parseSize(book.Size)
	}

	if bookBytes == 0 {
		return true // Can't determine size, include it
	}

	return bookBytes <= maxBytes
}

// parseSize parses a size string like "10MB" or "1.5 GB" to bytes
func parseSize(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0
	}

	re := regexp.MustCompile(`^(\d+\.?\d*)\s*(B|KB|MB|GB|TB)?$`)
	match := re.FindStringSubmatch(s)
	if len(match) < 2 {
		return 0
	}

	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0
	}

	unit := "B"
	if len(match) >= 3 && match[2] != "" {
		unit = match[2]
	}

	multipliers := map[string]float64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	return int64(value * multipliers[unit])
}

// printBooks prints books in a simple format
func printBooks(books []*anna.Book) {
	for i, book := range books {
		fmt.Printf("%d. %s\n", i+1, book.Title)
		if book.Authors != "" {
			fmt.Printf("   Author: %s\n", book.Authors)
		}
		fmt.Printf("   Format: %s", book.Format)
		if book.Size != "" {
			fmt.Printf(" | Size: %s", book.Size)
		}
		if book.Language != "" {
			fmt.Printf(" | Language: %s", book.Language)
		}
		fmt.Println()
		fmt.Printf("   MD5: %s\n", book.MD5Hash)
		fmt.Println()
	}
}

// startBookDownload initiates a download for the selected book
func startBookDownload(ctx context.Context, book *anna.Book) error {
	// This will be implemented in the download command
	// For now, just print the command to run
	fmt.Printf("Starting download: %s\n", book.Title)
	return runDownloadByHash(ctx, book.MD5Hash, "", book)
}

// saveSearchHistory saves a search to the history database
func saveSearchHistory(query string, resultCount int, filters filterOptions) {
	dbFilters := db.SearchFilters{
		Format:   filters.format,
		Language: filters.language,
		Year:     filters.year,
		MaxSize:  filters.maxSize,
	}
	// Ignore errors - history is not critical
	db.AddSearchHistory(query, resultCount, dbFilters)
}

// showSearchHistory displays recent search history
func showSearchHistory() error {
	history, err := db.GetUniqueSearchHistory(20)
	if err != nil {
		return fmt.Errorf("failed to get search history: %w", err)
	}

	if len(history) == 0 {
		fmt.Println("No search history.")
		fmt.Println("\nSearches are saved automatically when you search for books.")
		return nil
	}

	fmt.Printf("Recent Searches (%d):\n\n", len(history))

	for i, h := range history {
		// Query and result count
		fmt.Printf("  %d. \"%s\" (%d results)\n", i+1, h.Query, h.ResultCount)

		// Show filters if any were used
		var filterParts []string
		if h.Filters.Format != "" {
			filterParts = append(filterParts, "format="+h.Filters.Format)
		}
		if h.Filters.Language != "" {
			filterParts = append(filterParts, "language="+h.Filters.Language)
		}
		if h.Filters.Year != "" {
			filterParts = append(filterParts, "year="+h.Filters.Year)
		}
		if h.Filters.MaxSize != "" {
			filterParts = append(filterParts, "max-size="+h.Filters.MaxSize)
		}
		if len(filterParts) > 0 {
			fmt.Printf("     Filters: %s\n", strings.Join(filterParts, ", "))
		}

		// Date
		fmt.Printf("     %s\n", h.CreatedAt.Format("2006-01-02 15:04"))
		fmt.Println()
	}

	fmt.Println("To repeat a search, copy the query above.")
	fmt.Println("To clear history: bookdl history clear")
	return nil
}
