package workspace

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/profile"
)

// DeviceEdit is a device as its editor holds it.
type DeviceEdit struct {
	// Original is the device's name before the edit.
	Original    string
	Name, Brand string
	Icon        profile.Icon
	Color       profile.Color
	Category    profile.Category
	// Commands are in the order the remote lists them.
	Commands []CommandEdit
	// Keys bind keys of the remote (by name) to commands (by their names
	// in Commands).
	Keys profile.Buttons
	// PowerOn and PowerOff name commands, or are both empty.
	PowerOn, PowerOff string
	// Inputs name the commands that select each input; NoInputs says the
	// device switches none (as opposed to not set).
	Inputs   []string
	NoInputs bool
	// Macros are the device's macros; activities with the device run
	// them from their keys.
	Macros []MacroEdit
}

// MacroEdit is a device macro in the editor.
type MacroEdit struct {
	// Original is its name before the edit, or empty for a new one.
	Original string
	Name     string
	// Steps send the device's commands (by their names in Commands) or
	// wait.
	Steps []profile.DeviceStep
}

// CommandEdit is a command in the editor.
type CommandEdit struct {
	// Original is its name before the edit, or empty for a new one.
	Original string
	Name     string
	// Code is its code in the device's protocol. A command whose signal
	// is not a code (learned, or a descriptor of another protocol) has
	// none, and keeps its signal.
	Code string
	// Fixed says the signal is not a code, and cannot be edited.
	Fixed bool
	// Data, set on a Fixed command, is a signal it gets in place of the
	// one it has: a code learned from another remote, as the hub holds it
	// (hex).
	Data string
	// HTTP, when set, is the request a network command sends, which is
	// edited instead of a code.
	HTTP *profile.HTTP
	// Usage, when set, is the HID usage a Bluetooth device's command
	// presses (see HIDKeys), which is edited instead of a code.
	Usage string
}

// EditDevice is the editor's start for the device named name.
func (w *Workspace) EditDevice(name string) (DeviceEdit, error) {
	d := w.Config.Device(name)
	if d == nil {
		return DeviceEdit{}, errors.New(i18n.T("no device %q", name))
	}
	e := DeviceEdit{Original: d.Name, Name: d.Name, Brand: d.Brand, Icon: d.Icon, Color: d.Color, Category: d.Category, Keys: profile.Buttons{}}
	for _, c := range d.Commands {
		ce := CommandEdit{Original: c.Name, Name: c.Name, Fixed: c.Code == nil && c.HTTP == nil}
		if d.Bluetooth != nil && c.Code == nil && c.HTTP == nil && isUsage(c.Data) {
			ce.Usage, ce.Fixed = strings.ToLower(c.Data), false
		}
		if c.Code != nil {
			ce.Code = codeText(*c.Code)
		}
		if c.HTTP != nil {
			h := *c.HTTP
			h.Headers = append([]string(nil), h.Headers...)
			ce.HTTP = &h
		}
		e.Commands = append(e.Commands, ce)
	}
	for k, v := range d.Buttons {
		e.Keys[k] = v
	}
	if d.Power != nil {
		e.PowerOn, e.PowerOff = d.Power.On, d.Power.Off
	}
	e.Inputs = append(e.Inputs, d.Inputs...)
	e.NoInputs = d.Inputs != nil && len(d.Inputs) == 0
	for _, m := range d.Macros {
		e.Macros = append(e.Macros, MacroEdit{Original: m.Name, Name: m.Name, Steps: append([]profile.DeviceStep(nil), m.Steps...)})
	}
	return e, nil
}

func codeText(c profile.Code) string {
	if c.Decimal {
		return strconv.Itoa(int(c.Value))
	}
	return fmt.Sprintf("0x%02X", c.Value)
}

// CanAddCommands reports whether new commands can be added: given as
// codes, the device having an IR protocol, as requests, on a network
// device (IsNetwork), or as keys, on a Bluetooth one (IsBluetooth).
func (w *Workspace) CanAddCommands(device string) bool {
	d := w.Config.Device(device)
	return d != nil && (d.IR != nil || w.IsNetwork(device) || d.Bluetooth != nil)
}

// CanLearn reports whether the device can have commands learned from
// another remote: any device but a network one or a service's (their
// commands are not IR).
func (w *Workspace) CanLearn(device string) bool {
	d := w.Config.Device(device)
	return d != nil && !w.IsNetwork(device) && w.Service(device) == ""
}

// ActivitiesUsing names the activities that use the device.
func (w *Workspace) ActivitiesUsing(device string) []string {
	var out []string
	for _, a := range w.Config.Activities {
		for _, m := range a.Devices {
			if strings.EqualFold(m.Device, device) {
				out = append(out, a.Name)
				break
			}
		}
	}
	return out
}

