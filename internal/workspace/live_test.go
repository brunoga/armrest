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
