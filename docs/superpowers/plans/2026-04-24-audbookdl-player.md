# audbookdl Audio Player Implementation Plan (Plan 6 of 8)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a terminal audio player using gopxl/beep with ebitengine/oto backend. Supports play/pause, skip, chapter navigation, playback speed, volume control, sleep timer, and position persistence via SQLite.

**Architecture:** The Player struct manages a beep streamer pipeline: decoder → resample (speed) → volume → speaker. State (chapter, position, speed) is persisted to the `playback_state` table every 5 seconds. The TUI player tab renders the now-playing screen.

**Tech Stack:** gopxl/beep (MP3 decoding, resampling, effects), ebitengine/oto (audio output), database/sql

**IMPORTANT NOTE:** gopxl/beep uses CGO for the MP3 decoder (via libc). Since the project uses CGO_ENABLED=0 for SQLite, we need to handle this carefully. The player package will need CGO_ENABLED=1 for the MP3 decoder. However, for the initial implementation, we'll create the player architecture with interfaces that can work with different backends. We'll use the `github.com/gopxl/beep/mp3` decoder which requires CGO on some platforms.

**ALTERNATIVE APPROACH:** Since CGO_ENABLED=0 is a project constraint, we'll use `github.com/hajimehoshi/go-mp3` (pure Go MP3 decoder) with `github.com/gopxl/beep` for the streaming pipeline. The beep library itself is pure Go — only specific decoders may need CGO.

---

### Task 1: Player Core — Playback Engine

**Files:**
- Create: `internal/player/player.go`
- Create: `internal/player/state.go`
- Test: `internal/player/state_test.go`

- [ ] **Step 1: Create `internal/player/state.go` — playback state management**

```go
package player

import (
	"database/sql"
	"time"

	"github.com/billmal071/audbookdl/internal/db"
)

// State tracks playback position for an audiobook.
type State struct {
	AudiobookID  string
	ChapterIndex int
	PositionMS   int64
	Speed        float64
}

// SaveState persists the current playback state to SQLite.
func SaveState(database *sql.DB, s *State) error {
	return db.SavePlaybackState(database, &db.PlaybackState{
		AudiobookID:   s.AudiobookID,
		ChapterIndex:  s.ChapterIndex,
		PositionMS:    s.PositionMS,
		PlaybackSpeed: s.Speed,
	})
}

// LoadState retrieves the saved playback state for an audiobook.
// Returns nil if no state exists.
func LoadState(database *sql.DB, audiobookID string) *State {
	ps, err := db.GetPlaybackState(database, audiobookID)
	if err != nil {
		return nil
	}
	return &State{
		AudiobookID:  ps.AudiobookID,
		ChapterIndex: ps.ChapterIndex,
		PositionMS:   ps.PositionMS,
		Speed:        ps.PlaybackSpeed,
	}
}

// Playlist represents an ordered list of chapter files.
type Playlist struct {
	AudiobookID string
	Title       string
	Author      string
	Narrator    string
	Chapters    []ChapterInfo
}

// ChapterInfo holds metadata for a single chapter file.
type ChapterInfo struct {
	Index    int
	Title    string
	FilePath string
	Duration time.Duration
}
```

- [ ] **Step 2: Write state test**

Create file `internal/player/state_test.go`:

