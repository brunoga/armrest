package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
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
	// full shows the remote with keys alone, filling the screen.
	full func(model string, keys profile.Buttons)
}

// The panes of a device's or activity's view, in order.
const (
	remotePane = iota
	macrosPane
	detailsPane
)

// paneNames name the panes, on the buttons that pick them.
var paneNames = []string{i18n.Mark("Remote"), i18n.Mark("Macros"), i18n.Mark("Details")}

// activityView shows an activity: a remote to drive it with, its macros
// and favorites, and its details (devices, keys, start and end). ctl is
// nil while the activity is not on the hub; with edit, an Edit button
// calls it.
func activityView(a *profile.Activity, model string, ctl *controls, edit func(), deviceTile func(string) fyne.Resource) fyne.CanvasObject {
	var names []string
	for _, m := range a.Devices {
		names = append(names, m.Device)
	}
	head := func() fyne.CanvasObject {
		return withTile(tile(a.Icon, activityIcon, a.Color), heading(a.Name, strings.Join(names, " · ")))
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
		section(i18n.T("Macros"), i18n.T("No macros."), macros),
		section(i18n.T("Favorites"), i18n.T("No favorites."), favorites),
	}

	var devs []fyne.CanvasObject
	for _, m := range a.Devices {
		sub := ""
		if m.Input != "" {
			sub = i18n.T("On %s", m.Input)
		}
		devs = append(devs, newNavRow(deviceTile(m.Device), m.Device, sub, nil))
	}
	details := []fyne.CanvasObject{
		section(i18n.T("Devices"), i18n.T("No devices."), devs),
		keysSection(a.Buttons),
		section(i18n.T("When it starts"), i18n.T("Nothing."), numbered(steps(a.PowerOn))),
		section(i18n.T("When it ends"), i18n.T("Nothing."), numbered(steps(a.PowerOff))),
	}
	return views(head, edit, fullScreen(ctl, model, a.Buttons), remote(model, a.Buttons, ctl), lists, details)
}

