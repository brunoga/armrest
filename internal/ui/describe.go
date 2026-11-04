package ui

import (
	"slices"
	"strings"
	"time"

	"github.com/brunoga/armrest/internal/i18n"

	"github.com/brunoga/sofabaton/profile"
	"github.com/brunoga/sofabaton/record"
)

// keyNames is the keys of bs in the remote's order.
func keyNames(bs profile.Buttons) []string {
	names := make([]string, 0, len(bs))
	for n := range bs {
		names = append(names, n)
	}
	id := func(n string) int {
		if b, err := record.ParseButton(n); err == nil {
			return int(b)
		}
		return 0x100
	}
	slices.SortFunc(names, func(a, b string) int {
		if d := id(a) - id(b); d != 0 {
			return d
		}
		return strings.Compare(a, b)
	})
	return names
}

// describeKey is what a key runs, in words.
func describeKey(k profile.Key) string {
	if k.Long == "" {
		return k.Short
	}
	return i18n.T("%s; held: %s", k.Short, k.Long)
}

// describeStep is an activity's step, in words.
func describeStep(s profile.Step) string {
	switch s.Kind {
	case profile.StepWait:
		return i18n.T("Wait %s", seconds(s.Seconds))
	case profile.StepPowerOn:
		return i18n.T("Turn on %s", s.Device)
	case profile.StepPowerOff:
		return i18n.T("Turn off %s", s.Device)
	case profile.StepInput:
		return i18n.T("Switch %s to its input", s.Device)
	}
	text := s.Device + ": " + s.Command
	if s.Hold != 0 {
		text = i18n.T("%s, held %s", text, seconds(s.Hold))
	}
	return text
}

// describeDeviceStep is a device macro's step, in words.
func describeDeviceStep(s profile.DeviceStep) string {
	if s.Command == "" {
		return i18n.T("Wait %s", seconds(s.Seconds))
	}
	if s.Hold != 0 {
		return i18n.T("%s, held %s", s.Command, seconds(s.Hold))
	}
	return s.Command
}

func seconds(n uint8) string {
	return i18n.N("%d second", "%d seconds", int(n), n)
}

// took is how long something took, to the second.
func took(d time.Duration) string {
	if d < time.Second {
		return i18n.T("under a second")
	}
	n := int(d.Round(time.Second) / time.Second)
	if n < 60 {
		return i18n.N("%d second", "%d seconds", n, n)
	}
	return i18n.T("%d min %d s", n/60, n%60)
}

// describeLook is an icon and color, in words; empty if neither is set.
func describeLook(icon profile.Icon, color profile.Color) string {
	var parts []string
	if !icon.IsZero() {
		parts = append(parts, i18n.T("%s icon", icon.String()))
	}
	if !color.IsZero() {
		parts = append(parts, color.String())
	}
	return strings.Join(parts, ", ")
}