```go
package player

import (
	"path/filepath"
	"testing"

	"github.com/billmal071/audbookdl/internal/db"
)

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	database, err := db.InitWithPath(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("InitWithPath error: %v", err)
	}
	defer database.Close()

	state := &State{
		AudiobookID:  "test-123",
		ChapterIndex: 5,
		PositionMS:   123456,
		Speed:        1.5,
	}

	if err := SaveState(database, state); err != nil {
		t.Fatalf("SaveState error: %v", err)
	}

	loaded := LoadState(database, "test-123")
	if loaded == nil {
		t.Fatal("LoadState returned nil")
	}
	if loaded.ChapterIndex != 5 {
		t.Errorf("ChapterIndex = %d, want 5", loaded.ChapterIndex)
	}
	if loaded.PositionMS != 123456 {
		t.Errorf("PositionMS = %d, want 123456", loaded.PositionMS)
	}
	if loaded.Speed != 1.5 {
		t.Errorf("Speed = %f, want 1.5", loaded.Speed)
	}
}

func TestLoadState_NotFound(t *testing.T) {
	dir := t.TempDir()
	database, err := db.InitWithPath(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("InitWithPath error: %v", err)
	}
	defer database.Close()

	loaded := LoadState(database, "nonexistent")
	if loaded != nil {
		t.Error("LoadState should return nil for nonexistent")
	}
}

func TestSaveState_Upsert(t *testing.T) {
	dir := t.TempDir()
	database, err := db.InitWithPath(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("InitWithPath error: %v", err)
	}
	defer database.Close()

	SaveState(database, &State{AudiobookID: "upsert", ChapterIndex: 1, PositionMS: 1000, Speed: 1.0})
	SaveState(database, &State{AudiobookID: "upsert", ChapterIndex: 3, PositionMS: 50000, Speed: 2.0})

	loaded := LoadState(database, "upsert")
	if loaded.ChapterIndex != 3 {
		t.Errorf("ChapterIndex = %d, want 3", loaded.ChapterIndex)
	}
	if loaded.PositionMS != 50000 {
		t.Errorf("PositionMS = %d, want 50000", loaded.PositionMS)
	}
}
```

- [ ] **Step 3: Create `internal/player/player.go` — the Player struct**