// deviceView shows a device: a remote to drive it with, its macros, and
// its details (commands, each with a Test button, keys, power, inputs).
// ctl is nil while the device is not on the hub; with edit, an Edit
// button calls it, and with export, a button under its commands saves
// their codes to a file.
func deviceView(d *profile.Device, model string, ctl *controls, edit, export func()) fyne.CanvasObject {
	sub := d.Brand
	// The kind, unless the icon says it already.
	if !d.Category.IsZero() && d.Category.String() != d.Icon.String() {
		if sub != "" {
			sub += " · "
		}
		sub += d.Category.String()
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
	lists := []fyne.CanvasObject{section(i18n.T("Macros"), i18n.T("No macros."), macros)}

	var cmds []fyne.CanvasObject
	for _, c := range d.Commands {
		name := wrapped(c.Name)
		if ctl == nil {
			cmds = append(cmds, name)
			continue
		}
		test := widget.NewButtonWithIcon(i18n.T("Test"), theme.MediaPlayIcon(), func() { ctl.send(d.Name, c.Name) })
		cmds = append(cmds, container.NewBorder(nil, nil, nil, test, name))
	}
	if ctl != nil && len(cmds) > 0 {
		hint := wrapped(i18n.T("Test sends a command from the hub, to check that the device responds."))
		hint.TextStyle.Italic = true
		cmds = append([]fyne.CanvasObject{quiet(hint)}, cmds...)
	}
	var power []fyne.CanvasObject
	if d.Power != nil {
		power = form(i18n.T("On"), d.Power.On, i18n.T("Off"), d.Power.Off)
	}
	inputs := i18n.T("Not set.")
	if d.Inputs != nil {
		inputs = i18n.T("It has no inputs.")
	}
	details := []fyne.CanvasObject{
		section(i18n.T("Commands (%d)", len(d.Commands)), i18n.T("No commands."), cmds),
	}
	if export != nil {
		details = append(details, container.NewHBox(widget.NewButtonWithIcon(i18n.T("Save codes to a file…"), theme.DocumentSaveIcon(), export)))
	}
	details = append(details,
		keysSection(d.Buttons),
		section(i18n.T("Power"), i18n.T("Not set: activities turn it on and off with its own power macros."), power),
		section(i18n.T("Inputs"), inputs, numbered(d.Inputs)),
	)
	return views(head, edit, fullScreen(ctl, model, d.Buttons), remote(model, d.Buttons, ctl), lists, details)
}

// runnable is a macro or favorite named name, described by about, with a
// Run button where ctl drives it.
func runnable(name string, about []fyne.CanvasObject, ctl *controls, run func()) fyne.CanvasObject {
	title := widget.NewLabelWithStyle(name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Wrapping = fyne.TextWrapWord
	var top fyne.CanvasObject = title
	if ctl != nil {
		top = container.NewBorder(nil, nil, nil, widget.NewButtonWithIcon(i18n.T("Run"), theme.MediaPlayIcon(), run), title)
	}
	return container.NewVBox(append([]fyne.CanvasObject{top}, about...)...)
}

// remote draws the remote with the keys set in keys, which press them
// on the hub where ctl drives it: a tap presses a key, a long press (a
// right click) holds it.
func remote(model string, keys profile.Buttons, ctl *controls) []fyne.CanvasObject {
	hint := i18n.T("Click a key to press it; right-click it to hold it.")
	if fyne.CurrentDevice().IsMobile() {
		hint = i18n.T("Tap a key to press it; long-press it to hold it.")
	}
	if ctl == nil {
		hint = i18n.T("The remote works once this is on the hub: review and apply your changes, connected to it.")
	}
	note := wrapped(hint)
	note.TextStyle.Italic = true
	drawing := drawRemote(model, func(k string) bool { _, ok := keys[k]; return ok }, func(k, label string) fyne.CanvasObject {
		return newRemoteKey(k, label, keys[k], ctl)
	})
	return []fyne.CanvasObject{note, container.New(&centerLayout{width: PadWidth * Scale()}, drawing)}
}

// remoteKey is a key of the remote drawn: tapped it presses the key on
// the hub; held down for holdAfter it holds the key there and then, as
// the remote does (a right click holds it too). Keys that do nothing are
// disabled.
type remoteKey struct {
	padKey
	hold func()

	// pressed is set while a finger or mouse button holds the key down;
	// timer holds the key once it has been down for holdAfter, and held
	// is set once it did, so that letting go does not press it too.
	pressed bool
	timer   *time.Timer
	held    bool

	// lit is set while the key shows held down, since when, over its
	// usual look.
	lit   bool
	since time.Time
	usual widget.Importance
}

// afterFunc calls f after d, from its own goroutine; tests do without.
var afterFunc = time.AfterFunc

// holdAfter is how long a key is down before it is held, as long as a
// phone's long press.
const holdAfter = 500 * time.Millisecond

// litFor is the least a key shows held down: a quick tap shows too.
const litFor = 150 * time.Millisecond

// down shows the key held down, in the app's orange, and starts timing
// the hold.
func (k *remoteKey) down() {
	if k.Disabled() || k.pressed {
		return
	}
	k.pressed, k.held = true, false
	if k.hold != nil {
		k.timer = afterFunc(holdAfter, func() { fyne.Do(k.holding) })
	}
	if k.lit {
		return
	}
	k.lit, k.since, k.usual = true, time.Now(), k.Importance
	k.Importance = widget.WarningImportance
	k.Refresh()
}

// holding holds the key, if it is still down.
func (k *remoteKey) holding() {
	if k.pressed && !k.held && k.hold != nil {
		k.held = true
		k.hold()
	}
}

// up shows the key let go, once it has shown held down for litFor.
func (k *remoteKey) up() {
	if !k.pressed {
		return
	}
	k.pressed = false
	if k.timer != nil {
		k.timer.Stop()
		k.timer = nil
	}
	done := func() {
		k.lit, k.Importance = false, k.usual
		k.Refresh()
	}
	if wait := litFor - time.Since(k.since); wait > 0 {
		afterFunc(wait, func() { fyne.Do(done) })
		return
	}
	done()
}

// The key is held down by a finger, or a mouse button.
func (k *remoteKey) TouchDown(*mobile.TouchEvent)   { k.down() }
func (k *remoteKey) TouchUp(*mobile.TouchEvent)     { k.up() }
func (k *remoteKey) TouchCancel(*mobile.TouchEvent) { k.up() }
func (k *remoteKey) MouseDown(*desktop.MouseEvent)  { k.down() }
func (k *remoteKey) MouseUp(*desktop.MouseEvent)    { k.up() }

func newRemoteKey(key, label string, set profile.Key, ctl *controls) *remoteKey {
	k := &remoteKey{}
	k.init(key, label, nil)
	if set.Short != "" {
		k.Importance = widget.HighImportance
	}
	if ctl != nil {
		k.OnTapped = func() {
			tapFeedback()
			ctl.press(key, false)
		}
		// Held, every key answers, if only to say holding it does
		// nothing.
		k.hold = func() {
			tapFeedback()
			ctl.press(key, true)
		}
	}
	k.ExtendBaseWidget(k)
	if ctl == nil || set.Short == "" {
		k.Disable()
	}
	return k
}

// Tapped presses the key, unless it was held while down.
func (k *remoteKey) Tapped(e *fyne.PointEvent) {
	if k.held {
		k.held = false
		return
	}
	k.padKey.Tapped(e)
}

// TappedSecondary is a right click, which holds the key; on phones, a
// touch let go after Fyne's long press (shorter than holdAfter) but
// before holdAfter, which presses it.
func (k *remoteKey) TappedSecondary(*fyne.PointEvent) {
	switch {
	case k.held:
		k.held = false
	case k.Disabled():
	case fyne.CurrentDevice().IsMobile():
		if k.OnTapped != nil {
			k.OnTapped()
		}
	case k.hold != nil:
		k.hold()
	}
}

// views shows one of its panes at a time, each under a head, picked by a
// row of buttons that stays put, with Edit calling edit if it is set: the
// remote first.
func views(head func() fyne.CanvasObject, edit, full func(), panes ...[]fyne.CanvasObject) fyne.CanvasObject {
	pages := make([]fyne.CanvasObject, len(panes))
	names := make([]string, len(panes))
	show := func(i int) {
		for j, p := range pages {
			if j == i {
				p.Show()
			} else {
				p.Hide()
			}
		}
	}
	for i, p := range panes {
		pages[i] = page(append([]fyne.CanvasObject{head()}, p...))
		names[i] = i18n.T(paneNames[i])
	}
	show(remotePane)
	var row fyne.CanvasObject = newSegmented(names, remotePane, show)
	// Icons alone: more names across do not fit a phone.
	right := container.NewHBox()
	if full != nil {
		right.Add(newIconButton(i18n.T("Full screen"), theme.ViewFullScreenIcon(), full))
	}
	if edit != nil {
		b := newIconButton(i18n.T("Edit"), theme.DocumentCreateIcon(), edit)
		b.Importance = widget.HighImportance
		right.Add(b)
	}
	row = container.NewBorder(nil, nil, nil, right, row)
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
	return section(i18n.T("Keys"), i18n.T("No keys set."), form(rows...))
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

// section is a card of objs, a rule between each, under a small title;
// of the empty text if there are none.
func section(title, empty string, objs []fyne.CanvasObject) fyne.CanvasObject {
	if len(objs) == 0 {
		l := wrapped(empty)
		l.TextStyle.Italic = true
		objs = []fyne.CanvasObject{quiet(l)}
	}
	return group(title, objs...)
}

// card is a card under a title that wraps, for titles that hold names.
func card(title string, content fyne.CanvasObject) fyne.CanvasObject {
	return titled(title, content)
}

// titled is a card of content under a small, muted title: the app's
// sections, as group's.
func titled(title string, content fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(groupTitle(title), widget.NewCard("", "", content))
}

func lines(texts []string) []fyne.CanvasObject {
	var out []fyne.CanvasObject
	for _, t := range texts {
		out = append(out, wrapped(t))
	}
	return out
}

// numbered are texts, each beside its number, muted.
func numbered(texts []string) []fyne.CanvasObject {
	var out []fyne.CanvasObject
	for i, t := range texts {
		n := widget.NewLabelWithStyle(fmt.Sprint(i+1), fyne.TextAlignTrailing, fyne.TextStyle{Bold: true})
		out = append(out, container.NewBorder(nil, nil, quiet(n), nil, wrapped(t)))
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

// fullScreen shows the remote with keys alone, filling the screen, where
// ctl drives it; nil otherwise.
func fullScreen(ctl *controls, model string, keys profile.Buttons) func() {
	if ctl == nil || ctl.full == nil {
		return nil
	}
	return func() { ctl.full(model, keys) }
}
