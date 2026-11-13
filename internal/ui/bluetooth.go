package ui

import (
	"errors"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/armrest/internal/workspace"
)

// bluetoothScreen makes a Bluetooth device: one the hub pairs with and
// presses keys of, as a keyboard and remote would.
type bluetoothScreen struct {
	app *App

	name, brand *widget.Entry
	classic     *widget.Check
	rows        []*hidRow
	list        *fyne.Container
	powerOn     *widget.Select
	powerOf     *widget.Select
	problem     *widget.Label
}

type hidRow struct {
	name *widget.Entry
	key  *widget.Select
	box  fyne.CanvasObject
}

func (a *App) showBluetooth() {
	s := &bluetoothScreen{app: a}
	a.screen(i18n.T("New Bluetooth device"), a.showAddDevice, scrolled(s.content()))
}

// hidKeyNames are the names of the keys a command can press.
func hidKeyNames() []string {
	names := make([]string, len(workspace.HIDKeys))
	for i, k := range workspace.HIDKeys {
		names[i] = k.Name
	}
	return names
}

// newKeySelect picks the key a command presses, usage at first: a HID
// usage not among workspace.HIDKeys is offered as itself. The command's
// name entry, if any, shows the key's name as its placeholder.
func newKeySelect(usage string, name *widget.Entry) *widget.Select {
	names := hidKeyNames()
	sel := widget.NewSelect(names, func(k string) {
		if name != nil {
			name.SetPlaceHolder(k)
		}
	})
	sel.PlaceHolder = i18n.T("Key")
	if usage == "" {
		return sel
	}
	if k, ok := workspace.HIDKeyFor(usage); ok {
		sel.SetSelected(k.Name)
	} else {
		sel.Options = append(names, usage)
		sel.SetSelected(usage)
	}
	return sel
}

// keyUsage is the HID usage of the key named name, which may be a usage
// itself (see newKeySelect); empty for none.
func keyUsage(name string) string {
	if k, ok := workspace.HIDKeyNamed(name); ok {
		return k.Usage
	}
	return name
}

func (s *bluetoothScreen) content() fyne.CanvasObject {
	s.name = widget.NewEntry()
	s.name.SetPlaceHolder(i18n.T("Living room TV box"))
	s.brand = widget.NewEntry()
	s.brand.SetPlaceHolder(i18n.T("Optional"))
	s.classic = widget.NewCheck(i18n.T("Classic Bluetooth"), nil)
	classicNote := wrapped(i18n.T("Most devices pair over Bluetooth Low Energy; a few, as older Apple TVs and the PS4, over classic Bluetooth."))
	classicNote.TextStyle.Italic = true
	device := titled(i18n.T("Device"), container.NewVBox(
		wrapped(i18n.T("A device the hub pairs with over Bluetooth, as a keyboard and remote: a TV box, a streaming stick, a projector or a console. Search for its model first: the catalog may have it.")),
		widget.NewForm(
			widget.NewFormItem(i18n.T("Name"), s.name),
			widget.NewFormItem(i18n.T("Brand"), s.brand)),
		s.classic, classicNote))

	s.list = container.NewVBox()
	for _, k := range workspace.UsualBluetoothKeys {
		s.addRow(k)
	}
	add := widget.NewButtonWithIcon(i18n.T("Add a command"), theme.ContentAddIcon(), func() { s.addRow("") })
	commands := titled(i18n.T("Commands"), container.NewVBox(
		wrapped(i18n.T("The key each command presses, and its name: the key's, unless you give another. Commands named like a key of the remote (OK, Up, Volume Up…) are put on that key.")+" "+i18n.T(dragHint)),
		s.list, container.NewHBox(add)))

	s.powerOn = widget.NewSelect(nil, nil)
	s.powerOf = widget.NewSelect(nil, nil)
	s.powerOn.PlaceHolder = i18n.T("None")
	s.powerOf.PlaceHolder = i18n.T("None")
	s.refreshPower()
	power := titled(i18n.T("Power"), container.NewVBox(
		wrapped(i18n.T("The commands activities use to turn the device on and off. Pick the same one twice if it has a single power key. Many devices do not wake over Bluetooth: leave them as None for those.")),
		widget.NewForm(widget.NewFormItem(i18n.T("On"), s.powerOn), widget.NewFormItem(i18n.T("Off"), s.powerOf))))

	s.problem = wrapped("")
	s.problem.Importance = widget.DangerImportance
	s.problem.Hide()
	create := widget.NewButtonWithIcon(i18n.T("Add the device"), theme.ConfirmIcon(), s.create)
	create.Importance = widget.HighImportance
	note := wrapped(i18n.T("Nothing is written to the hub until you review and apply your changes. Applying then has the hub pair: pick \"Sofabaton\" in the device's Bluetooth settings when asked."))
	note.TextStyle.Italic = true
	return container.NewVBox(device, commands, power, s.problem, create, note)
}