```go
package player

import (
	"database/sql"
	"sync"
	"time"
)

// Status represents the player's current state.
type Status int

const (
	StatusStopped Status = iota
	StatusPlaying
	StatusPaused
)

// Player manages audio playback for an audiobook.
type Player struct {
	mu sync.RWMutex

	// Playback state
	status       Status
	playlist     *Playlist
	chapterIndex int
	positionMS   int64
	speed        float64
	volume       float64

	// Sleep timer
	sleepTimer    *time.Timer
	sleepRemainMS int64

	// Persistence
	db          *sql.DB
	saveTicker  *time.Ticker
	stopChan    chan struct{}
}

// NewPlayer creates a new Player.
func NewPlayer(database *sql.DB) *Player {
	return &Player{
		status: StatusStopped,
		speed:  1.0,
		volume: 0.8,
		db:     database,
	}
}

// Load prepares a playlist for playback. Resumes from saved state if available.
func (p *Player) Load(playlist *Playlist) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.playlist = playlist
	p.chapterIndex = 0
	p.positionMS = 0
	p.speed = 1.0

	// Try to restore saved state
	if p.db != nil {
		if state := LoadState(p.db, playlist.AudiobookID); state != nil {
			p.chapterIndex = state.ChapterIndex
			p.positionMS = state.PositionMS
			p.speed = state.Speed
		}
	}
}

// Play starts or resumes playback.
// NOTE: Actual audio output is implemented in Plan 6 integration.
// This sets up the state management framework.
func (p *Player) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.playlist == nil || len(p.playlist.Chapters) == 0 {
		return
	}

	p.status = StatusPlaying

	// Start periodic state saving
	if p.saveTicker == nil {
		p.saveTicker = time.NewTicker(5 * time.Second)
		p.stopChan = make(chan struct{})
		go p.saveLoop()
	}
}

// Pause pauses playback.
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status = StatusPaused
}

// Stop stops playback and saves state.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.status = StatusStopped

	if p.saveTicker != nil {
		p.saveTicker.Stop()
		close(p.stopChan)
		p.saveTicker = nil
	}

	// Save final state
	p.saveState()
}

// NextChapter moves to the next chapter.
func (p *Player) NextChapter() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.playlist == nil {
		return
	}
	if p.chapterIndex < len(p.playlist.Chapters)-1 {
		p.chapterIndex++
		p.positionMS = 0
	}
}

// PrevChapter moves to the previous chapter.
func (p *Player) PrevChapter() {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If more than 3 seconds into chapter, restart it. Otherwise go to previous.
	if p.positionMS > 3000 {
		p.positionMS = 0
		return
	}
	if p.chapterIndex > 0 {
		p.chapterIndex--
		p.positionMS = 0
	}
}

// SkipForward skips forward by the given duration.
func (p *Player) SkipForward(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.positionMS += d.Milliseconds()
}

// SkipBackward skips backward by the given duration.
func (p *Player) SkipBackward(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.positionMS -= d.Milliseconds()
	if p.positionMS < 0 {
		p.positionMS = 0
	}
}

// SetSpeed sets playback speed (0.5 - 3.0).
func (p *Player) SetSpeed(speed float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if speed < 0.5 {
		speed = 0.5
	}
	if speed > 3.0 {
		speed = 3.0
	}
	p.speed = speed
}

// SetVolume sets volume (0.0 - 1.0).
func (p *Player) SetVolume(vol float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if vol < 0 {
		vol = 0
	}
	if vol > 1 {
		vol = 1
	}
	p.volume = vol
}

// SetSleepTimer sets a sleep timer that pauses playback after the duration.
func (p *Player) SetSleepTimer(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sleepTimer != nil {
		p.sleepTimer.Stop()
	}

	if d <= 0 {
		p.sleepTimer = nil
		p.sleepRemainMS = 0
		return
	}

	p.sleepRemainMS = d.Milliseconds()
	p.sleepTimer = time.AfterFunc(d, func() {
		p.Pause()
		p.mu.Lock()
		p.sleepRemainMS = 0
		p.mu.Unlock()
	})
}

// GetStatus returns current playback info.
func (p *Player) GetStatus() PlayerStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	ps := PlayerStatus{
		Status:       p.status,
		Speed:        p.speed,
		Volume:       p.volume,
		SleepRemainMS: p.sleepRemainMS,
	}

	if p.playlist != nil {
		ps.AudiobookTitle = p.playlist.Title
		ps.Author = p.playlist.Author
		ps.Narrator = p.playlist.Narrator
		ps.TotalChapters = len(p.playlist.Chapters)
		ps.ChapterIndex = p.chapterIndex
		ps.PositionMS = p.positionMS

		if p.chapterIndex < len(p.playlist.Chapters) {
			ch := p.playlist.Chapters[p.chapterIndex]
			ps.ChapterTitle = ch.Title
			ps.ChapterDurationMS = ch.Duration.Milliseconds()
		}
	}

	return ps
}

// PlayerStatus is a snapshot of the player's current state (thread-safe read).
type PlayerStatus struct {
	Status           Status
	AudiobookTitle   string
	Author           string
	Narrator         string
	ChapterTitle     string
	ChapterIndex     int
	TotalChapters    int
	PositionMS       int64
	ChapterDurationMS int64
	Speed            float64
	Volume           float64
	SleepRemainMS    int64
}

func (p *Player) saveLoop() {
	for {
		select {
		case <-p.saveTicker.C:
			p.mu.RLock()
			p.saveState()
			p.mu.RUnlock()
		case <-p.stopChan:
			return
		}
	}
}

// saveState persists current state (caller must hold at least RLock).
func (p *Player) saveState() {
	if p.db == nil || p.playlist == nil {
		return
	}
	SaveState(p.db, &State{
		AudiobookID:  p.playlist.AudiobookID,
		ChapterIndex: p.chapterIndex,
		PositionMS:   p.positionMS,
		Speed:        p.speed,
	})
}
```

- [ ] **Step 4: Run tests**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go test -v ./internal/player/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/player/
git commit -m "feat: add audio player core with state management and playback controls"
```

---

### Task 2: Player Controls Unit Tests

**Files:**
- Create: `internal/player/player_test.go`

- [ ] **Step 1: Write player tests**

Create file `internal/player/player_test.go`:

```go
package player

import (
	"testing"
	"time"
)

