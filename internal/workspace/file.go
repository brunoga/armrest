package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/brunoga/sofabaton/dump"
	"github.com/brunoga/sofabaton/profile"
)

// Format is the version of the workspace files Save writes.
const Format = 1

// Ext is the extension of workspace files.
const Ext = ".armrest"

// file is a workspace file: the hub as read and the edits, as YAML (so a
// person can read them).
type file struct {
	Armrest int        `json:"armrest"`
	Saved   time.Time  `json:"saved"`
	Dump    *dump.File `json:"dump"`
	Edits   string     `json:"edits,omitempty"`
}

// MaxFile bounds the files Open reads: a hub's dump is a few hundred
// kilobytes.
const MaxFile = 64 << 20

// Save writes the workspace, edits included, as a file saved at now.
func (w *Workspace) Save(out io.Writer, now time.Time) error {
	edits, err := w.Edits()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", " ")
	return enc.Encode(file{Armrest: Format, Saved: now, Dump: w.Dump, Edits: string(edits)})
}

// Open reads a workspace file, or a hub's dump (sbx dump writes them), and
// says when it was saved (zero for a dump).
func Open(ctx context.Context, r io.Reader) (*Workspace, time.Time, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxFile+1))
	if err != nil {
		return nil, time.Time{}, err
	}
	if len(b) > MaxFile {
		return nil, time.Time{}, errors.New("the file is too big to be a hub's")
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, time.Time{}, errors.New("this is not a file Armrest or sbx saved")
	}
	if f.Armrest == 0 {
		// A dump: the same fields as its own top level.
		f = file{}
		if err := json.Unmarshal(b, &f.Dump); err != nil {
			return nil, time.Time{}, errors.New("this is not a file Armrest or sbx saved")
		}
	}
	switch {
	case f.Armrest > Format:
		return nil, time.Time{}, errors.New("this file was saved by a newer Armrest: update the app to open it")
	case f.Dump == nil || f.Dump.Info.ID == "" || len(f.Dump.Entities) == 0 && f.Dump.Devices == nil:
		return nil, time.Time{}, errors.New("this is not a file Armrest or sbx saved")
	}
	w, err := FromDump(ctx, f.Dump)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("the hub in this file cannot be read: %w", err)
	}
	if f.Edits != "" {
		if err := w.LoadEdits([]byte(f.Edits)); err != nil {
			return nil, time.Time{}, fmt.Errorf("the changes in this file cannot be read: %w", err)
		}
	}
	return w, f.Saved, nil
}

// Restore makes cfg, another hub's configuration or this hub's read
// before, the workspace's edits: applying them makes the hub hold cfg.
// Undo takes it back. Devices and activities are matched to the hub's by
// id when sameHub (a rename since is renamed back), else by name (ids are
// another hub's); those the hub lacks are new, and the hub's that cfg
// lacks are deleted.
func (w *Workspace) Restore(cfg *profile.Config, sameHub bool) error {
	c, err := cloneConfig(cfg)
	if err != nil {
		return err
	}
	devNames, actNames := map[string]uint8{}, map[string]uint8{}
	devIDs, actIDs := map[uint8]bool{}, map[uint8]bool{}
	for _, d := range w.base.Devices {
		devNames[strings.ToLower(d.Name)], devIDs[d.ID] = d.ID, true
	}
	for _, a := range w.base.Activities {
		actNames[strings.ToLower(a.Name)], actIDs[a.ID] = a.ID, true
	}
	for _, d := range c.Devices {
		if !sameHub || !devIDs[d.ID] {
			d.ID = devNames[strings.ToLower(d.Name)]
		}
	}
	for _, a := range c.Activities {
		if !sameHub || !actIDs[a.ID] {
			a.ID = actNames[strings.ToLower(a.Name)]
		}
	}
	if err := c.Validate(); err != nil {
		return err
	}
	w.commit(c, "restoring the file")
	return nil
}
