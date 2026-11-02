package workspace

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/brunoga/armrest/internal/demo"
)

// A saved workspace opens with its edits; a hub's dump opens as it is.
func TestSaveOpen(t *testing.T) {
	ctx := context.Background()
	w := demoWorkspace(t)
	if err := w.RemoveActivity("Play PC"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	var b bytes.Buffer
	if err := w.Save(&b, now); err != nil {
		t.Fatal(err)
	}
	o, saved, err := Open(ctx, bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Equal(now) || !o.Same(w) || !o.Changed() || o.Config.Activity("Play PC") != nil {
		t.Fatal("the edits did not come back")
	}

	d, _ := demo.Dump()
	b.Reset()
	if err := d.Write(&b); err != nil {
		t.Fatal(err)
	}
	o, saved, err = Open(ctx, &b)
	if err != nil || !saved.IsZero() || o.Changed() || len(o.Config.Devices) != 3 {
		t.Fatalf("a dump: %v", err)
	}

	for _, bad := range []string{"", "hello", "{}", `{"armrest": 1}`, `{"armrest": 99, "dump": {}}`} {
		if _, _, err := Open(ctx, strings.NewReader(bad)); err == nil {
			t.Errorf("opened %q", bad)
		}
	}
}

// A file restored onto a hub is matched to it by name, whatever its ids:
// applying makes the hub hold the file.
func TestRestore(t *testing.T) {
	ctx := context.Background()
	file := demoWorkspace(t)
	if err := file.RemoveActivity("Play PC"); err != nil {
		t.Fatal(err)
	}
	e, _ := file.EditDevice("Receiver")
	e.Brand = "Marantz"
	if err := file.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	// Another hub's ids.
	for _, d := range file.Config.Devices {
		d.ID += 50
	}
	for _, a := range file.Config.Activities {
		a.ID += 50
	}

	w := demoWorkspace(t)
	if err := w.Restore(file.Config, false); err != nil {
		t.Fatal(err)
	}
	if w.DeviceState(w.Config.Device("Receiver")) != Edited || w.DeviceState(w.Config.Device("3D Player")) != Unchanged {
		t.Fatal("not matched by name")
	}
	p, err := w.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(p); got != "Change device Receiver\nDelete activity Play PC" && got != "Delete activity Play PC\nChange device Receiver" {
		t.Fatalf("plan:\n%s", got)
	}
	if w.Undo() != "restoring the file" || w.Changed() {
		t.Fatal("undo")
	}
}

// Restored onto its own hub, a file keeps its ids: what was renamed on
// the hub since is renamed back.
func TestRestoreSameHub(t *testing.T) {
	file := demoWorkspace(t)
	w := demoWorkspace(t)
	e, _ := w.EditDevice("Receiver")
	e.Name = "AVR"
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	// As if the hub had been changed so, and read again.
	w.base = w.Config
	w.undo = nil
	if err := w.Restore(file.Config, true); err != nil {
		t.Fatal(err)
	}
	d := w.Config.Device("Receiver")
	if d == nil || w.DeviceState(d) != Edited || len(w.Config.Devices) != 3 {
		t.Fatalf("devices %+v", w.Config.Devices)
	}
}
