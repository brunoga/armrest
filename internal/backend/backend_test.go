package backend

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/transport"
)

func TestBusy(t *testing.T) {
	b := Backend{Dial: func(context.Context, netip.Addr) (*hub.Client, error) {
		return nil, transport.ErrRefused
	}}
	if _, err := b.Connect(context.Background(), Hub{Addr: netip.MustParseAddr("192.168.1.20")}); !errors.Is(err, ErrBusy) {
		t.Errorf("got %v, want ErrBusy", err)
	}
}

func TestDemo(t *testing.T) {
	c, err := Backend{}.Connect(context.Background(), DemoHub)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	devs, err := c.Devices(context.Background())
	if err != nil || len(devs) != 3 {
		t.Errorf("got %d devices, %v; want the demo's 3", len(devs), err)
	}
}
