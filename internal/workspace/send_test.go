package workspace

import (
	"context"
	"errors"
	"testing"

	"github.com/brunoga/armrest/internal/demo"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/hubsim"
	"github.com/brunoga/sofabaton/record"
	"github.com/brunoga/sofabaton/wire"
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

// Keys are pressed and macros run as the hub has them.
func TestPressAndRunMacro(t *testing.T) {
	ctx := context.Background()
	d, err := demo.Dump()
	if err != nil {
		t.Fatal(err)
	}
	m, err := d.Model()
	if err != nil {
		t.Fatal(err)
	}
	// Watch Google TV gets a macro, on the Red key.
	act := m.Activities[0]
	quiet, err := record.NewMacro(wire.X2, act.ID, 5, "Quiet")
	if err != nil {
		t.Fatal(err)
	}
	mute := m.Commands[m.Devices[0].ID][0]
	quiet.Steps = []record.Step{hub.StepFor(mute), {Command: 0xFF, Delay: 2}, hub.StepFor(mute)}
	m.Macros[act.ID] = append(m.Macros[act.ID], quiet)
	m.Keymaps[act.ID] = append(m.Keymaps[act.ID], record.Binding{Entity: act.ID, Button: record.ButtonRed,
		Short: record.Press{Device: act.ID, Command: 5}})
	c, err := hub.Open(ctx, hubsim.Pipe(hubsim.FromModel(m)), hub.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	w, err := Read(ctx, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	name := w.Config.Activities[0].Name

	if err := w.Press(ctx, c, name, "Vol+", false); err != nil {
		t.Fatal(err)
	}
	if len(m.Runs) != 1 || m.Runs[0].Steps[0].Device != w.Config.Device("Receiver").ID {
		t.Errorf("Vol+ ran %+v", m.Runs)
	}
	if err := w.Press(ctx, c, name, "Vol+", true); !errors.Is(err, ErrNoKey) {
		t.Errorf("holding Vol+: %v", err)
	}
	if err := w.Press(ctx, c, name, "Blue", false); !errors.Is(err, ErrNoKey) {
		t.Errorf("an unset key: %v", err)
	}
	if err := w.Press(ctx, c, name, "Red", false); err != nil {
		t.Fatal(err)
	}
	if err := w.RunMacro(ctx, c, name, "quiet"); err != nil {
		t.Fatal(err)
	}
	if len(m.Runs) != 3 || len(m.Runs[1].Steps) != 3 || len(m.Runs[2].Steps) != 3 {
		t.Errorf("the macro ran %+v", m.Runs)
	}
	if err := w.RunMacro(ctx, c, name, "Loud"); !errors.Is(err, ErrNotOnHub) {
		t.Errorf("an unknown macro: %v", err)
	}
	if err := w.Press(ctx, c, "Receiver", "Mute", false); err != nil {
		t.Fatal(err)
	}
}
