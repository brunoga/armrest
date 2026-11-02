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
	"fmt"

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
	// Config is its configuration, as profiles.
	Config *profile.Config
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

// Refresh is Read for a hub read before into old (a cached dump): the
// commands that did not change keep old's signals, which makes it about
// twice as fast. A nil old reads everything.
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

// Same reports whether w and o hold the same configuration. The hub
// changes some bytes on its own (a device's power state), so dumps can
// differ where configurations do not.
func (w *Workspace) Same(o *Workspace) bool {
	a, err1 := yaml.Marshal(w.Config)
	b, err2 := yaml.Marshal(o.Config)
	return err1 == nil && err2 == nil && bytes.Equal(a, b)
}

// FromDump is the workspace of a dump.
func FromDump(ctx context.Context, d *dump.File) (*Workspace, error) {
	sim, err := d.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}
	defer func() { _ = sim.Close() }()
	cfg, err := profile.Snapshot(ctx, sim, nil)
	if err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}
	return &Workspace{Dump: d, Config: cfg}, nil
}
