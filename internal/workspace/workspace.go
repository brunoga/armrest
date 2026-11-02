// Package workspace is a hub's configuration as the app holds it: read
// once from the hub, then looked at and edited without it.
//
// Reading keeps the hub's lossless dump, and takes the configuration from
// a simulated copy of the hub made from it: that is faster than reading
// the configuration from the hub itself, and the copy is where the app
// can try changes before writing them.
package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/brunoga/sofabaton/dump"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/profile"
)

// Tool names the app in the dumps it makes.
const Tool = "armrest"

// Workspace is a hub's configuration.
type Workspace struct {
	// Dump is the hub as read.
	Dump *dump.File
	// Config is its configuration, as profiles, with the user's edits.
	Config *profile.Config
	// base is the configuration as read.
	base *profile.Config
	// undo and redo are the configurations before the edits undone by
	// Undo and after those Redo makes again, the latest last.
	undo, redo []step
}

// step is a configuration in the edit history, and the edit that left it,
// in words.
type step struct {
	cfg  *profile.Config
	what string
}

// History is how many edits Undo can take back.
const History = 100

// commit makes cfg the configuration, after the edit what ("changes to
// TV"), which Undo can take back. An edit that changes nothing is not
// one.
func (w *Workspace) commit(cfg *profile.Config, what string) {
	if sameConfig(cfg, w.Config) {
		return
	}
	w.undo = append(w.undo, step{w.Config, what})
	if len(w.undo) > History {
		w.undo = w.undo[1:]
	}
	w.redo = nil
	w.Config = cfg
}

// CanUndo and CanRedo report whether Undo and Redo have an edit to take
// back or make again.
func (w *Workspace) CanUndo() bool { return len(w.undo) > 0 }
func (w *Workspace) CanRedo() bool { return len(w.redo) > 0 }

// Undo takes back the last edit, and says which it was.
func (w *Workspace) Undo() string {
	if len(w.undo) == 0 {
		return ""
	}
	last := w.undo[len(w.undo)-1]
	w.undo = w.undo[:len(w.undo)-1]
	w.redo = append(w.redo, step{w.Config, last.what})
	w.Config = last.cfg
	return last.what
}

// Redo makes the last edit Undo took back again, and says which it was.
func (w *Workspace) Redo() string {
	if len(w.redo) == 0 {
		return ""
	}
	next := w.redo[len(w.redo)-1]
	w.redo = w.redo[:len(w.redo)-1]
	w.undo = append(w.undo, step{w.Config, next.what})
	w.Config = next.cfg
	return next.what
}

// KeepEdits gives w the edits of old, a workspace of the hub read before,
// with their history.
func (w *Workspace) KeepEdits(old *Workspace) {
	w.Config, w.undo, w.redo = old.Config, old.undo, old.redo
}

// Edits are the edits as bytes to keep, or nil if there are none.
func (w *Workspace) Edits() ([]byte, error) {
	if !w.Changed() {
		return nil, nil
	}
	return yaml.Marshal(w.Config)
}

