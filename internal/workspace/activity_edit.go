package workspace

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/brunoga/sofabaton/profile"
)

// ActivityEdit is an activity as its editor holds it.
type ActivityEdit struct {
	// Original is the activity's name before the edit, or empty for a new
	// one.
	Original string
	Name     string
	Icon     profile.Icon
	Color    profile.Color
	// Devices are its devices, each with the input it switches to.
	Devices []profile.Member
	// Keys bind keys of the remote to its devices' commands and macros
	// ("Device: Command") or its own macros (by name).
	Keys profile.Buttons
	// PowerOn and PowerOff are what starting and ending it does; nil is
	// the usual: each device turned on (off) a second apart, then, when
	// starting, each switched to its input.
	PowerOn, PowerOff []profile.Step
	Macros            []ActivityMacroEdit
	// Favorites are commands of its devices ("Device: Command"), in the
	// remote's order.
	Favorites []string
}

// ActivityMacroEdit is an activity macro in the editor.
type ActivityMacroEdit struct {
	Original string
	Name     string
	Steps    []profile.Step
}

// NewActivity is the editor's start for a new activity.
func (w *Workspace) NewActivity() ActivityEdit {
	return ActivityEdit{Keys: profile.Buttons{}}
}

// EditActivity is the editor's start for the activity named name.
func (w *Workspace) EditActivity(name string) (ActivityEdit, error) {
	a := w.Config.Activity(name)
	if a == nil {
		return ActivityEdit{}, fmt.Errorf("no activity %q", name)
	}
	e := ActivityEdit{
		Original: a.Name, Name: a.Name, Icon: a.Icon, Color: a.Color,
		Devices:  slices.Clone(a.Devices),
		Keys:     profile.Buttons{},
		PowerOn:  slices.Clone(a.PowerOn), PowerOff: slices.Clone(a.PowerOff),
		Favorites: slices.Clone([]string(a.Favorites)),
	}
	for k, v := range a.Buttons {
		e.Keys[k] = v
	}
	for _, m := range a.Macros {
		e.Macros = append(e.Macros, ActivityMacroEdit{Original: m.Name, Name: m.Name, Steps: slices.Clone(m.Steps)})
	}
	return e, nil
}

// Commands are what an activity with these devices can send: each
// device's commands, as "Device: Command".
func (w *Workspace) Commands(devices []string) []string {
	var out []string
	for _, n := range devices {
		if d := w.Config.Device(n); d != nil {
			for _, c := range d.Commands {
				out = append(out, d.Name+": "+c.Name)
			}
		}
	}
	return out
}

// DeviceMacros are the devices' macros, as "Device: Macro".
func (w *Workspace) DeviceMacros(devices []string) []string {
	var out []string
	for _, n := range devices {
		if d := w.Config.Device(n); d != nil {
			for _, m := range d.Macros {
				out = append(out, d.Name+": "+m.Name)
			}
		}
	}
	return out
}

// DeviceNames are the configuration's devices, in order.
func (w *Workspace) DeviceNames() []string {
	var out []string
	for _, d := range w.Config.Devices {
		out = append(out, d.Name)
	}
	return out
}

// Inputs are the inputs of the device named device.
func (w *Workspace) Inputs(device string) []string {
	if d := w.Config.Device(device); d != nil {
		return d.Inputs
	}
	return nil
}