func TestNewPlayer(t *testing.T) {
	p := NewPlayer(nil)
	if p.status != StatusStopped {
		t.Errorf("status = %d, want StatusStopped", p.status)
	}
	if p.speed != 1.0 {
		t.Errorf("speed = %f, want 1.0", p.speed)
	}
	if p.volume != 0.8 {
		t.Errorf("volume = %f, want 0.8", p.volume)
	}
}

func TestPlayer_PlayPauseStop(t *testing.T) {
	p := NewPlayer(nil)
	p.Load(&Playlist{
		AudiobookID: "test",
		Chapters:    []ChapterInfo{{Index: 1, Title: "Ch1", FilePath: "/tmp/ch1.mp3"}},
	})

	p.Play()
	s := p.GetStatus()
	if s.Status != StatusPlaying {
		t.Errorf("after Play: status = %d, want Playing", s.Status)
	}

	p.Pause()
	s = p.GetStatus()
	if s.Status != StatusPaused {
		t.Errorf("after Pause: status = %d, want Paused", s.Status)
	}

	p.Stop()
	s = p.GetStatus()
	if s.Status != StatusStopped {
		t.Errorf("after Stop: status = %d, want Stopped", s.Status)
	}
}

func TestPlayer_ChapterNavigation(t *testing.T) {
	p := NewPlayer(nil)
	p.Load(&Playlist{
		AudiobookID: "test",
		Chapters: []ChapterInfo{
			{Index: 1, Title: "Ch1"},
			{Index: 2, Title: "Ch2"},
			{Index: 3, Title: "Ch3"},
		},
	})

	s := p.GetStatus()
	if s.ChapterIndex != 0 {
		t.Errorf("initial chapter = %d, want 0", s.ChapterIndex)
	}

	p.NextChapter()
	s = p.GetStatus()
	if s.ChapterIndex != 1 {
		t.Errorf("after NextChapter: chapter = %d, want 1", s.ChapterIndex)
	}

	p.NextChapter()
	p.NextChapter() // Should not go past last
	s = p.GetStatus()
	if s.ChapterIndex != 2 {
		t.Errorf("should not go past last chapter: %d", s.ChapterIndex)
	}

	p.PrevChapter() // Position is 0, so should go to prev
	s = p.GetStatus()
	if s.ChapterIndex != 1 {
		t.Errorf("after PrevChapter: chapter = %d, want 1", s.ChapterIndex)
	}
}

func TestPlayer_PrevChapter_RestartsIfDeepInChapter(t *testing.T) {
	p := NewPlayer(nil)
	p.Load(&Playlist{
		AudiobookID: "test",
		Chapters: []ChapterInfo{
			{Index: 1, Title: "Ch1"},
			{Index: 2, Title: "Ch2"},
		},
	})

	p.NextChapter()
	p.positionMS = 5000 // 5 seconds in

	p.PrevChapter() // Should restart current chapter, not go to prev
	s := p.GetStatus()
	if s.ChapterIndex != 1 {
		t.Errorf("should stay on chapter 1, got %d", s.ChapterIndex)
	}
	if s.PositionMS != 0 {
		t.Errorf("position should be 0, got %d", s.PositionMS)
	}
}

func TestPlayer_SkipForwardBackward(t *testing.T) {
	p := NewPlayer(nil)
	p.Load(&Playlist{
		AudiobookID: "test",
		Chapters:    []ChapterInfo{{Index: 1, Title: "Ch1"}},
	})

	p.SkipForward(15 * time.Second)
	s := p.GetStatus()
	if s.PositionMS != 15000 {
		t.Errorf("after skip forward: position = %d, want 15000", s.PositionMS)
	}

	p.SkipBackward(5 * time.Second)
	s = p.GetStatus()
	if s.PositionMS != 10000 {
		t.Errorf("after skip backward: position = %d, want 10000", s.PositionMS)
	}

	p.SkipBackward(20 * time.Second) // Should clamp to 0
	s = p.GetStatus()
	if s.PositionMS != 0 {
		t.Errorf("skip backward past 0: position = %d, want 0", s.PositionMS)
	}
}

