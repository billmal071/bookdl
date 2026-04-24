# audbookdl CLI Commands Implementation Plan (Plan 4 of 8)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement all Cobra CLI subcommands that wire together the source clients, search orchestrator, download manager, and database CRUD into a usable command-line tool.

**Architecture:** Each command is a separate file in `internal/cli/`. Commands use the initialized DB and config from `root.go`'s PersistentPreRunE. The search command uses the search orchestrator + a simple table output (TUI selector comes in Plan 5).

**Tech Stack:** Go 1.22+, github.com/spf13/cobra, existing internal packages

---

### Task 1: Search Command

**Files:**
- Create: `internal/cli/search.go`

- [ ] **Step 1: Create `internal/cli/search.go`**

```go
package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/billmal071/audbookdl/internal/config"
	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/archive"
	"github.com/billmal071/audbookdl/internal/librivox"
	"github.com/billmal071/audbookdl/internal/loyalbooks"
	"github.com/billmal071/audbookdl/internal/openlibrary"
	"github.com/billmal071/audbookdl/internal/search"
	"github.com/billmal071/audbookdl/internal/source"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for audiobooks",
	Long: `Search across LibriVox, Internet Archive, Loyal Books, and Open Library.

Examples:
  audbookdl search "sherlock holmes"
  audbookdl search -n 5 "pride and prejudice"
  audbookdl search -s librivox "war and peace"
  audbookdl search -a "dickens" "christmas carol"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().IntP("limit", "n", 10, "number of results")
	searchCmd.Flags().StringP("source", "s", "", "filter source (librivox, archive, loyalbooks, openlibrary)")
	searchCmd.Flags().StringP("language", "l", "", "filter by language")
	searchCmd.Flags().StringP("author", "a", "", "filter by author")
	searchCmd.Flags().StringP("format", "f", "", "prefer format (mp3, m4b, ogg)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.Join(args, " ")
	limit, _ := cmd.Flags().GetInt("limit")
	sourceFilter, _ := cmd.Flags().GetString("source")
	language, _ := cmd.Flags().GetString("language")
	author, _ := cmd.Flags().GetString("author")

	opts := source.SearchOptions{
		Limit:    limit,
		Language: language,
		Author:   author,
	}

	http := httpclient.New()
	searcher := buildSearcher(http, sourceFilter)

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	fmt.Println("Searching...")
	books, err := searcher.Search(ctx, query, opts)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(books) == 0 {
		fmt.Println("No audiobooks found.")
		return nil
	}

	// Save search history
	db.AddSearchHistory(db.DB(), query, sourceFilter, len(books))

	// Print results
	for i, b := range books {
		fmt.Printf("\n%d. %s\n", i+1, b.Title)
		fmt.Printf("   Author: %s", b.Author)
		if b.Narrator != "" {
			fmt.Printf(" · Narrator: %s", b.Narrator)
		}
		fmt.Println()
		if b.Duration > 0 {
			fmt.Printf("   %s · ", b.DurationFormatted())
		}
		if b.ChapterCount > 0 {
			fmt.Printf("%d chapters · ", b.ChapterCount)
		}
		fmt.Printf("%s · %s\n", b.Format, b.Source)
	}

	fmt.Printf("\nFound %d audiobooks.\n", len(books))
	return nil
}

func buildSearcher(http *httpclient.Client, sourceFilter string) *search.Searcher {
	cfg := config.Get()
	var sources []source.Source

	enabledSources := cfg.Search.Sources
	if sourceFilter != "" {
		enabledSources = []string{sourceFilter}
	}

	for _, s := range enabledSources {
		switch s {
		case "librivox":
			sources = append(sources, librivox.NewClient("", http))
		case "archive":
			sources = append(sources, archive.NewClient("", http))
		case "loyalbooks":
			sources = append(sources, loyalbooks.NewClient("", http))
		case "openlibrary":
			sources = append(sources, openlibrary.NewClient("", "", http))
		}
	}

	return search.New(sources...)
}
```

- [ ] **Step 2: Register in root.go**

Add `rootCmd.AddCommand(searchCmd)` to the `init()` function in `root.go`.

- [ ] **Step 3: Build and verify**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go build -o ./build/audbookdl ./cmd/audbookdl
./build/audbookdl search --help
```

