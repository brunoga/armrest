//go:build live

package workspace

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/sofabaton/profile"
)

// TestLive reads each hub on the network into a workspace. It only reads.
func TestLive(t *testing.T) {
	ctx := context.Background()
	b := backend.Network()
	hubs, err := b.Discover(ctx)
	if err != nil || len(hubs) == 0 {
		t.Fatalf("no hub answered (%v)", err)
	}
	for _, h := range hubs {
		c, err := b.Connect(ctx, h)
		if err != nil {
			t.Fatalf("%v: %v", h, err)
		}
		start := time.Now()
		w, err := Read(ctx, c, nil)
		if err != nil {
			_ = c.Close()
			t.Fatalf("%v: %v", h, err)
		}
		t.Logf("%v: %d devices, %d activities, in %v", h, len(w.Config.Devices), len(w.Config.Activities), time.Since(start).Round(time.Second))
		start = time.Now()
		again, err := Refresh(ctx, c, w.Dump, nil)
		_ = c.Close()
		if err != nil {
			t.Fatalf("%v: refresh: %v", h, err)
		}
		if !again.Same(w) {
			t.Errorf("%v: the refresh differs", h)
		}
		t.Logf("%v: refreshed in %v", h, time.Since(start).Round(time.Second))
	}
}

// TestLiveAddDevice adds a ZZ-test device from a template through the
// workspace, then removes it, checking each plan is only that. It writes
// to the hub, but touches nothing else.
func TestLiveAddDevice(t *testing.T) {
	ctx := context.Background()
	b := backend.Network()
	hubs, err := b.Discover(ctx)
	if err != nil || len(hubs) != 1 {
		t.Fatalf("want one hub, got %d (%v)", len(hubs), err)
	}
	c, err := b.Connect(ctx, hubs[0])
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	w, err := Read(ctx, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Whatever happens, the hub keeps no ZZ-test device.
	defer func() {
		devs, _ := c.Devices(ctx)
		for _, e := range devs {
			if strings.HasPrefix(e.Name, "ZZ-test") {
				t.Logf("removing %q", e.Name)
				_ = c.DeleteDevice(ctx, e.ID)
			}
		}
	}()
	var ugoos *Template
	for _, x := range Templates() {
		if x.Name == "ir/ugoos/am6b-plus" {
			ugoos = &x
		}
	}
	d, err := ugoos.Device("ZZ-test Box")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.AddDevice(d); err != nil {
		t.Fatal(err)
	}
	// apply applies w's edits, checking the plan is just want, and reads
	// the hub back: the device named name must be as w had it.
	apply := func(w *Workspace, want, name string) *Workspace {
		t.Helper()
		p, err := w.Plan(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Changes) != 1 || p.Changes[0].Title != want {
			t.Fatalf("plan is not just %q:\n%+v", want, p.Changes)
		}
		start := time.Now()
		if err := w.Apply(ctx, c, p.Writes, nil); err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d writes in %v", want, p.Writes, time.Since(start).Round(time.Second))
		back, err := w.ReadBack(ctx, c, nil)
		if err != nil {
			t.Fatal(err)
		}
		if name != "" {
			wantD, gotD := *w.Config.Device(name), back.Config.Device(name)
			if gotD == nil {
				t.Fatalf("%s is not on the hub", name)
			}
			g := *gotD
			// The hub keeps no comments, and picks an icon and a color
			// when the device has none.
			wantD.ID, g.ID, wantD.From = 0, 0, ""
			if wantD.Icon == 0 {
				g.Icon = 0
			}
			if wantD.Color == 0 {
				g.Color = 0
			}
			wantD.Commands = append([]profile.Command(nil), wantD.Commands...)
			for i := range wantD.Commands {
				wantD.Commands[i].Comment = ""
			}
			if !sameYAML(&wantD, &g) {
				a, _ := yaml.Marshal(&wantD)
				b, _ := yaml.Marshal(&g)
				t.Fatalf("%s on the hub:\n%s\nwant:\n%s", name, b, a)
			}
		}
		if p, err := back.Plan(ctx); err != nil || len(p.Changes) != 0 {
			t.Fatalf("after %q: %v %+v", want, err, p)
		}
		return back
	}
	w = apply(w, "Add device ZZ-test Box", "ZZ-test Box")
	if w.Config.Device("ZZ-test Box") == nil {
		t.Fatal("not on the hub")
	}

	// Edit it as the editor does: rename it and a command, recode
	// another, and put a command on a key.
	e, err := w.EditDevice("ZZ-test Box")
	if err != nil {
		t.Fatal(err)
	}
	e.Name = "ZZ-test Player"
	// The editor's pickers follow a rename, so the keys do too.
	old := e.Commands[1].Name
	e.Commands[1].Name = "Cursor Up"
	for k, v := range e.Keys {
		if v.Short == old {
			e.Keys[k] = profile.Key{Short: "Cursor Up", Long: v.Long}
		}
	}
	e.Commands[2].Code = "0x7F"
	e.Keys["Red"] = profile.Key{Short: e.Commands[3].Name}
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	w = apply(w, "Change device ZZ-test Player", "ZZ-test Player")
	d = w.Config.Device("ZZ-test Player")
	if d == nil || d.Commands[1].Name != "Cursor Up" || d.Commands[2].Code == nil || d.Commands[2].Code.Value != 0x7F || d.Buttons["Red"].Short != e.Commands[3].Name {
		t.Fatalf("edit not on the hub: %+v", d)
	}

	if err := w.RemoveDevice("ZZ-test Player"); err != nil {
		t.Fatal(err)
	}
	w = apply(w, "Delete device ZZ-test Player", "")
	if w.Config.Device("ZZ-test Player") != nil {
		t.Fatal("still on the hub")
	}
}
