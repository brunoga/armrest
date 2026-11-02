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
