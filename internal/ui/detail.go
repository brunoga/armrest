package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/sofabaton/profile"
)

// activityView shows an activity: its devices, keys, power steps, macros
// and favorites.
func activityView(a *profile.Activity) fyne.CanvasObject {
	parts := []fyne.CanvasObject{heading(a.Name, describeLook(a.Icon, a.Color))}

	var devs []string
	for _, m := range a.Devices {
		if m.Input != "" {
			devs = append(devs, fmt.Sprintf("%s, on %s", m.Device, m.Input))
		} else {
			devs = append(devs, m.Device)
		}
	}
	parts = append(parts, section("Devices", "No devices.", lines(devs)))
	parts = append(parts, keysSection(a.Buttons))

	steps := func(ss []profile.Step) []string {
		var out []string
		for _, s := range ss {
			out = append(out, describeStep(s))
		}
		return out
	}
	parts = append(parts,
		section("When it starts", "Nothing.", numbered(steps(a.PowerOn))),
		section("When it ends", "Nothing.", numbered(steps(a.PowerOff))))

	var macros []fyne.CanvasObject
	for _, m := range a.Macros {
		macros = append(macros, subsection(m.Name, numbered(steps(m.Steps))))
	}
	parts = append(parts, section("Macros", "No macros.", macros))
	parts = append(parts, section("Favorites", "No favorites.", numbered(a.Favorites)))
	return page(parts)
}

// deviceView shows a device: its commands, keys, power, inputs and
// macros. With send, each command has a Test button that sends it; with
// edit, an Edit button calls it.
func deviceView(d *profile.Device, send func(command string), edit func()) fyne.CanvasObject {
	sub := d.Brand
	if look := describeLook(d.Icon, d.Color); look != "" {
		if sub != "" {
			sub += " · "
		}
		sub += look
	}
	head := heading(d.Name, sub)
	if edit != nil {
		b := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), edit)
		b.Importance = widget.HighImportance
		head = container.NewBorder(nil, nil, nil, container.NewVBox(b), head)
	}
	parts := []fyne.CanvasObject{head}

	var cmds []fyne.CanvasObject
	for _, c := range d.Commands {
		name := wrapped(c.Name)
		if send == nil {
			cmds = append(cmds, name)
			continue
		}
		test := widget.NewButtonWithIcon("Test", theme.MediaPlayIcon(), func() { send(c.Name) })
		cmds = append(cmds, container.NewBorder(nil, nil, nil, test, name))
	}
	if send != nil && len(cmds) > 0 {
		hint := wrapped("Test sends a command from the hub, to check that the device responds.")
		hint.TextStyle.Italic = true
		cmds = append([]fyne.CanvasObject{hint}, cmds...)
	}
	parts = append(parts, section(fmt.Sprintf("Commands (%d)", len(d.Commands)), "No commands.", cmds))
	parts = append(parts, keysSection(d.Buttons))

	var power []fyne.CanvasObject
	if d.Power != nil {
		power = form("On", d.Power.On, "Off", d.Power.Off)
	}
	parts = append(parts, section("Power", "Not set: activities turn it on and off with its own power macros.", power))
	inputs := "Not set."
	if d.Inputs != nil {
		inputs = "It has no inputs."
	}
	parts = append(parts, section("Inputs", inputs, numbered(d.Inputs)))

	var macros []fyne.CanvasObject
	for _, m := range d.Macros {
		var steps []string
		for _, s := range m.Steps {
			steps = append(steps, describeDeviceStep(s))
		}
		macros = append(macros, subsection(m.Name, numbered(steps)))
	}
	parts = append(parts, section("Macros", "No macros.", macros))
	return page(parts)
}

func keysSection(bs profile.Buttons) fyne.CanvasObject {
	var rows []string
	for _, n := range keyNames(bs) {
		rows = append(rows, n, describeKey(bs[n]))
	}
	return section("Keys", "No keys set.", form(rows...))
}

// page is a detail's parts, scrolled, at a readable width.
func page(parts []fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVScroll(column(container.NewPadded(container.NewVBox(parts...))))
}

func heading(title, sub string) fyne.CanvasObject {
	t := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	t.SizeName = theme.SizeNameHeadingText
	t.Wrapping = fyne.TextWrapWord
	if sub == "" {
		return t
	}
	return container.NewVBox(t, wrapped(sub))
}

// section is a titled card of objs, or of the empty text if there are
// none.
func section(title, empty string, objs []fyne.CanvasObject) fyne.CanvasObject {
	if len(objs) == 0 {
		l := wrapped(empty)
		l.TextStyle.Italic = true
		objs = []fyne.CanvasObject{l}
	}
	return widget.NewCard(title, "", container.NewVBox(objs...))
}

// card is a card whose title wraps, for titles that hold names.
func card(title string, content fyne.CanvasObject) fyne.CanvasObject {
	t := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	t.SizeName = theme.SizeNameSubHeadingText
	t.Wrapping = fyne.TextWrapWord
	return widget.NewCard("", "", container.NewVBox(t, content))
}

func subsection(title string, objs []fyne.CanvasObject) fyne.CanvasObject {
	t := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return container.NewVBox(append([]fyne.CanvasObject{t}, objs...)...)
}

func lines(texts []string) []fyne.CanvasObject {
	var out []fyne.CanvasObject
	for _, t := range texts {
		out = append(out, wrapped(t))
	}
	return out
}

func numbered(texts []string) []fyne.CanvasObject {
	var out []fyne.CanvasObject
	for i, t := range texts {
		out = append(out, wrapped(fmt.Sprintf("%d.  %s", i+1, t)))
	}
	return out
}

// form lays out label, value pairs in two columns.
func form(pairs ...string) []fyne.CanvasObject {
	if len(pairs) == 0 {
		return nil
	}
	var objs []fyne.CanvasObject
	for i := 0; i+1 < len(pairs); i += 2 {
		objs = append(objs, widget.NewLabelWithStyle(pairs[i], fyne.TextAlignTrailing, fyne.TextStyle{Bold: true}), wrapped(pairs[i+1]))
	}
	return []fyne.CanvasObject{container.New(layout.NewFormLayout(), objs...)}
}
