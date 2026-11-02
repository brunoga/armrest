package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/brunoga/armrest/internal/demo"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/hubsim"
	"github.com/brunoga/sofabaton/wire"
)

func TestRead(t *testing.T) {
	ctx := context.Background()
	c, err := demo.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var steps int
	w, err := Read(ctx, c, func(done, total int, _ string) {
		steps++
		if total != 7 || done > total {
			t.Errorf("progress %d of %d", done, total)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if steps != 8 {
		t.Errorf("%d progress calls, want 8", steps)
	}
	if len(w.Config.Devices) != 3 || len(w.Config.Activities) != 4 {
		t.Errorf("%d devices and %d activities, want 3 and 4", len(w.Config.Devices), len(w.Config.Activities))
	}
	if a := w.Config.Activity("Watch Google TV"); a == nil || len(a.Devices) == 0 {
		t.Errorf("Watch Google TV: %+v", a)
	}

	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Read(cctx, c, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want cancelled", err)
	}
}

// A read that does not hold together, because the hub once said a device
// had no commands, is read again.
func TestReadAgain(t *testing.T) {
	ctx := context.Background()
	d, err := demo.Dump()
	if err != nil {
		t.Fatal(err)
	}
	m, err := d.Model()
	if err != nil {
		t.Fatal(err)
	}
	h := hubsim.FromModel(m)
	commands := h.Handler(wire.FamReqCommands)
	var glitches int
	h.Handle(wire.FamReqCommands, func(req wire.Frame) []wire.Frame {
		if req.Payload[0] == 2 && glitches == 0 {
			glitches++
			return []wire.Frame{hubsim.Status(hubsim.NotFound)}
		}
		return commands(req)
	})
	c, err := hub.Open(ctx, hubsim.Pipe(h), hub.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var again bool
	w, err := Read(ctx, c, func(_, _ int, name string) { again = again || name == Again })
	if err != nil {
		t.Fatal(err)
	}
	if !again || len(w.Config.Devices) != 3 {
		t.Errorf("again %v, %d devices", again, len(w.Config.Devices))
	}
}

// Edits are undone and redone in order; a new edit drops what was undone.
func TestUndo(t *testing.T) {
	w := demoWorkspace(t)
	if w.CanUndo() || w.CanRedo() || w.Undo() != "" || w.Redo() != "" {
		t.Fatal("history before any edit")
	}
	if err := w.RemoveActivity("Play PC"); err != nil {
		t.Fatal(err)
	}
	e, _ := w.EditDevice("Receiver")
	e.Brand = "Marantz"
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	// An edit that changes nothing is not one.
	e, _ = w.EditDevice("Receiver")
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	if got := w.Undo(); got != "changes to Receiver" || w.Config.Device("Receiver").Brand != "Denon" || w.Config.Activity("Play PC") != nil {
		t.Fatalf("undid %q", got)
	}
	if got := w.Undo(); got != "deleting Play PC" || w.Config.Activity("Play PC") == nil || w.Changed() || w.CanUndo() {
		t.Fatalf("undid %q", got)
	}
	if got := w.Redo(); got != "deleting Play PC" || w.Config.Activity("Play PC") != nil || !w.CanRedo() {
		t.Fatalf("redid %q", got)
	}
	if err := w.Discard(); err != nil || w.Changed() || w.CanRedo() {
		t.Fatalf("discard: %v", err)
	}
	if got := w.Undo(); got != "discarding the changes" || w.Config.Activity("Play PC") != nil {
		t.Fatalf("undid %q", got)
	}
}

// Edits kept as bytes come back as they were.
func TestEdits(t *testing.T) {
	w := demoWorkspace(t)
	if b, err := w.Edits(); b != nil || err != nil {
		t.Fatalf("edits before any: %q %v", b, err)
	}
	e, _ := w.EditDevice("Receiver")
	e.Name = "AVR"
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	b, err := w.Edits()
	if err != nil {
		t.Fatal(err)
	}
	o := demoWorkspace(t)
	if err := o.LoadEdits(b); err != nil {
		t.Fatal(err)
	}
	if !o.Same(w) || o.Config.Device("AVR") == nil || o.DeviceState(o.Config.Device("AVR")) != Edited || o.CanUndo() {
		t.Fatal("the edits did not come back")
	}
	if err := o.LoadEdits([]byte("activities: [{name: X, devices: [Nothing]}]\n")); err == nil {
		t.Error("edits that do not hold together loaded")
	}
}
