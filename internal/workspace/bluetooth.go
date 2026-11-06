package workspace

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/profile"
)

// OtherBluetoothProfile is the id of the profile Sofabaton's catalog
// offers for a Bluetooth device of no listed brand ("Other"), which a
// device made from scratch carries, as the app's would.
const OtherBluetoothProfile = "de011c8ae34543ecac2db4a73729eebd"

// HIDKey is a key the hub can press as a Bluetooth keyboard and remote:
// its name and its HID usage, as a command's data (the usage page, then
// the usage).
type HIDKey struct {
	Name, Usage string
}

// HIDKeys are the keys a Bluetooth command can press, those of remotes
// first, with the usages of the catalog's Android Box profile.
var HIDKeys = func() []HIDKey {
	keys := []HIDKey{
		{"Up", "070052"}, {"Down", "070051"}, {"Left", "070050"}, {"Right", "07004f"},
		{"OK", "0c0041"}, {"Back", "0c0224"}, {"Home", "0c0223"}, {"Menu", "070076"},
		{"Power", "0c0030"}, {"Sleep", "0c0032"},
		{"Volume Up", "0c00e9"}, {"Volume Down", "0c00ea"}, {"Mute", "0c00e2"},
		{"Play/Pause", "0c00cd"}, {"Play", "0c00b0"}, {"Stop", "0c00b7"}, {"Record", "0c00b2"},
		{"Fast Forward", "0c00b3"}, {"Rewind", "0c00b4"}, {"Next", "0c00b5"}, {"Previous", "0c00b6"},
		{"Channel Up", "0c009c"}, {"Channel Down", "0c009d"}, {"Search", "0c0221"},
		{"Enter", "070028"}, {"Escape", "070029"}, {"Backspace", "07002a"}, {"Tab", "07002b"},
		{"Space", "07002c"}, {"Page Up", "07004b"}, {"Page Down", "07004e"},
	}
	for d := 1; d <= 10; d++ {
		keys = append(keys, HIDKey{fmt.Sprint(d % 10), fmt.Sprintf("07%04x", 0x1d+d)})
	}
	for l := 'A'; l <= 'Z'; l++ {
		keys = append(keys, HIDKey{string(l), fmt.Sprintf("07%04x", 0x04+l-'A')})
	}
	for f := 1; f <= 12; f++ {
		keys = append(keys, HIDKey{fmt.Sprint("F", f), fmt.Sprintf("07%04x", 0x39+f)})
	}
	return keys
}()

// HIDKeyNamed is the key of HIDKeys named name.
func HIDKeyNamed(name string) (HIDKey, bool) {
	for _, k := range HIDKeys {
		if strings.EqualFold(k.Name, strings.TrimSpace(name)) {
			return k, true
		}
	}
	return HIDKey{}, false
}

// HIDKeyFor is the key of HIDKeys that presses usage.
func HIDKeyFor(usage string) (HIDKey, bool) {
	for _, k := range HIDKeys {
		if strings.EqualFold(k.Usage, usage) {
			return k, true
		}
	}
	return HIDKey{}, false
}

// isUsage reports whether s is a HID usage as a command's data: three
// bytes, in hex.
func isUsage(s string) bool {
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 3
}

// IsBluetooth reports whether the device is a Bluetooth one.
func (w *Workspace) IsBluetooth(device string) bool {
	d := w.Config.Device(device)
	return d != nil && d.Bluetooth != nil
}

// UsualBluetoothKeys are the keys a new Bluetooth device starts with:
// what a TV box's remote has.
var UsualBluetoothKeys = []string{
	"Up", "Down", "Left", "Right", "OK", "Back", "Home",
	"Play/Pause", "Volume Up", "Volume Down", "Mute",
}

// BluetoothSpec is a Bluetooth device being made, as the user picked it:
// one the hub pairs with and presses keys of, as a keyboard and remote.
type BluetoothSpec struct {
	Name, Brand string
	// Classic is set for a device that pairs over classic Bluetooth.
	Classic  bool
	Commands []BluetoothCommand
	// PowerOn and PowerOff name the commands that switch it on and off;
	// empty if none.
	PowerOn, PowerOff string
}

// BluetoothCommand is a command as picked: a name and the key it presses
// (a name of HIDKeys).
type BluetoothCommand struct {
	Name, Key string
}

// Build makes the Bluetooth device the spec describes. Commands whose
// name is a key of the remote's are set on that key, as in Spec.Build.
func (s BluetoothSpec) Build() (*profile.Device, error) {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		return nil, &FieldError{Field: "name", Msg: i18n.T("The device needs a name.")}
	}
	d := &profile.Device{
		Name: name, Brand: strings.TrimSpace(s.Brand), Category: bluetoothCategory,
		Bluetooth: &profile.Bluetooth{Profile: OtherBluetoothProfile, Classic: s.Classic},
		Buttons:   profile.Buttons{},
	}
	seen := map[string]bool{}
	for i, c := range s.Commands {
		cname := strings.TrimSpace(c.Name)
		if cname == "" && c.Key == "" {
			continue // an empty row
		}
		key, ok := HIDKeyNamed(c.Key)
		if !ok {
			return nil, &FieldError{Field: "command", Index: i, Msg: i18n.T("Pick the key command %d presses.", i+1)}
		}
		if cname == "" {
			cname = key.Name
		}
		if seen[strings.ToLower(cname)] {
			return nil, &FieldError{Field: "command", Index: i, Msg: i18n.T("There are two commands named %q.", cname)}
		}
		seen[strings.ToLower(cname)] = true
		d.Commands = append(d.Commands, profile.Command{Name: cname, Data: key.Usage})
		if button, ok := KeyFor(cname); ok {
			if _, taken := d.Buttons[button]; !taken {
				d.Buttons[button] = profile.Key{Short: cname}
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
