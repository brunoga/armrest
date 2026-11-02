// Package backend finds hubs and connects to them: real ones on the local
// network, and the demo hub.
package backend

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/brunoga/armrest/internal/demo"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/transport"
)

// DiscoverTime is how long Discover listens for hubs.
const DiscoverTime = 3 * time.Second

// ErrBusy reports a hub that is serving another client: it takes one at a
// time.
var ErrBusy = errors.New("the hub is busy with another app, or this is not a hub: close the Sofabaton app (or sbx) and try again")

// Hub is a hub the app can connect to.
type Hub struct {
	Name     string
	Model    string
	Firmware int
	Addr     netip.Addr
	// Demo is the simulated hub, which has no address.
	Demo bool
}

// DemoHub is the simulated hub.
var DemoHub = Hub{Name: "Demo hub", Model: "X2", Demo: true}

func (h Hub) String() string {
	if h.Demo {
		return h.Name
	}
	return fmt.Sprintf("%s (%s, %v)", h.Name, h.Model, h.Addr)
}

// Backend finds and connects to hubs. Its functions are fields so tests
// can replace them.
type Backend struct {
	// Discover lists the hubs that answer on the local network. Hubs
	// serving another client do not answer.
	Discover func(context.Context) ([]Hub, error)
	// Dial connects to the hub at an address.
	Dial func(context.Context, netip.Addr) (*hub.Client, error)
}

// Network is the backend for the local network.
func Network() Backend {
	return Backend{
		Discover: func(ctx context.Context) ([]Hub, error) {
			ctx, cancel := context.WithTimeout(ctx, DiscoverTime)
			defer cancel()
			var d transport.Discoverer
			found, err := d.Discover(ctx)
			var hubs []Hub
			for _, a := range found {
				hubs = append(hubs, Hub{Name: a.Info.Name, Model: a.Info.Variant.String(), Firmware: int(a.Info.Firmware), Addr: a.Addr})
			}
			return hubs, err
		},
		Dial: func(ctx context.Context, addr netip.Addr) (*hub.Client, error) {
			return hub.Dial(ctx, addr, hub.Options{})
		},
	}
}

// Connect opens a client of h.
func (b Backend) Connect(ctx context.Context, h Hub) (*hub.Client, error) {
	if h.Demo {
		return demo.Open(ctx)
	}
	c, err := b.Dial(ctx, h.Addr)
	if errors.Is(err, transport.ErrRefused) {
		return nil, ErrBusy
	}
	return c, err
}