- [ ] **Step 4: Commit**

```bash
git add internal/cli/
git commit -m "feat: add search command with multi-source querying"
```

---

### Task 2: Download Command

**Files:**
- Create: `internal/cli/download.go`

- [ ] **Step 1: Create `internal/cli/download.go`**

```go
package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/billmal071/audbookdl/internal/config"
	"github.com/billmal071/audbookdl/internal/db"
	"github.com/billmal071/audbookdl/internal/downloader"
	"github.com/billmal071/audbookdl/internal/httpclient"
	"github.com/billmal071/audbookdl/internal/source"
	"github.com/spf13/cobra"
)

var downloadCmd = &cobra.Command{
	Use:   "download [audiobook-id]",
	Short: "Download an audiobook by ID",
	Long: `Download an audiobook using its source-specific ID.

The ID can be obtained from search results. Use --source to specify
which source to fetch chapters from.

Examples:
  audbookdl download 1234 --source librivox
  audbookdl download adventures_sherlock_holmes_0711_librivox --source archive`,
	Args: cobra.ExactArgs(1),
	RunE: runDownload,
}

func init() {
	downloadCmd.Flags().StringP("source", "s", "librivox", "source (librivox, archive, loyalbooks, openlibrary)")
	downloadCmd.Flags().StringP("output", "o", "", "output directory (default: ~/Audiobooks)")
}

func runDownload(cmd *cobra.Command, args []string) error {
	bookID := args[0]
	sourceName, _ := cmd.Flags().GetString("source")
	outputDir, _ := cmd.Flags().GetString("output")

	cfg := config.Get()
	if outputDir == "" {
		outputDir = cfg.Download.Directory
	}

	http := httpclient.New()
	src := getSource(http, sourceName)
	if src == nil {
		return fmt.Errorf("unknown source: %s", sourceName)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
	defer cancel()

	fmt.Printf("Fetching chapter list from %s...\n", sourceName)
	chapters, err := src.GetChapters(ctx, bookID)
	if err != nil {
		return fmt.Errorf("get chapters: %w", err)
	}

	if len(chapters) == 0 {
		return fmt.Errorf("no chapters found for %s", bookID)
	}

	// We need audiobook metadata. For now, construct from available info.
	book := &source.Audiobook{
		ID:           bookID,
		Title:        bookID, // Will be improved when search+download are wired together
		Author:       "Unknown",
		Source:       sourceName,
		ChapterCount: len(chapters),
	}

	fmt.Printf("Downloading %d chapters to %s...\n", len(chapters), outputDir)

	mgr := downloader.NewManager(db.DB(), outputDir, cfg.Download.MaxConcurrent)
	err = mgr.DownloadAudiobook(ctx, book, chapters, func(chIdx, total int, bytes int64) {
		// Simple progress output
	})
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Println("Download complete!")
	return nil
}

func getSource(http *httpclient.Client, name string) source.Source {
	switch strings.ToLower(name) {
	case "librivox":
		return librivox.NewClient("", http)
	case "archive":
		return archive.NewClient("", http)
	case "loyalbooks":
		return loyalbooks.NewClient("", http)
	case "openlibrary":
		return openlibrary.NewClient("", "", http)
	default:
		return nil
	}
}
```

NOTE: The `getSource` function imports the source packages. Make sure the imports include:
```go
"github.com/billmal071/audbookdl/internal/archive"
"github.com/billmal071/audbookdl/internal/librivox"
"github.com/billmal071/audbookdl/internal/loyalbooks"
"github.com/billmal071/audbookdl/internal/openlibrary"
```

- [ ] **Step 2: Register in root.go**

Add `rootCmd.AddCommand(downloadCmd)` to init().

- [ ] **Step 3: Build and verify**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go build -o ./build/audbookdl ./cmd/audbookdl
./build/audbookdl download --help
```

- [ ] **Step 4: Commit**

```bash
git add internal/cli/
git commit -m "feat: add download command with source-specific chapter fetching"
```

---

### Task 3: List, Pause, Resume Commands

**Files:**
- Create: `internal/cli/list.go`
- Create: `internal/cli/pause.go`
- Create: `internal/cli/resume.go`

- [ ] **Step 1: Create `internal/cli/list.go`**

```go
package cli

