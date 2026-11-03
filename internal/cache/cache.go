// Package cache keeps the last dump of each hub on the device, so the app
// can show a hub at once and check it for changes in the background.
package cache

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brunoga/sofabaton/dump"
	"github.com/brunoga/sofabaton/irsource"
)

// Entry is a hub's cached dump, when it was read, and the user's edits
// not yet applied to it (workspace.Edits), if any.
type Entry struct {
	Saved time.Time  `json:"saved"`
	Dump  *dump.File `json:"dump"`
	Edits []byte     `json:"edits,omitempty"`
}

// Cache stores entries in a directory (the app's storage), one file per
// hub id.
type Cache struct {
	Dir string
}

func (c Cache) path(hubID string) string {
	return filepath.Join(c.Dir, "hub-"+hubID+".json")
}

// Load is the hub's entry, or nil if there is none or it cannot be read
// (a cache is only ever a shortcut).
func (c Cache) Load(hubID string) *Entry {
	if !validID(hubID) {
		return nil
	}
	r, err := os.Open(c.path(hubID))
	if err != nil {
		return nil
	}
	defer func() { _ = r.Close() }()
	var e Entry
	if err := json.NewDecoder(io.LimitReader(r, 64<<20)).Decode(&e); err != nil || e.Dump == nil || e.Dump.Info.ID != hubID {
		return nil
	}
	return &e
}

// Save stores e as its hub's entry.
func (c Cache) Save(e Entry) error {
	d := e.Dump
	if d == nil || !validID(d.Info.ID) {
		return errors.New("cache: a dump without a hub id")
	}
	return c.write(c.path(d.Info.ID), e)
}

// write stores v as JSON at path, written aside and renamed, so a crash
// never leaves half a file.
func (c Cache) write(path string, v any) error {
	if err := os.MkdirAll(c.Dir, 0o777); err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	tmp, err := os.CreateTemp(c.Dir, "cache-*.tmp")
	if err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := json.NewEncoder(tmp).Encode(v); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	return nil
}

// Index is an online source's index of IR code files, and when it was
// fetched.
type Index struct {
	Saved   time.Time        `json:"saved"`
	Entries []irsource.Entry `json:"entries"`
}

func (c Cache) indexPath(s irsource.Source) string {
	return filepath.Join(c.Dir, "index-"+strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '_'
	}, string(s))+".json")
}

// LoadIndex is the source's index, or nil if there is none or it cannot
// be read.
func (c Cache) LoadIndex(s irsource.Source) *Index {
	r, err := os.Open(c.indexPath(s))
	if err != nil {
		return nil
	}
	defer func() { _ = r.Close() }()
	var ix Index
	if err := json.NewDecoder(io.LimitReader(r, 64<<20)).Decode(&ix); err != nil {
		return nil
	}
	return &ix
}

// SaveIndex stores the source's index.
func (c Cache) SaveIndex(s irsource.Source, ix Index) error {
	return c.write(c.indexPath(s), ix)
}

// Forget removes the hub's entry.
func (c Cache) Forget(hubID string) error {
	if !validID(hubID) {
		return errors.New("cache: not a hub id")
	}
	return os.Remove(c.path(hubID))
}

// validID reports whether id is a hub id (hex), so it is safe in a file
// name.
func validID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
