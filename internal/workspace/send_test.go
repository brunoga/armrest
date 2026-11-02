package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/brunoga/armrest/internal/demo"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/hubsim"
)

func TestSend(t *testing.T) {
	ctx := context.Background()
	d, err := demo.Dump()
	if err != nil {
		t.Fatal(err)
	}
	m, err := d.Model()
	if err != nil {
		t.Fatal(err)
	}
	c, err := hub.Open(ctx, hubsim.Pipe(hubsim.FromModel(m)), hub.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	w, err := Read(ctx, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Send(ctx, c, "Receiver", "Mute"); err != nil {
		t.Fatal(err)
	}
	if len(m.Runs) != 1 || len(m.Runs[0].Steps) != 1 || m.Runs[0].Steps[0].Device != w.Config.Device("Receiver").ID {
		t.Errorf("runs: %+v", m.Runs)
	}
	if err := w.Send(ctx, c, "Receiver", "No Such"); !errors.Is(err, ErrNotOnHub) {
		t.Errorf("unknown command: %v", err)
	}
	box, _ := Templates()[0].Device("New Box")
	_ = w.AddDevice(box)
	if err := w.Send(ctx, c, "New Box", box.Commands[0].Name); !errors.Is(err, ErrNotOnHub) {
		t.Errorf("new device: %v", err)
	}
}
