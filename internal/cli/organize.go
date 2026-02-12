package cli

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/billmal071/bookdl/internal/anna"
	"github.com/billmal071/bookdl/internal/config"
)

// OrganizedPath returns the organized file path based on config and book metadata
func OrganizedPath(baseDir string, book *anna.Book, filename string) string {
	cfg := config.Get()
	mode := cfg.Files.OrganizeMode

	// If flat mode or no book info, just return base path
	if mode == "flat" || mode == "" || book == nil {
		return filepath.Join(baseDir, filename)
	}

	var subDir string

	switch mode {
	case "author":
		author := sanitizePathComponent(book.Authors)
		if author == "" {
			author = "Unknown Author"
		}
		subDir = author

	case "format":
		format := strings.ToUpper(book.Format)
		if format == "" {
			format = "Other"
		}
		subDir = format

	case "year":
		year := book.Year
		if year == "" {
			year = "Unknown Year"
		}
		subDir = year

	case "custom":
		subDir = expandPattern(cfg.Files.OrganizePattern, book)

	default:
		// Unknown mode, use flat
		return filepath.Join(baseDir, filename)
	}

	// Handle file renaming if enabled
	if cfg.Files.RenameFiles && book.Title != "" {
		filename = buildFilename(book)
	}

	return filepath.Join(baseDir, subDir, filename)
}

// expandPattern expands a custom pattern with book metadata
func expandPattern(pattern string, book *anna.Book) string {
	if pattern == "" {
		return ""
	}

	replacements := map[string]string{
		"{author}":    sanitizePathComponent(book.Authors),
		"{title}":     sanitizePathComponent(book.Title),
		"{year}":      book.Year,
		"{format}":    strings.ToUpper(book.Format),
		"{language}":  book.Language,
		"{publisher}": sanitizePathComponent(book.Publisher),
	}

	result := pattern
	for placeholder, value := range replacements {
		if value == "" {
			value = "Unknown"
		}
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

// buildFilename creates a filename from book metadata
func buildFilename(book *anna.Book) string {
	var parts []string

	// Start with author if available
	if book.Authors != "" {
		author := firstAuthor(book.Authors)
		if author != "" {
			parts = append(parts, sanitizePathComponent(author))
		}
	}

	// Add title
	title := sanitizePathComponent(book.Title)
	if title != "" {
		parts = append(parts, title)
	}

	// Add year in parentheses if available
	if book.Year != "" {
		parts = append(parts, "("+book.Year+")")
	}

	name := strings.Join(parts, " - ")
	if name == "" {
		name = "book"
	}

	// Add extension
	ext := strings.ToLower(book.Format)
	if ext == "" {
		ext = "epub"
	}

	return name + "." + ext
}

// firstAuthor extracts the first author from a potentially comma-separated list
func firstAuthor(authors string) string {
	// Split by common separators
	for _, sep := range []string{",", ";", "&", " and "} {
		if idx := strings.Index(authors, sep); idx > 0 {
			return strings.TrimSpace(authors[:idx])
		}
	}
	return strings.TrimSpace(authors)
}

// sanitizePathComponent removes/replaces characters invalid in file paths
func sanitizePathComponent(s string) string {
	if s == "" {
		return ""
	}

	// Remove or replace invalid characters
	invalid := regexp.MustCompile(`[<>:"/\\|?*]`)
	s = invalid.ReplaceAllString(s, "_")

	// Replace multiple spaces/underscores with single
	s = regexp.MustCompile(`[\s_]+`).ReplaceAllString(s, " ")

	// Trim whitespace
	s = strings.TrimSpace(s)

	// Limit length
	if len(s) > 80 {
		s = s[:80]
	}

	return s
}