import (
	"fmt"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all downloads",
	Long:  "Show all audiobook downloads with their status and progress.",
	RunE: func(cmd *cobra.Command, args []string) error {
		downloads, err := db.ListDownloads(db.DB())
		if err != nil {
			return fmt.Errorf("list downloads: %w", err)
		}

		if len(downloads) == 0 {
			fmt.Println("No downloads yet.")
			return nil
		}

		for _, dl := range downloads {
			progress := float64(0)
			if dl.TotalSize > 0 {
				progress = float64(dl.DownloadedSize) / float64(dl.TotalSize) * 100
			}

			statusIcon := statusToIcon(dl.Status)
			fmt.Printf("%s #%d %s — %s (%s) %.0f%%\n",
				statusIcon, dl.ID, dl.Title, dl.Author, dl.Status, progress)
		}

		return nil
	},
}

func statusToIcon(status db.DownloadStatus) string {
	switch status {
	case db.StatusCompleted:
		return "[done]"
	case db.StatusDownloading:
		return "[....]"
	case db.StatusPaused:
		return "[stop]"
	case db.StatusFailed:
		return "[fail]"
	default:
		return "[wait]"
	}
}
```

- [ ] **Step 2: Create `internal/cli/pause.go`**

```go
package cli

import (
	"fmt"
	"strconv"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var pauseCmd = &cobra.Command{
	Use:   "pause [download-id]",
	Short: "Pause an active download",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid download ID: %s", args[0])
		}

		if err := db.UpdateDownloadStatus(db.DB(), id, db.StatusPaused); err != nil {
			return fmt.Errorf("pause download: %w", err)
		}

		fmt.Printf("Download #%d paused.\n", id)
		return nil
	},
}
```

- [ ] **Step 3: Create `internal/cli/resume.go`**

```go
package cli

import (
	"fmt"
	"strconv"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume [download-id]",
	Short: "Resume a paused download",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid download ID: %s", args[0])
		}

		dl, err := db.GetDownload(db.DB(), id)
		if err != nil {
			return fmt.Errorf("get download: %w", err)
		}

		if dl.Status != db.StatusPaused && dl.Status != db.StatusFailed {
			return fmt.Errorf("download #%d is %s, not paused or failed", id, dl.Status)
		}

		// Mark as pending so the download manager can pick it up
		if err := db.UpdateDownloadStatus(db.DB(), id, db.StatusPending); err != nil {
			return fmt.Errorf("resume download: %w", err)
		}

		fmt.Printf("Download #%d queued for resume.\n", id)
		return nil
	},
}
```

- [ ] **Step 4: Register all three in root.go**

Add to init():
```go
rootCmd.AddCommand(listCmd)
rootCmd.AddCommand(pauseCmd)
rootCmd.AddCommand(resumeCmd)
```

- [ ] **Step 5: Build and verify**

- [ ] **Step 6: Commit**

```bash
git add internal/cli/
git commit -m "feat: add list, pause, and resume commands"
```

---

### Task 4: Bookmark and History Commands

**Files:**
- Create: `internal/cli/bookmark.go`
- Create: `internal/cli/history.go`

- [ ] **Step 1: Create `internal/cli/bookmark.go`**

```go
package cli

import (
	"fmt"
	"strconv"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var bookmarkCmd = &cobra.Command{
	Use:   "bookmark",
	Short: "Manage bookmarks",
}

var bookmarkListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all bookmarks",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		bookmarks, err := db.ListBookmarks(db.DB())
		if err != nil {
			return fmt.Errorf("list bookmarks: %w", err)
		}
		if len(bookmarks) == 0 {
			fmt.Println("No bookmarks yet.")
			return nil
		}
		for _, bm := range bookmarks {
			fmt.Printf("#%d %s — %s (%s)\n", bm.ID, bm.Title, bm.Author, bm.Source)
			if bm.Note != "" {
				fmt.Printf("   Note: %s\n", bm.Note)
			}
		}
		return nil
	},
}

var bookmarkDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a bookmark",
	Aliases: []string{"rm"},
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid bookmark ID: %s", args[0])
		}
		if err := db.DeleteBookmark(db.DB(), id); err != nil {
			return fmt.Errorf("delete bookmark: %w", err)
		}
		fmt.Printf("Bookmark #%d deleted.\n", id)
		return nil
	},
}

