package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/record"
)

// ErrNotOnHub reports a command to send that the hub does not have yet.
var ErrNotOnHub = errors.New("this is not on the hub yet: review and apply your changes first, then test it")

// Send makes the hub send a device's command now, to test it. The device
// and command must be on the hub: Send sends what the hub has, which is
// the command as last applied.
func (w *Workspace) Send(ctx context.Context, c *hub.Client, device, command string) error {
	d := w.Config.Device(device)
	if d == nil || d.ID == 0 {
		return ErrNotOnHub
	}
	cmds, err := c.Commands(ctx, d.ID)
	if err != nil {
		return fmt.Errorf("reading %s's commands: %w", d.Name, err)
	}
	for _, cmd := range cmds {
		if strings.EqualFold(cmd.Label, command) {
			return c.Run(ctx, d.ID, []record.Step{hub.StepFor(cmd)})
		}
	}
	return ErrNotOnHub
}

// ErrNoKey reports a key that does nothing on the hub, pressed or held.
var ErrNoKey = errors.New("it does nothing on the hub")

// entity is the hub's id of the device or activity named name, and
// whether it is an activity.
func (w *Workspace) entity(name string) (id byte, activity bool, err error) {
	if d := w.Config.Device(name); d != nil {
		id = d.ID
	} else if a := w.Config.Activity(name); a != nil {
		id, activity = a.ID, true
	}
	if id == 0 {
		return 0, false, ErrNotOnHub
	}
	return id, activity, nil
}

// Press makes the hub do what a key of the device or activity named
// entity does, pressed or held, as the hub has it: as if pressed on the
// remote.
func (w *Workspace) Press(ctx context.Context, c *hub.Client, entity, key string, hold bool) error {
	id, activity, err := w.entity(entity)
	if err != nil {
		return err
	}
	button, err := record.ParseButton(key)
	if err != nil {
		return err
	}
	keys, err := c.Keymap(ctx, id)
	if err != nil {
		return fmt.Errorf("reading %s's keys: %w", entity, err)
	}
	for _, k := range keys {
		if k.Button != button {
			continue
		}
		p := k.Short
		if hold {
			p = k.Long
		}
		if p.Device == 0 {
			return ErrNoKey
		}
		if activity && p.Device == id {
			// The activity's own: one of its macros.
			return w.runMacro(ctx, c, id, func(m *record.Macro) bool { return m.ID == p.Command })
		}
		return c.Run(ctx, p.Device, []record.Step{{Device: p.Device, Command: p.Command, Code: p.Code, Delay: 0xFF}})
	}
	return ErrNoKey
}

// RunMacro makes the hub run the macro named name of the device or
// activity named entity, as the hub has it. A device's macros are run
// from their copies in its activities, where the hub keeps them.
func (w *Workspace) RunMacro(ctx context.Context, c *hub.Client, entity, name string) error {
	id, activity, err := w.entity(entity)
	if err != nil {
		return err
	}
	named := func(m *record.Macro) bool { return strings.EqualFold(m.Label, name) }
	if activity {
		return w.runMacro(ctx, c, id, func(m *record.Macro) bool { return m.Origin == 0 && named(m) })
	}
	for _, a := range w.Config.Activities {
		if a.ID == 0 {
			continue
		}
		err := w.runMacro(ctx, c, a.ID, func(m *record.Macro) bool { return m.Origin == id && named(m) })
		if !errors.Is(err, ErrNotOnHub) {
			return err
		}
	}
	return ErrNotOnHub
}

// runMacro runs the first macro of entity that is the one.
func (w *Workspace) runMacro(ctx context.Context, c *hub.Client, entity byte, the func(*record.Macro) bool) error {
	ms, err := c.Macros(ctx, entity)
	if err != nil {
		return fmt.Errorf("reading macros: %w", err)
	}
	for _, m := range ms {
		if the(m) {
			return c.Run(ctx, entity, m.Steps)
		}
	}
	return ErrNotOnHub
}
