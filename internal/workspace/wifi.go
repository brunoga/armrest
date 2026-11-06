package workspace

import (
	"fmt"
	"strings"
)

// Service is the network service a device is the hub's integration of:
// "Hue", "Roku" or "Sonos", whose commands are made from the device
// itself; empty for others.
func (w *Workspace) Service(device string) string {
	d := w.Config.Device(device)
	switch {
	case d == nil:
		return ""
	case d.Hue != nil:
		return "Hue"
	case d.Roku != nil:
		return "Roku"
	case d.Sonos != nil:
		return "Sonos"
	}
	return ""
}

// FreeName is name, or name with a number if the hub has a device of that
// name already.
func (w *Workspace) FreeName(name string) string {
	n := name
	for i := 2; w.Config.Device(n) != nil; i++ {
		n = fmt.Sprintf("%s %d", name, i)
	}
	return n
}

// HeaderValue is the value of the header named, in a command of the
// devices whose commands go to a URL starting with prefix: the key a
// cloud device of the hub holds (Govee's API key, LIFX's token), to make
// another without asking for it again; empty if none does.
func (w *Workspace) HeaderValue(prefix, header string) string {
	for _, d := range w.Config.Devices {
		for _, c := range d.Commands {
			if c.HTTP == nil || !strings.HasPrefix(c.HTTP.URL, prefix) {
				continue
			}
			for _, h := range c.HTTP.Headers {
				if name, value, ok := strings.Cut(h, ":"); ok && strings.EqualFold(strings.TrimSpace(name), header) {
					return strings.TrimSpace(value)
				}
			}
		}
	}
	return ""
}
