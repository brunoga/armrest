package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/sofabaton/profile"
)

// controls drive a device or activity on the hub, as the hub has it.
type controls struct {
	// press presses one of its keys, or holds it.
	press func(key string, hold bool)
	// run runs one of its macros.
	run func(macro string)
	// send sends a device's command.
	send func(device, command string)
}

// The panes of a device's or activity's view, in order.
const (
	remotePane = iota
	macrosPane
	detailsPane
)

// paneNames name the panes, on the buttons that pick them.
var paneNames = []string{"Remote", "Macros", "Details"}

// activityView shows an activity: a remote to drive it with, its macros
// and favorites, and its details (devices, keys, start and end). ctl is
// nil while the activity is not on the hub; with edit, an Edit button
// calls it.
func activityView(a *profile.Activity, model string, ctl *controls, edit func()) fyne.CanvasObject {
	head := func() fyne.CanvasObject {
		return withTile(tile(a.Icon, activityIcon, a.Color), heading(a.Name, describeLook(a.Icon, a.Color)))
	}

	steps := func(ss []profile.Step) []string {
		var out []string
		for _, s := range ss {
			out = append(out, describeStep(s))
		}
		return out
	}
	var macros, favorites []fyne.CanvasObject
	for _, m := range a.Macros {
		macros = append(macros, runnable(m.Name, numbered(steps(m.Steps)), ctl, func() { ctl.run(m.Name) }))
	}
	for _, f := range a.Favorites {
		dev, cmd, _ := strings.Cut(f, ": ")
		favorites = append(favorites, runnable(f, nil, ctl, func() { ctl.send(dev, cmd) }))
	}
	lists := []fyne.CanvasObject{
		section("Macros", "No macros.", macros),
		section("Favorites", "No favorites.", favorites),
	}

	var devs []string
	for _, m := range a.Devices {
		if m.Input != "" {
			devs = append(devs, fmt.Sprintf("%s, on %s", m.Device, m.Input))
		} else {
			devs = append(devs, m.Device)
		}
	}
	details := []fyne.CanvasObject{
		section("Devices", "No devices.", lines(devs)),
		keysSection(a.Buttons),
		section("When it starts", "Nothing.", numbered(steps(a.PowerOn))),
		section("When it ends", "Nothing.", numbered(steps(a.PowerOff))),
	}
	return views(head, edit, remote(model, a.Buttons, ctl), lists, details)
}

// deviceView shows a device: a remote to drive it with, its macros, and
// its details (commands, each with a Test button, keys, power, inputs).
// ctl is nil while the device is not on the hub; with edit, an Edit
// button calls it.
func deviceView(d *profile.Device, model string, ctl *controls, edit func()) fyne.CanvasObject {
	sub := d.Brand
	if look := describeLook(d.Icon, d.Color); look != "" {
		if sub != "" {
			sub += " · "
		}
		sub += look
	}
	head := func() fyne.CanvasObject { return withTile(tile(d.Icon, deviceIcon, d.Color), heading(d.Name, sub)) }

	var macros []fyne.CanvasObject
	for _, m := range d.Macros {
		var steps []string
		for _, s := range m.Steps {
			steps = append(steps, describeDeviceStep(s))
		}
		macros = append(macros, runnable(m.Name, numbered(steps), ctl, func() { ctl.run(m.Name) }))
	}
	lists := []fyne.CanvasObject{section("Macros", "No macros.", macros)}

	var cmds []fyne.CanvasObject
	for _, c := range d.Commands {
		name := wrapped(c.Name)
		if ctl == nil {
			cmds = append(cmds, name)
			continue
		}
		test := widget.NewButtonWithIcon("Test", theme.MediaPlayIcon(), func() { ctl.send(d.Name, c.Name) })
		cmds = append(cmds, container.NewBorder(nil, nil, nil, test, name))
	}
	if ctl != nil && len(cmds) > 0 {
		hint := wrapped("Test sends a command from the hub, to check that the device responds.")
		hint.TextStyle.Italic = true
		cmds = append([]fyne.CanvasObject{hint}, cmds...)
	}
	var power []fyne.CanvasObject
	if d.Power != nil {
		power = form("On", d.Power.On, "Off", d.Power.Off)
	}
	inputs := "Not set."
	if d.Inputs != nil {
		inputs = "It has no inputs."
	}
	details := []fyne.CanvasObject{
		section(fmt.Sprintf("Commands (%d)", len(d.Commands)), "No commands.", cmds),
		keysSection(d.Buttons),
		section("Power", "Not set: activities turn it on and off with its own power macros.", power),
		section("Inputs", inputs, numbered(d.Inputs)),
	}
	return views(head, edit, remote(model, d.Buttons, ctl), lists, details)
}

