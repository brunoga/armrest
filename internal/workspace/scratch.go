package workspace

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/profile"
)

// Protocol is an IR protocol a device made from scratch can use: one the
// library makes codes for.
type Protocol struct {
	// ID is the library's name for it.
	ID string
	// Title and About describe it to the user.
	Title, About string
	// MaxCode is the largest command code; Decimal says manufacturers
	// list codes in decimal.
	MaxCode uint16
	Decimal bool
}

// Protocols are the IR protocols, the most common first.
var Protocols = []Protocol{
	{ID: "NEC", Title: i18n.Mark("NEC"), About: i18n.Mark("Most TVs, projectors, media players and set-top boxes. Each device has a system code (often listed as its address or custom code), and each command a code from 0x00 to 0xFF."), MaxCode: 0xFF},
	{ID: "DenonK", Title: i18n.Mark("Denon, newer receivers"), About: i18n.Mark("Denon and Marantz receivers whose IR charts list a device, a subdevice and a function (0 to 4095) for each command."), MaxCode: 0xFFF, Decimal: true},
	{ID: "Denon", Title: i18n.Mark("Denon, older receivers"), About: i18n.Mark("Denon receivers whose IR charts list a device (an address, 0 to 31) and a function (0 to 255)."), MaxCode: 0xFF, Decimal: true},
}

// ProtocolNamed is the protocol with that ID.
func ProtocolNamed(id string) (Protocol, bool) {
	for _, p := range Protocols {
		if p.ID == id {
			return p, true
		}
	}
	return Protocol{}, false
}

// Spec is a device being made from scratch, as the user typed it.
type Spec struct {
	Name, Brand string
	// Protocol is a Protocols ID.
	Protocol string
	// System is NEC's system code ("0x00FF"); Device and Subdevice are
	// Denon's.
	System            string
	Device, Subdevice string
	Commands          []CommandSpec
	// PowerOn and PowerOff name the commands that switch it on and off
	// (the same one for a device with a single power key); empty if none.
	PowerOn, PowerOff string
}

// CommandSpec is a command as typed: a name and a code.
type CommandSpec struct {
	Name, Code string
}

// FieldError is a problem with what was typed, saying where.
type FieldError struct {
	// Field is "name", "system", "device", "subdevice", "commands",
	// "command" (with Index) or "power".
	Field string
	Index int
	Msg   string
}

func (e *FieldError) Error() string { return e.Msg }

// Build makes the device the spec describes. Commands whose name is a key
// of the remote's (OK, Up, Vol+, or a usual name for one such as Volume
// Up) are set on that key.
func (s Spec) Build() (*profile.Device, error) {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		return nil, &FieldError{Field: "name", Msg: i18n.T("The device needs a name.")}
	}
	p, ok := ProtocolNamed(s.Protocol)
	if !ok {
		return nil, &FieldError{Field: "protocol", Msg: i18n.T("Pick how the device's remote sends its codes.")}
	}
	ir := &profile.IR{Protocol: p.ID}
	switch p.ID {
	case "NEC":
		v, err := strconv.ParseUint(strings.TrimSpace(s.System), 0, 16)
		if err != nil {
			return nil, &FieldError{Field: "system", Msg: i18n.T("The system code is a number from 0x0000 to 0xFFFF, such as 0x00FF.")}
		}
		ir.System = profile.Hex16(v)
	case "DenonK":
		d, err1 := small(s.Device, 15)
		sub, err2 := small(s.Subdevice, 15)
		if err1 != nil {
			return nil, &FieldError{Field: "device", Msg: i18n.T("The device is a number from 0 to 15 (4 for most receivers).")}
		}
		if err2 != nil {
			return nil, &FieldError{Field: "subdevice", Msg: i18n.T("The subdevice is a number from 0 to 15 (1 for a receiver's main zone).")}
		}
		ir.Device, ir.Subdevice = &d, &sub
	case "Denon":
		d, err := small(s.Device, 31)
		if err != nil {
			return nil, &FieldError{Field: "device", Msg: i18n.T("The device is a number from 0 to 31 (2 or 12 for most receivers).")}
		}
		ir.Device = &d
	}

	d := &profile.Device{Name: name, Brand: strings.TrimSpace(s.Brand), IR: ir, Buttons: profile.Buttons{}}
	seen := map[string]bool{}
	for i, c := range s.Commands {
		cname := strings.TrimSpace(c.Name)
		code := strings.TrimSpace(c.Code)
		if cname == "" && code == "" {
			continue // an empty row
		}
		if cname == "" {
			return nil, &FieldError{Field: "command", Index: i, Msg: i18n.T("Command %d needs a name.", i+1)}
		}
		if seen[strings.ToLower(cname)] {
			return nil, &FieldError{Field: "command", Index: i, Msg: i18n.T("There are two commands named %q.", cname)}
		}
		seen[strings.ToLower(cname)] = true
		v, err := strconv.ParseUint(code, 0, 16)
		if err != nil || v > uint64(p.MaxCode) {
			return nil, &FieldError{Field: "command", Index: i, Msg: i18n.T("%s: the code is a number from 0 to %d (0x%X).", cname, p.MaxCode, p.MaxCode)}
		}
		decimal := !strings.HasPrefix(strings.ToLower(code), "0x")
		d.Commands = append(d.Commands, profile.Command{Name: cname, Code: &profile.Code{Value: uint16(v), Decimal: decimal}})
		if key, ok := KeyFor(cname); ok {
			if _, taken := d.Buttons[key]; !taken {
				d.Buttons[key] = profile.Key{Short: cname}
			}
		}
	}
	if len(d.Commands) == 0 {
		return nil, &FieldError{Field: "commands", Msg: i18n.T("Add at least one command.")}
	}
	if len(d.Buttons) == 0 {
		d.Buttons = nil
	}
	on, off := strings.TrimSpace(s.PowerOn), strings.TrimSpace(s.PowerOff)
	if on != "" || off != "" {
		if on == "" || off == "" {
			return nil, &FieldError{Field: "power", Msg: i18n.T("Pick both the command that turns the device on and the one that turns it off (the same one if it has a single power key).")}
		}
		for _, n := range []string{on, off} {
			if !seen[strings.ToLower(n)] {
				return nil, &FieldError{Field: "power", Msg: i18n.T("There is no command named %q.", n)}
			}
		}
		d.Power = &profile.Power{On: on, Off: off}
	}
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("the device is not valid"), err)
	}
	return d, nil
}

