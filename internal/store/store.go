// Package store keeps local watch history and a to-watch list in JSON files
// under the user config dir. This powers the "Continue" / "Add" menu actions
// until AniList syncing is implemented.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Entry is a saved anime in the history or watchlist.
type Entry struct {
	AnimeID   int       `json:"anime_id"`
	SlugID    string    `json:"slug_id"`
	Title     string    `json:"title"`
	Cover     string    `json:"cover"`
	Episode   int       `json:"episode,omitempty"` // last watched episode
	WatchedAt time.Time `json:"watched_at"`
}

type store struct {
	path  string
	items []Entry
}

// History opens (creating if needed) the saved-history file.
func History() (*File, error) {
	return open("history.json")
}

// Watchlist opens (creating if needed) the to-watch list file.
func Watchlist() (*File, error) {
	return open("watchlist.json")
}

func open(name string) (*File, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	dir = filepath.Join(dir, "prts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := newFile(filepath.Join(dir, name))
	if err != nil {
		return nil, err
	}
	return f, nil
}

// File wraps the mutable contents of a JSON store file.
type File struct {
	path string
	// mutex would be needed for concurrent use; the TUI is single-threaded.
	items []Entry
}

func newFile(path string) (*File, error) {
	f := &File{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return f, nil
	}
	if err := json.Unmarshal(data, &f.items); err != nil {
		return nil, err
	}
	return f, nil
}

// All returns entries sorted by most recently watched/added first.
func (f *File) All() []Entry {
	items := make([]Entry, len(f.items))
	copy(items, f.items)
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].WatchedAt.After(items[j].WatchedAt)
	})
	return items
}

// Add inserts or updates an entry and persists the file.
func (f *File) Add(e Entry) error {
	e.WatchedAt = time.Now()
	found := -1
	for i, it := range f.items {
		if it.SlugID == e.SlugID {
			found = i
			break
		}
	}
	if found >= 0 {
		f.items[found] = e
	} else {
		f.items = append(f.items, e)
	}
	return f.write()
}

// MarkWatched updates the last-watched episode of an existing entry.
func (f *File) MarkWatched(slugID string, episode int) error {
	for i := range f.items {
		if f.items[i].SlugID == slugID {
			f.items[i].Episode = episode
			f.items[i].WatchedAt = time.Now()
			return f.write()
		}
	}
	return nil
}

func (f *File) write() error {
	data, err := json.MarshalIndent(f.items, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(f.path, data, 0o644)
}
