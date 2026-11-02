// Package cache keeps the last dump of each hub on the device, so the app
// can show a hub at once and check it for changes in the background.
package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"fyne.io/fyne/v2"

	"github.com/brunoga/sofabaton/dump"
)

// Entry is a hub's cached dump and when it was read.
type Entry struct {
	Saved time.Time  `json:"saved"`
	Dump  *dump.File `json:"dump"`
}

// Cache stores entries in an app's storage, one file per hub id.
type Cache struct {
	Storage fyne.Storage
}

func name(hubID string) string { return "hub-" + hubID + ".json" }

// Load is the hub's entry, or nil if there is none or it cannot be read
// (a cache is only ever a shortcut).
func (c Cache) Load(hubID string) *Entry {
	r, err := c.Storage.Open(name(hubID))
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
	if d == nil || d.Info.ID == "" {
		return errors.New("cache: a dump without a hub id")
	}
	// Save overwrites, Create makes a new file; neither does both.
	w, err := c.Storage.Save(name(d.Info.ID))
	if err != nil {
		w, err = c.Storage.Create(name(d.Info.ID))
	}
	if err != nil {
		return fmt.Errorf("cache: %w", err)
	}
	if err := json.NewEncoder(w).Encode(Entry{Saved: now, Dump: d}); err != nil {
		_ = w.Close()
		return fmt.Errorf("cache: %w", err)
	}
	return w.Close()
}

// Forget removes the hub's entry.
func (c Cache) Forget(hubID string) error {
	return c.Storage.Remove(name(hubID))
}