func small(s string, max uint64) (uint8, error) {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 8)
	if err != nil || v > max {
		return 0, errors.New("out of range")
	}
	return uint8(v), nil
}

// keyNames are the usual names of commands for each key of the remote, by
// the key's name in the library.
var keyNames = map[string][]string{
	"OK":      {"ok", "select", "enter", "confirm"},
	"Up":      {"up", "cursor up", "arrow up"},
	"Down":    {"down", "cursor down", "arrow down"},
	"Left":    {"left", "cursor left", "arrow left"},
	"Right":   {"right", "cursor right", "arrow right"},
	"Back":    {"back", "return"},
	"Home":    {"home"},
	"Menu":    {"menu", "options", "context menu"},
	"Vol+":    {"vol+", "volume up", "vol up"},
	"Vol-":    {"vol-", "volume down", "vol down"},
	"Mute":    {"mute"},
	"Ch+":     {"ch+", "channel up", "ch up", "page up"},
	"Ch-":     {"ch-", "channel down", "ch down", "page down"},
	"Play":    {"play", "play/pause", "play pause"},
	"Pause":   {"pause"},
	"Rewind":  {"rewind", "rew"},
	"Forward": {"forward", "fast forward", "ff"},
	"Guide":   {"guide", "epg"},
	"Exit":    {"exit"},
	"DVR":     {"dvr", "record", "rec"},
	"Red":     {"red"},
	"Green":   {"green"},
	"Yellow":  {"yellow"},
	"Blue":    {"blue"},
	"Power":   {"power", "power toggle"},
	"0":       {"0"}, "1": {"1"}, "2": {"2"}, "3": {"3"}, "4": {"4"},
	"5": {"5"}, "6": {"6"}, "7": {"7"}, "8": {"8"}, "9": {"9"},
}

// KeyFor is the key of the remote a command of that name goes on, if
// any.
func KeyFor(command string) (string, bool) {
	c := strings.ToLower(strings.TrimSpace(command))
	for key, names := range keyNames {
		for _, n := range names {
			if c == n {
				return key, true
			}
		}
	}
	return "", false
}

// CommonCommands are names to suggest for commands, the usual ones first.
var CommonCommands = []string{
	"Power On", "Power Off", "Power", "Up", "Down", "Left", "Right", "OK", "Back", "Home", "Menu",
	"Volume Up", "Volume Down", "Mute", "Channel Up", "Channel Down", "Play", "Pause", "Play/Pause",
	"Stop", "Rewind", "Fast Forward", "Next", "Previous", "Guide", "Info", "Exit", "Input",
	"Red", "Green", "Yellow", "Blue",
	"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
}