func init() {
	bookmarkCmd.AddCommand(bookmarkListCmd)
	bookmarkCmd.AddCommand(bookmarkDeleteCmd)
}
```

- [ ] **Step 2: Create `internal/cli/history.go`**

```go
package cli

import (
	"fmt"

	"github.com/billmal071/audbookdl/internal/db"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show search history",
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		entries, err := db.ListSearchHistory(db.DB(), limit)
		if err != nil {
			return fmt.Errorf("list history: %w", err)
		}
		if len(entries) == 0 {
			fmt.Println("No search history.")
			return nil
		}
		for _, e := range entries {
			source := e.Source
			if source == "" {
				source = "all"
			}
			fmt.Printf("[%s] %q — %d results (%s)\n",
				e.CreatedAt.Format("2006-01-02 15:04"), e.Query, e.ResultCount, source)
		}
		return nil
	},
}

var historyClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear search history",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := db.ClearSearchHistory(db.DB()); err != nil {
			return fmt.Errorf("clear history: %w", err)
		}
		fmt.Println("Search history cleared.")
		return nil
	},
}

func init() {
	historyCmd.Flags().IntP("limit", "n", 20, "number of entries to show")
	historyCmd.AddCommand(historyClearCmd)
}
```

- [ ] **Step 3: Register in root.go**

Add to init():
```go
rootCmd.AddCommand(bookmarkCmd)
rootCmd.AddCommand(historyCmd)
```

- [ ] **Step 4: Build and verify**

- [ ] **Step 5: Commit**

```bash
git add internal/cli/
git commit -m "feat: add bookmark and history commands"
```

---

### Task 5: Config and Completion Commands

**Files:**
- Create: `internal/cli/config_cmd.go`
- Create: `internal/cli/completion.go`

- [ ] **Step 1: Create `internal/cli/config_cmd.go`**

```go
package cli

import (
	"fmt"

	"github.com/billmal071/audbookdl/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		val := config.GetValue(args[0])
		if val == nil {
			return fmt.Errorf("config key %q not found", args[0])
		}
		fmt.Printf("%s = %v\n", args[0], val)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Set(args[0], args[1]); err != nil {
			return fmt.Errorf("set config: %w", err)
		}
		fmt.Printf("%s = %s\n", args[0], args[1])
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show config file path",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.GetConfigPath())
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)
}
```

- [ ] **Step 2: Create `internal/cli/completion.go`**

```go
package cli

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for audbookdl.

  bash:       audbookdl completion bash > /etc/bash_completion.d/audbookdl
  zsh:        audbookdl completion zsh > "${fpath[1]}/_audbookdl"
  fish:       audbookdl completion fish > ~/.config/fish/completions/audbookdl.fish
  powershell: audbookdl completion powershell | Out-String | Invoke-Expression`,
	Args:      cobra.ExactValidArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return nil
		}
	},
}
```

- [ ] **Step 3: Register in root.go**

Add to init():
```go
rootCmd.AddCommand(configCmd)
rootCmd.AddCommand(completionCmd)
```

- [ ] **Step 4: Build and verify all commands show in help**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go build -o ./build/audbookdl ./cmd/audbookdl
./build/audbookdl --help
```

Expected: search, download, list, pause, resume, bookmark, history, config, completion, version all visible.

- [ ] **Step 5: Run all tests**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go test ./...
```

- [ ] **Step 6: Commit**

```bash
git add internal/cli/
git commit -m "feat: add config and completion commands"
```

---

### Task 6: Verify Full Build and All Tests

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

```bash
cd ~/Documents/personal/audbookdl
CGO_ENABLED=0 /usr/local/go/bin/go test -v ./...
```

- [ ] **Step 2: Build and verify all commands**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go build -o ./build/audbookdl ./cmd/audbookdl
./build/audbookdl --help
./build/audbookdl search --help
./build/audbookdl download --help
./build/audbookdl list --help
./build/audbookdl bookmark --help
./build/audbookdl history --help
./build/audbookdl config --help
./build/audbookdl completion --help
```

- [ ] **Step 3: Format and vet**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go fmt ./...
CGO_ENABLED=0 /usr/local/go/bin/go vet ./...
```