func TestPlayer_SpeedClamping(t *testing.T) {
	p := NewPlayer(nil)

	p.SetSpeed(0.1) // Below min
	if p.speed != 0.5 {
		t.Errorf("speed = %f, want 0.5 (min)", p.speed)
	}

	p.SetSpeed(5.0) // Above max
	if p.speed != 3.0 {
		t.Errorf("speed = %f, want 3.0 (max)", p.speed)
	}

	p.SetSpeed(1.5)
	if p.speed != 1.5 {
		t.Errorf("speed = %f, want 1.5", p.speed)
	}
}

func TestPlayer_VolumeClamping(t *testing.T) {
	p := NewPlayer(nil)

	p.SetVolume(-0.5) // Below min
	if p.volume != 0 {
		t.Errorf("volume = %f, want 0.0 (min)", p.volume)
	}

	p.SetVolume(1.5) // Above max
	if p.volume != 1.0 {
		t.Errorf("volume = %f, want 1.0 (max)", p.volume)
	}

	p.SetVolume(0.6)
	if p.volume != 0.6 {
		t.Errorf("volume = %f, want 0.6", p.volume)
	}
}

func TestPlayer_GetStatus(t *testing.T) {
	p := NewPlayer(nil)
	p.Load(&Playlist{
		AudiobookID: "test-book",
		Title:       "Test Book",
		Author:      "Test Author",
		Narrator:    "Test Narrator",
		Chapters: []ChapterInfo{
			{Index: 1, Title: "Chapter One", Duration: 30 * time.Minute},
			{Index: 2, Title: "Chapter Two", Duration: 25 * time.Minute},
		},
	})

	s := p.GetStatus()
	if s.AudiobookTitle != "Test Book" {
		t.Errorf("AudiobookTitle = %q", s.AudiobookTitle)
	}
	if s.TotalChapters != 2 {
		t.Errorf("TotalChapters = %d, want 2", s.TotalChapters)
	}
	if s.ChapterTitle != "Chapter One" {
		t.Errorf("ChapterTitle = %q, want 'Chapter One'", s.ChapterTitle)
	}
	if s.ChapterDurationMS != 30*60*1000 {
		t.Errorf("ChapterDurationMS = %d", s.ChapterDurationMS)
	}
}

func TestPlayer_LoadRestoresState(t *testing.T) {
	dir := t.TempDir()
	database, err := db.InitWithPath(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("InitWithPath error: %v", err)
	}
	defer database.Close()

	// Save state first
	SaveState(database, &State{
		AudiobookID:  "restore-test",
		ChapterIndex: 2,
		PositionMS:   45000,
		Speed:        1.5,
	})

	p := NewPlayer(database)
	p.Load(&Playlist{
		AudiobookID: "restore-test",
		Chapters: []ChapterInfo{
			{Index: 1, Title: "Ch1"},
			{Index: 2, Title: "Ch2"},
			{Index: 3, Title: "Ch3"},
		},
	})

	s := p.GetStatus()
	if s.ChapterIndex != 2 {
		t.Errorf("ChapterIndex = %d, want 2", s.ChapterIndex)
	}
	if s.PositionMS != 45000 {
		t.Errorf("PositionMS = %d, want 45000", s.PositionMS)
	}
	if s.Speed != 1.5 {
		t.Errorf("Speed = %f, want 1.5", s.Speed)
	}
}
```

NOTE: The last test (LoadRestoresState) needs `db` and `filepath` imports:
```go
import (
	"path/filepath"
	"testing"
	"time"

	"github.com/billmal071/audbookdl/internal/db"
)
```

- [ ] **Step 2: Run tests**

```bash
CGO_ENABLED=0 /usr/local/go/bin/go test -v ./internal/player/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/player/
git commit -m "feat: add player control tests (navigation, speed, volume, state restore)"
```

---

### Task 3: Verify Full Build and All Tests

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

```bash
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
