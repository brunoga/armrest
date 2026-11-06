package workspace

import (
	"github.com/brunoga/sofabaton/hue"
	"github.com/brunoga/sofabaton/profile"
)

// HueUser is the user a Hue bridge device of the hub holds for the bridge
// at addr, to talk to it without pressing its link button again; empty
// if none does.
func (w *Workspace) HueUser(addr string) string {
	for _, d := range w.Config.Devices {
		if d.Hue != nil && d.Hue.Bridge == addr {
			return d.Hue.User
		}
	}
	return ""
}

// IsHue reports whether the device is a Philips Hue bridge.
func (w *Workspace) IsHue(device string) bool {
	d := w.Config.Device(device)
	return d != nil && d.Hue != nil
}

// HueDevice is the bridge device of the commands, named as the app names
// it, with a number if the hub has one of that name already (the app
// adds a bridge twice for more commands than one device takes).
func (w *Workspace) HueDevice(c *hue.Client, cmds []profile.Command) *profile.Device {
	d := hue.Device(c, cmds)
	d.Name = w.FreeName(d.Name)
	return d
}