// runnable is a macro or favorite named name, described by about, with a
// Run button where ctl drives it.
func runnable(name string, about []fyne.CanvasObject, ctl *controls, run func()) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Wrapping = fyne.TextWrapWord
	var top fyne.CanvasObject = title
	if ctl != nil {
		top = container.NewBorder(nil, nil, nil, widget.NewButtonWithIcon("Run", theme.MediaPlayIcon(), run), title)
	}
	return container.NewVBox(append([]fyne.CanvasObject{top}, about...)...)
}

// remote draws the remote with the keys set in keys, which press them
// on the hub where ctl drives it: a tap presses a key, a long press (a
// right click) holds it.
func remote(model string, keys profile.Buttons, ctl *controls) []fyne.CanvasObject {
	hint := "Click a key to press it; right-click it to hold it."
	if fyne.CurrentDevice().IsMobile() {
		hint = "Tap a key to press it; long-press it to hold it."
	}
	if ctl == nil {
		hint = "The remote works once this is on the hub: review and apply your changes, connected to it."
	}
	note := wrapped(hint)
	note.TextStyle.Italic = true
	drawing := drawRemote(model, func(k string) bool { _, ok := keys[k]; return ok }, func(k, label string) fyne.CanvasObject {
		return newRemoteKey(k, label, keys[k], ctl)
	})
	return []fyne.CanvasObject{note, container.New(&centerLayout{width: PadWidth * Scale()}, drawing)}
}

// remoteKey is a key of the remote drawn: tapped it presses the key on
// the hub, tapped with the second button (a long press on phones) holds
// it. Keys that do nothing are disabled.
type remoteKey struct {
	widget.Button
	hold func()
}

func newRemoteKey(key, label string, set profile.Key, ctl *controls) *remoteKey {
	k := &remoteKey{}
	k.Text = label
	if set.Short != "" {
		k.Importance = widget.HighImportance
	}
	if ctl != nil {
		k.OnTapped = func() { ctl.press(key, false) }
		if set.Long != "" {
			k.hold = func() { ctl.press(key, true) }
		}
	}
	k.ExtendBaseWidget(k)
	if ctl == nil || set.Short == "" {
		k.Disable()
	}
	return k
}

// TappedSecondary holds the key.
func (k *remoteKey) TappedSecondary(*fyne.PointEvent) {
	if k.hold != nil && !k.Disabled() {
		k.hold()
	}
}

// views shows one of its panes at a time, each under a head, picked by a
// row of buttons that stays put, with Edit calling edit if it is set: the
// remote first.
func views(head func() fyne.CanvasObject, edit func(), panes ...[]fyne.CanvasObject) fyne.CanvasObject {
	pages := make([]fyne.CanvasObject, len(panes))
	pick := make([]*widget.Button, len(panes))
	show := func(i int) {
		for j := range panes {
			if j == i {
				pages[j].Show()
			} else {
				pages[j].Hide()
			}
			pick[j].Importance = widget.MediumImportance
			if j == i {
				pick[j].Importance = widget.HighImportance
			}
			pick[j].Refresh()
		}
	}
	for i, p := range panes {
		pages[i] = page(append([]fyne.CanvasObject{head()}, p...))
		pick[i] = widget.NewButton(paneNames[i], func() { show(i) })
	}
	show(remotePane)
	bar := container.NewGridWithColumns(len(pick))
	for _, b := range pick {
		bar.Add(b)
	}
	var row fyne.CanvasObject = bar
	if edit != nil {
		// A pencil alone: four names across do not fit a phone.
		b := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), edit)
		b.Importance = widget.HighImportance
		row = container.NewBorder(nil, nil, nil, b, bar)
	}
	row = container.NewThemeOverride(row, tightKeys{})
	p, g := theme.Padding(), gutter()
	top := column(container.New(layout.NewCustomPaddedLayout(p, p, g, g), row))
	return container.NewBorder(top, nil, nil, nil, container.NewStack(pages...))
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
	return scrolled(container.NewVBox(parts...))
}

// withTile puts a device's or activity's tile left of its heading.
func withTile(t fyne.Resource, heading fyne.CanvasObject) fyne.CanvasObject {
	img := canvas.NewImageFromResource(t)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSquareSize(headingTile * Scale()))
	return container.NewBorder(nil, nil, container.NewCenter(img), nil, heading)
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
