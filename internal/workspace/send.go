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