// SaveActivity makes the configuration hold the edit, adding the
// activity if it is new. Nothing changes if the edit is not valid.
func (w *Workspace) SaveActivity(e ActivityEdit) error {
	name := strings.TrimSpace(e.Name)
	if name == "" {
		return &FieldError{Field: "name", Msg: "The activity needs a name."}
	}
	if o := w.Config.Activity(name); o != nil && !strings.EqualFold(o.Name, e.Original) {
		return &FieldError{Field: "name", Msg: fmt.Sprintf("There is an activity named %q already.", o.Name)}
	}
	var members []string
	seen := map[string]bool{}
	for i, m := range e.Devices {
		d := w.Config.Device(m.Device)
		switch {
		case d == nil:
			return &FieldError{Field: "device", Index: i, Msg: fmt.Sprintf("Pick device %d.", i+1)}
		case seen[strings.ToLower(d.Name)]:
			return &FieldError{Field: "device", Index: i, Msg: fmt.Sprintf("%s is in the activity twice.", d.Name)}
		case m.Input != "" && !slices.Contains(d.Inputs, m.Input):
			return &FieldError{Field: "device", Index: i, Msg: fmt.Sprintf("%s has no input %q.", d.Name, m.Input)}
		}
		seen[strings.ToLower(d.Name)] = true
		members = append(members, d.Name)
	}
	if len(members) == 0 {
		// The hub deletes an activity with no devices.
		return &FieldError{Field: "devices", Msg: "An activity needs at least one device."}
	}

	runs := map[string]bool{}
	for _, c := range append(w.Commands(members), w.DeviceMacros(members)...) {
		runs[strings.ToLower(c)] = true
	}
	// Steps send commands; keys can also run macros.
	commands := map[string]bool{}
	for _, c := range w.Commands(members) {
		commands[strings.ToLower(c)] = true
	}
	macroNames := map[string]bool{}
	a := &profile.Activity{Name: name, Icon: e.Icon, Color: e.Color, Devices: slices.Clone(e.Devices)}
	for i, m := range e.Macros {
		n := strings.TrimSpace(m.Name)
		switch {
		case n == "":
			return &FieldError{Field: "macro", Index: i, Msg: fmt.Sprintf("Macro %d needs a name.", i+1)}
		case macroNames[strings.ToLower(n)]:
			return &FieldError{Field: "macro", Index: i, Msg: fmt.Sprintf("There are two macros named %q.", n)}
		case len(m.Steps) == 0:
			return &FieldError{Field: "macro", Index: i, Msg: fmt.Sprintf("%s: a macro needs at least one step.", n)}
		}
		if err := checkSteps(m.Steps, seen, commands, n); err != nil {
			return &FieldError{Field: "macro", Index: i, Msg: err.Error()}
		}
		macroNames[strings.ToLower(n)] = true
		a.Macros = append(a.Macros, profile.ActivityMacro{Name: n, Steps: m.Steps})
	}
	for _, p := range []struct {
		field string
		steps []profile.Step
		to    *[]profile.Step
	}{{"power_on", e.PowerOn, &a.PowerOn}, {"power_off", e.PowerOff, &a.PowerOff}} {
		if p.steps == nil {
			continue
		}
		what := "When it starts"
		if p.field == "power_off" {
			what = "When it ends"
		}
		if err := checkSteps(p.steps, seen, commands, what); err != nil {
			return &FieldError{Field: p.field, Msg: err.Error()}
		}
		*p.to = p.steps
	}
	runnable := func(s string) bool {
		return s != "" && (runs[strings.ToLower(s)] || macroNames[strings.ToLower(s)])
	}
	for k, v := range e.Keys {
		if !runnable(v.Short) {
			continue
		}
		if !runnable(v.Long) {
			v.Long = ""
		}
		if a.Buttons == nil {
			a.Buttons = profile.Buttons{}
		}
		a.Buttons[k] = v
	}
	if e.Favorites != nil {
		a.Favorites = profile.FavoriteList{}
		for _, f := range e.Favorites {
			if commands[strings.ToLower(f)] && !slices.Contains(a.Favorites, f) {
				a.Favorites = append(a.Favorites, f)
			}
		}
	}

	if err := a.Validate(); err != nil {
		return fmt.Errorf("the activity is not valid: %w", err)
	}
	cfg, err := cloneConfig(w.Config)
	if err != nil {
		return err
	}
	if e.Original == "" {
		cfg.Activities = append(cfg.Activities, a)
	} else {
		old := cfg.Activity(e.Original)
		if old == nil {
			return fmt.Errorf("no activity %q", e.Original)
		}
		a.ID = old.ID
		*old = *a
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	w.Config = cfg
	return nil
}

// checkSteps checks steps against the activity's devices (members, lower
// case) and their commands (lower case "device: command").
func checkSteps(steps []profile.Step, members, commands map[string]bool, what string) error {
	for j, s := range steps {
		switch s.Kind {
		case profile.StepWait:
			if s.Seconds == 0 {
				return fmt.Errorf("%s, step %d: wait at least a second", what, j+1)
			}
		case profile.StepCommand:
			if !commands[strings.ToLower(s.Device+": "+s.Command)] {
				return fmt.Errorf("%s, step %d: pick a command of the activity's devices", what, j+1)
			}
		default:
			if !members[strings.ToLower(s.Device)] {
				return fmt.Errorf("%s, step %d: pick one of the activity's devices", what, j+1)
			}
		}
	}
	return nil
}

// UsualSteps are what starting (on) or ending an activity with these
// devices does by default, as Sofabaton's app writes it: each device
// turned on (off) a second apart and, starting, each switched to its
// input.
func UsualSteps(devices []string, on bool) []profile.Step {
	kind := profile.StepPowerOff
	if on {
		kind = profile.StepPowerOn
	}
	var out []profile.Step
	for _, d := range devices {
		out = append(out, profile.Step{Kind: kind, Device: d}, profile.Step{Kind: profile.StepWait, Seconds: 1})
	}
	if on {
		for _, d := range devices {
			out = append(out, profile.Step{Kind: profile.StepInput, Device: d}, profile.Step{Kind: profile.StepWait, Seconds: 1})
		}
	}
	return out
}

// IsUsual reports whether steps are the usual ones for the devices.
func IsUsual(steps []profile.Step, devices []string, on bool) bool {
	return steps != nil && slices.EqualFunc(steps, UsualSteps(devices, on), func(a, b profile.Step) bool {
		return a.Kind == b.Kind && strings.EqualFold(a.Device, b.Device) && a.Command == b.Command && a.Seconds == b.Seconds && a.Hold == b.Hold
	})
}

// RemoveActivity removes an activity.
func (w *Workspace) RemoveActivity(name string) error {
	if w.Config.Activity(name) == nil {
		return errors.New("no activity " + name)
	}
	return w.Config.RemoveActivity(name)
}