// LoadEdits makes edits, from Edits, the configuration.
func (w *Workspace) LoadEdits(edits []byte) error {
	var cfg profile.Config
	if err := yaml.Unmarshal(edits, &cfg); err != nil {
		return fmt.Errorf("workspace: the edits: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("workspace: the edits: %w", err)
	}
	w.Config, w.undo, w.redo = &cfg, nil, nil
	return nil
}

// Attempts is how many times Read reads the hub: now and then the X2
// answers a request wrongly (once, on ours, that a device had no
// commands), and the dump it makes then does not hold together.
const Attempts = 2

// Read reads the hub c is connected to. progress, if not nil, gets how
// many of the hub's devices and activities are read, of how many, and the
// one being read; the name is Again when a read that did not hold
// together starts over. ctx cancels it between them.
func Read(ctx context.Context, c *hub.Client, progress func(done, total int, name string)) (*Workspace, error) {
	return Refresh(ctx, c, nil, progress)
}

// Refresh is Read for a hub read before into old: the commands whose
// record did not change keep old's signals, which makes it about twice as
// fast. The hub keeps a command's record when its signal is rewritten, so
// old must hold only signals known to be current (see ReadBack); a nil old
// reads everything.
func Refresh(ctx context.Context, c *hub.Client, old *dump.File, progress func(done, total int, name string)) (*Workspace, error) {
	var err error
	for try := 0; try < Attempts; try++ {
		if try > 0 && progress != nil {
			progress(0, 1, Again)
		}
		var d *dump.File
		if d, err = dump.Refresh(ctx, c, old, Tool, nil, progress); err != nil {
			return nil, err
		}
		var w *Workspace
		if w, err = FromDump(ctx, d); err == nil {
			return w, nil
		}
	}
	return nil, err
}

// Again is the name Read's progress gets when it starts over.
const Again = "(again)"

// ReadBack reads the hub after the workspace's edits were applied to it:
// the signals of the devices the edits changed are read again, the others'
// kept.
func (w *Workspace) ReadBack(ctx context.Context, c *hub.Client, progress func(done, total int, name string)) (*Workspace, error) {
	stale := map[byte]bool{}
	for _, d := range w.Config.Devices {
		if w.DeviceState(d) == Edited {
			stale[d.ID] = true
		}
	}
	old := *w.Dump
	old.Entities = nil
	for _, e := range w.Dump.Entities {
		if stale[e.ID] {
			cp := *e
			cp.Data = nil
			e = &cp
		}
		old.Entities = append(old.Entities, e)
	}
	return Refresh(ctx, c, &old, progress)
}

// Same reports whether w and o hold the same configuration. The hub
// changes some bytes on its own (a device's power state), so dumps can
// differ where configurations do not.
func (w *Workspace) Same(o *Workspace) bool {
	return sameConfig(w.Config, o.Config)
}

// SameBase reports whether w and o were read from the same configuration,
// whatever their edits.
func (w *Workspace) SameBase(o *Workspace) bool {
	return sameConfig(w.base, o.base)
}

func sameConfig(a, b *profile.Config) bool { return sameYAML(a, b) }

func sameYAML(a, b any) bool {
	x, err1 := yaml.Marshal(a)
	y, err2 := yaml.Marshal(b)
	return err1 == nil && err2 == nil && bytes.Equal(x, y)
}

// FromDump is the workspace of a dump.
func FromDump(ctx context.Context, d *dump.File) (*Workspace, error) {
	sim, err := d.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}
	defer func() { _ = sim.Close() }()
	// Two snapshots: one to edit, one to compare the edits with.
	var cfgs [2]*profile.Config
	for i := range cfgs {
		if cfgs[i], err = profile.Snapshot(ctx, sim, nil); err != nil {
			return nil, fmt.Errorf("workspace: %w", err)
		}
	}
	return &Workspace{Dump: d, Config: cfgs[0], base: cfgs[1]}, nil
}

// Changed reports whether the configuration has edits.
func (w *Workspace) Changed() bool {
	return !sameConfig(w.Config, w.base)
}

// Discard drops the edits; Undo takes it back.
func (w *Workspace) Discard() error {
	cfg, err := cloneConfig(w.base)
	if err != nil {
		return err
	}
	w.commit(cfg, "discarding the changes")
	return nil
}

// State is how a device or activity differs from the hub.
type State int

const (
	Unchanged State = iota
	New
	Edited
)

// DeviceState is how d differs from the hub's.
func (w *Workspace) DeviceState(d *profile.Device) State {
	if d.ID == 0 {
		return New
	}
	for _, b := range w.base.Devices {
		if b.ID == d.ID {
			if sameYAML(b, d) {
				return Unchanged
			}
			return Edited
		}
	}
	return New
}

// ActivityState is how a differs from the hub's.
func (w *Workspace) ActivityState(a *profile.Activity) State {
	if a.ID == 0 {
		return New
	}
	for _, b := range w.base.Activities {
		if b.ID == a.ID {
			if sameYAML(b, a) {
				return Unchanged
			}
			return Edited
		}
	}
	return New
}

// AddDevice adds a new device. Its name must be one no device has.
func (w *Workspace) AddDevice(d *profile.Device) error {
	if strings.TrimSpace(d.Name) == "" {
		return errors.New("the device needs a name")
	}
	if o := w.Config.Device(d.Name); o != nil {
		return fmt.Errorf("there is a device named %q already", o.Name)
	}
	cfg, err := cloneConfig(w.Config)
	if err != nil {
		return err
	}
	d.ID = 0
	cfg.Devices = append(cfg.Devices, d)
	w.commit(cfg, "adding "+d.Name)
	return nil
}