// SaveDevice makes the configuration hold the edit: renames of the device
// and its commands carry to the activities, and removed commands leave
// what used them. Nothing changes if the edit is not valid.
func (w *Workspace) SaveDevice(e DeviceEdit) error {
	cfg, err := cloneConfig(w.Config)
	if err != nil {
		return err
	}
	d := cfg.Device(e.Original)
	if d == nil {
		return errors.New(i18n.T("no device %q", e.Original))
	}
	name := strings.TrimSpace(e.Name)
	if name == "" {
		return &FieldError{Field: "name", Msg: i18n.T("The device needs a name.")}
	}
	var proto Protocol
	if d.IR != nil {
		proto, _ = ProtocolNamed(d.IR.Protocol)
	}

	// Check the commands before changing anything.
	seen := map[string]bool{}
	kept := map[string]bool{}
	for i, c := range e.Commands {
		c.Name = strings.TrimSpace(c.Name)
		if d.Bluetooth != nil && !c.Fixed && c.HTTP == nil && !isUsage(c.Usage) {
			return &FieldError{Field: "command", Index: i, Msg: i18n.T("Pick the key command %d presses.", i+1)}
		}
		if c.Name == "" {
			return &FieldError{Field: "command", Index: i, Msg: i18n.T("Command %d needs a name.", i+1)}
		}
		if seen[strings.ToLower(c.Name)] {
			return &FieldError{Field: "command", Index: i, Msg: i18n.T("There are two commands named %q.", c.Name)}
		}
		seen[strings.ToLower(c.Name)] = true
		if c.Original != "" {
			kept[strings.ToLower(c.Original)] = true
		}
		if c.Fixed {
			continue
		}
		if c.HTTP != nil {
			if err := CheckRequest(c.Name, tidyRequest(*c.HTTP)); err != nil {
				return &FieldError{Field: "command", Index: i, Msg: err.Error()}
			}
			continue
		}
		if d.Bluetooth != nil {
			continue // its key, checked above
		}
		if d.IR == nil {
			return &FieldError{Field: "command", Index: i, Msg: i18n.T("%s: this device has no IR protocol, so a command cannot be given as a code.", c.Name)}
		}
		if v, err := strconv.ParseUint(strings.TrimSpace(c.Code), 0, 16); err != nil || v > uint64(proto.MaxCode) {
			return &FieldError{Field: "command", Index: i, Msg: i18n.T("%s: the code is a number from 0 to %d (0x%X).", c.Name, proto.MaxCode, proto.MaxCode)}
		}
	}
	if len(e.Commands) == 0 {
		return &FieldError{Field: "commands", Msg: i18n.T("A device needs at least one command.")}
	}
	macros := map[string]bool{}
	keptMacros := map[string]bool{}
	for i, m := range e.Macros {
		n := strings.TrimSpace(m.Name)
		switch {
		case n == "":
			return &FieldError{Field: "macro", Index: i, Msg: i18n.T("Macro %d needs a name.", i+1)}
		case macros[strings.ToLower(n)]:
			return &FieldError{Field: "macro", Index: i, Msg: i18n.T("There are two macros named %q.", n)}
		case seen[strings.ToLower(n)]:
			return &FieldError{Field: "macro", Index: i, Msg: i18n.T("%q is a command's name; a macro needs another.", n)}
		case len(m.Steps) == 0:
			return &FieldError{Field: "macro", Index: i, Msg: i18n.T("%s: a macro needs at least one step.", n)}
		}
		macros[strings.ToLower(n)] = true
		if m.Original != "" {
			keptMacros[strings.ToLower(m.Original)] = true
		}
		for j, st := range m.Steps {
			if st.Command == "" && st.Seconds == 0 {
				return &FieldError{Field: "macro", Index: i, Msg: i18n.T("%s, step %d: pick a command, or wait at least a second.", n, j+1)}
			}
			if st.Command != "" && !seen[strings.ToLower(st.Command)] {
				return &FieldError{Field: "macro", Index: i, Msg: i18n.T("%s, step %d: there is no command named %q.", n, j+1, st.Command)}
			}
		}
	}

	// Removed commands go, with what used them; renames carry over,
	// through temporary names so that swaps work.
	for _, c := range append([]profile.Command(nil), d.Commands...) {
		if !kept[strings.ToLower(c.Name)] {
			if err := cfg.RemoveCommand(d.Name, c.Name); err != nil {
				return err
			}
		}
	}
	for i, c := range e.Commands {
		if c.Original != "" && c.Original != strings.TrimSpace(c.Name) {
			if err := cfg.RenameCommand(d.Name, c.Original, fmt.Sprintf("\x00%d", i)); err != nil {
				return err
			}
		}
	}
	for i, c := range e.Commands {
		if c.Original != "" && c.Original != strings.TrimSpace(c.Name) {
			if err := cfg.RenameCommand(d.Name, fmt.Sprintf("\x00%d", i), strings.TrimSpace(c.Name)); err != nil {
				return err
			}
		}
	}
	for _, m := range append([]profile.DeviceMacro(nil), d.Macros...) {
		if !keptMacros[strings.ToLower(m.Name)] {
			if err := cfg.RemoveMacro(d.Name, m.Name); err != nil {
				return err
			}
		}
	}
	for i, m := range e.Macros {
		if m.Original != "" && m.Original != strings.TrimSpace(m.Name) {
			if err := cfg.RenameMacro(d.Name, m.Original, fmt.Sprintf("\x00m%d", i)); err != nil {
				return err
			}
		}
	}
	for i, m := range e.Macros {
		if m.Original != "" && m.Original != strings.TrimSpace(m.Name) {
			if err := cfg.RenameMacro(d.Name, fmt.Sprintf("\x00m%d", i), strings.TrimSpace(m.Name)); err != nil {
				return &FieldError{Field: "macro", Index: i, Msg: err.Error()}
			}
		}
	}
	if name != d.Name {
		if err := cfg.RenameDevice(d.Name, name); err != nil {
			return &FieldError{Field: "name", Msg: err.Error()}
		}
	}

	// The device's own fields, in the editor's order.
	byName := map[string]profile.Command{}
	for _, c := range d.Commands {
		byName[strings.ToLower(c.Name)] = c
	}
	var cmds []profile.Command
	for _, c := range e.Commands {
		n := strings.TrimSpace(c.Name)
		pc := byName[strings.ToLower(n)]
		pc.Name = n
		switch {
		case c.Fixed && c.Data != "":
			pc.Data, pc.Code, pc.Descriptor, pc.HTTP = c.Data, nil, "", nil
		case c.HTTP != nil:
			h := tidyRequest(*c.HTTP)
			pc.HTTP, pc.Code = &h, nil
			pc.Descriptor, pc.Data = "", ""
		case d.Bluetooth != nil && !c.Fixed:
			pc.Data, pc.Code, pc.Descriptor = strings.ToLower(c.Usage), nil, ""
		case !c.Fixed:
			code := strings.TrimSpace(c.Code)
			v, _ := strconv.ParseUint(code, 0, 16)
			pc.Code = &profile.Code{Value: uint16(v), Decimal: !strings.HasPrefix(strings.ToLower(code), "0x")}
			pc.Descriptor, pc.Data = "", ""
		}
		cmds = append(cmds, pc)
	}
	d.Brand = strings.TrimSpace(e.Brand)
	d.Icon, d.Color, d.Category = e.Icon, e.Color, e.Category
	d.Commands = cmds
	has := func(n string) bool { return n != "" && seen[strings.ToLower(n)] }
	d.Buttons = nil
	runnable := func(n string) bool { return has(n) || n != "" && macros[strings.ToLower(n)] }
	for k, v := range e.Keys {
		if !runnable(v.Short) {
			continue
		}
		if !runnable(v.Long) {
			v.Long = ""
		}
		if d.Buttons == nil {
			d.Buttons = profile.Buttons{}
		}
		d.Buttons[k] = v
	}
	switch {
	case e.PowerOn == "" && e.PowerOff == "":
		d.Power = nil
	case has(e.PowerOn) && has(e.PowerOff):
		d.Power = &profile.Power{On: e.PowerOn, Off: e.PowerOff}
	default:
		return &FieldError{Field: "power", Msg: i18n.T("Pick both the command that turns the device on and the one that turns it off, or neither.")}
	}
	d.Inputs = nil
	if e.NoInputs {
		d.Inputs = profile.InputList{}
	}
	for _, in := range e.Inputs {
		if has(in) {
			d.Inputs = append(d.Inputs, in)
		}
	}
	if d.Macros != nil || len(e.Macros) > 0 {
		d.Macros = profile.MacroList{}
		for _, m := range e.Macros {
			d.Macros = append(d.Macros, profile.DeviceMacro{Name: strings.TrimSpace(m.Name), Steps: m.Steps})
		}
	}
	if err := d.Validate(); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("the device is not valid"), err)
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	w.commit(cfg, i18n.T("changes to %s", name))
	return nil
}

// RemoveDevice removes a device, and from the activities that use it.
func (w *Workspace) RemoveDevice(name string) error {
	cfg, err := cloneConfig(w.Config)
	if err != nil {
		return err
	}
	if err := cfg.RemoveDevice(name); err != nil {
		return err
	}
	w.commit(cfg, i18n.T("deleting %s", name))
	return nil
}

// cloneConfig is a deep copy of cfg, through YAML.
func cloneConfig(cfg *profile.Config) (*profile.Config, error) {
	b, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var out profile.Config
	if err := yaml.Unmarshal(b, &out); err != nil {
		return nil, errors.New("copying the configuration: " + err.Error())
	}
	return &out, nil
}
