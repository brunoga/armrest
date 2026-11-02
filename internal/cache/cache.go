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
	"time"

	"github.com/brunoga/sofabaton/dump"
)

// Entry is a hub's cached dump and when it was read.
type Entry struct {
	Saved time.Time  `json:"saved"`
	Dump  *dump.File `json:"dump"`
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

// Save stores d as the hub's entry, read now.
func (c Cache) Save(d *dump.File, now time.Time) error {
	if d == nil || !validID(d.Info.ID) {
		return errors.New("cache: a dump without a hub id")
	}
	if err := os.MkdirAll(c.Dir, 0o777); err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	// Written aside and renamed, so a crash never leaves half an entry.
	tmp, err := os.CreateTemp(c.Dir, "hub-*.tmp")
	if err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if err := json.NewEncoder(tmp).Encode(Entry{Saved: now, Dump: d}); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	if err := os.Rename(tmp.Name(), c.path(d.Info.ID)); err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	return nil
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
