//go:build live

package backend

import (
	"context"
	"testing"
)

// TestLive finds the hubs on the network and reads each one's devices.
// Run with make test-live; it only reads.
func TestLive(t *testing.T) {
	ctx := context.Background()
	b := Network()
	hubs, err := b.Discover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hubs) == 0 {
		t.Fatal("no hub answered")
	}
	for _, h := range hubs {
		c, err := b.Connect(ctx, h)
		if err != nil {
			t.Fatalf("%v: %v", h, err)
		}
		devs, err := c.Devices(ctx)
		_ = c.Close()
		if err != nil {
			t.Fatalf("%v: %v", h, err)
		}
		t.Logf("%v: %d devices", h, len(devs))
	}
}
