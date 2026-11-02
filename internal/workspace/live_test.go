//go:build live

package workspace

import (
	"context"
	"testing"
	"time"

	"github.com/brunoga/armrest/internal/backend"
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
	apply := func(w *Workspace, want string) *Workspace {
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
		again, err := Refresh(ctx, c, w.Dump, nil)
		if err != nil {
			t.Fatal(err)
		}
		if p, err := again.Plan(ctx); err != nil || len(p.Changes) != 0 {
			t.Fatalf("after %q: %v %+v", want, err, p)
		}
		return again
	}
	w = apply(w, "Add device ZZ-test Box")
	if w.Config.Device("ZZ-test Box") == nil {
		t.Fatal("not on the hub")
	}
	if err := w.Config.RemoveDevice("ZZ-test Box"); err != nil {
		t.Fatal(err)
	}
	w = apply(w, "Delete device ZZ-test Box")
	if w.Config.Device("ZZ-test Box") != nil {
		t.Fatal("still on the hub")
	}
}
