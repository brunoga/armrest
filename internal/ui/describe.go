package ui

import (
	"fmt"
	"slices"
	"strings"

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
	return fmt.Sprintf("%s; held: %s", k.Short, k.Long)
}

// describeStep is an activity's step, in words.
func describeStep(s profile.Step) string {
	switch s.Kind {
	case profile.StepWait:
		return fmt.Sprintf("Wait %s", seconds(s.Seconds))
	case profile.StepPowerOn:
		return "Turn on " + s.Device
	case profile.StepPowerOff:
		return "Turn off " + s.Device
	case profile.StepInput:
		return "Switch " + s.Device + " to its input"
	}
	text := s.Device + ": " + s.Command
	if s.Hold != 0 {
		text += fmt.Sprintf(", held %s", seconds(s.Hold))
	}
	return text
}

// describeDeviceStep is a device macro's step, in words.
func describeDeviceStep(s profile.DeviceStep) string {
	if s.Command == "" {
		return fmt.Sprintf("Wait %s", seconds(s.Seconds))
	}
	if s.Hold != 0 {
		return fmt.Sprintf("%s, held %s", s.Command, seconds(s.Hold))
	}
	return s.Command
}

func seconds(n uint8) string {
	if n == 1 {
		return "1 second"
	}
	return fmt.Sprintf("%d seconds", n)
}

// describeLook is an icon and color, in words; empty if neither is set.
func describeLook(icon profile.Icon, color profile.Color) string {
	var parts []string
	if !icon.IsZero() {
		parts = append(parts, icon.String()+" icon")
	}
	if !color.IsZero() {
		parts = append(parts, color.String())
	}
	return strings.Join(parts, ", ")
}