// addRow adds a command row pressing key, none if empty.
func (s *bluetoothScreen) addRow(key string) {
	r := &hidRow{name: widget.NewEntry()}
	r.name.OnChanged = func(string) { s.refreshPower() }
	r.name.SetPlaceHolder(i18n.T("Name"))
	r.key = newKeySelect("", r.name)
	r.key.OnChanged = func(k string) {
		r.name.SetPlaceHolder(k)
		s.refreshPower()
	}
	if key != "" {
		r.key.SetSelected(key)
	}
	remove := newIconButton(i18n.T("Delete"), theme.DeleteIcon(), func() { s.removeRow(r) })
	grip := newDragHandle(s.list, func(i, j int) { s.rows[i], s.rows[j] = s.rows[j], s.rows[i] }, s.refreshPower)
	r.box = container.NewBorder(nil, nil, grip, remove, container.NewGridWithColumns(2, r.key, r.name))
	grip.row = r.box
	s.rows = append(s.rows, r)
	s.list.Add(r.box)
}

func (s *bluetoothScreen) removeRow(r *hidRow) {
	for i, o := range s.rows {
		if o == r {
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
		}
	}
	s.list.Remove(r.box)
	s.refreshPower()
}

// commandName is the row's command name: its own, else its key's.
func (r *hidRow) commandName() string {
	if n := strings.TrimSpace(r.name.Text); n != "" {
		return n
	}
	return r.key.Selected
}

// refreshPower offers the commands' names for power on and off.
func (s *bluetoothScreen) refreshPower() {
	if s.powerOn == nil {
		return
	}
	var names []string
	for _, r := range s.rows {
		if n := r.commandName(); n != "" {
			names = append(names, n)
		}
	}
	options := append([]string{i18n.T("None")}, names...)
	for _, sel := range []*widget.Select{s.powerOn, s.powerOf} {
		sel.Options = options
		if sel.Selected != "" && !contains(options, sel.Selected) {
			sel.ClearSelected()
		}
		sel.Refresh()
	}
}

func (s *bluetoothScreen) spec() workspace.BluetoothSpec {
	sp := workspace.BluetoothSpec{Name: s.name.Text, Brand: s.brand.Text, Classic: s.classic.Checked}
	for _, r := range s.rows {
		sp.Commands = append(sp.Commands, workspace.BluetoothCommand{Name: r.name.Text, Key: r.key.Selected})
	}
	for _, x := range []struct {
		sel *widget.Select
		to  *string
	}{{s.powerOn, &sp.PowerOn}, {s.powerOf, &sp.PowerOff}} {
		if x.sel.Selected != i18n.T("None") {
			*x.to = x.sel.Selected
		}
	}
	return sp
}

// create adds the device to the workspace, or says what is wrong.
func (s *bluetoothScreen) create() {
	d, err := s.spec().Build()
	if err == nil {
		err = s.app.work.AddDevice(d)
	}
	if err != nil {
		var fe *workspace.FieldError
		if errors.As(err, &fe) && fe.Field == "command" && fe.Index < len(s.rows) {
			s.app.win.Canvas().Focus(s.rows[fe.Index].name)
		}
		s.problem.SetText(err.Error())
		s.problem.Show()
		return
	}
	keys := keyNames(d.Buttons)
	msg := i18n.T("Added %s.", d.Name)
	if len(keys) > 0 {
		msg += " " + i18n.T("Keys set from command names: %s.", strings.Join(keys, ", "))
	}
	b := s.app.showBrowse(s.app.hub)
	b.selectDevice(d.Name)
	b.setStatus(false, msg)
}
