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

// Discard drops the edits.
func (w *Workspace) Discard(ctx context.Context) error {
	o, err := FromDump(ctx, w.Dump)
	if err != nil {
		return err
	}
	w.Config = o.Config
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
	d.ID = 0
	w.Config.Devices = append(w.Config.Devices, d)
	return nil
}
