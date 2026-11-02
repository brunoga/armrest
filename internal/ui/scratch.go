package ui

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/workspace"
)

// CodeWidth is the width of a command's code entry.
const CodeWidth = 96

// scratchScreen makes a device from its IR codes.
type scratchScreen struct {
	app *App

	name, brand *widget.Entry
	protocol    *widget.RadioGroup
	about       *widget.Label
	system      *widget.Entry
	device, sub *widget.Entry
	fields      *fyne.Container

	rows    []*commandRow
	list    *fyne.Container
	powerOn *widget.Select
	powerOf *widget.Select
	problem *widget.Label
}

type commandRow struct {
	name *widget.SelectEntry
	code *widget.Entry
	box  fyne.CanvasObject
}

func (a *App) showScratch() {
	s := &scratchScreen{app: a}
	a.screen("New device", a.showAddDevice, container.NewVScroll(column(container.NewPadded(s.content()))))
}

func (s *scratchScreen) content() fyne.CanvasObject {
	s.name = widget.NewEntry()
	s.name.SetPlaceHolder("Living room TV")
	s.brand = widget.NewEntry()
	s.brand.SetPlaceHolder("Optional")
	device := widget.NewCard("Device", "", widget.NewForm(
		widget.NewFormItem("Name", s.name),
		widget.NewFormItem("Brand", s.brand)))

	var titles []string
	for _, p := range workspace.Protocols {
		titles = append(titles, p.Title)
	}
	s.about = wrapped("")
	s.system = widget.NewEntry()
	s.system.SetPlaceHolder("0x00FF")
	s.device = widget.NewEntry()
	s.device.SetPlaceHolder("4")
	s.sub = widget.NewEntry()
	s.sub.SetPlaceHolder("1")
	s.fields = container.NewVBox()
	s.protocol = widget.NewRadioGroup(titles, func(string) { s.showProtocol() })
	s.protocol.SetSelected(workspace.Protocols[0].Title)
	codes := widget.NewCard("Codes", "", container.NewVBox(
		wrapped("How does the device's remote send its codes? The device's manual or IR code chart says; NEC is the most common."),
		s.protocol, s.about, s.fields))

	s.list = container.NewVBox()
	add := widget.NewButtonWithIcon("Add a command", theme.ContentAddIcon(), func() { s.addRow("", "") })
	s.addRow("", "")
	commands := widget.NewCard("Commands", "", container.NewVBox(
		wrapped("A name and a code for each. Commands named like a key of the remote (OK, Up, Volume Up…) are put on that key."),
		s.list, container.NewHBox(add)))

	s.powerOn = widget.NewSelect(nil, nil)
	s.powerOf = widget.NewSelect(nil, nil)
	s.powerOn.PlaceHolder = "None"
	s.powerOf.PlaceHolder = "None"
	s.refreshPower()
	power := widget.NewCard("Power", "", container.NewVBox(
		wrapped("The commands activities use to turn the device on and off. Pick the same one twice if it has a single power key."),
		widget.NewForm(widget.NewFormItem("On", s.powerOn), widget.NewFormItem("Off", s.powerOf))))

	s.problem = wrapped("")
	s.problem.Importance = widget.DangerImportance
	s.problem.Hide()
	create := widget.NewButtonWithIcon("Add the device", theme.ConfirmIcon(), s.create)
	create.Importance = widget.HighImportance
	note := wrapped("Nothing is written to the hub until you review and apply your changes. Once it is on the hub, you can test each command.")
	note.TextStyle.Italic = true

	return container.NewVBox(device, codes, commands, power, s.problem, create, note)
}

// showProtocol shows the fields the picked protocol needs.
func (s *scratchScreen) showProtocol() {
	p := s.picked()
	s.about.SetText(p.About)
	switch p.ID {
	case "NEC":
		s.fields.Objects = []fyne.CanvasObject{widget.NewForm(widget.NewFormItem("System code", s.system))}
	case "DenonK":
		s.fields.Objects = []fyne.CanvasObject{widget.NewForm(widget.NewFormItem("Device", s.device), widget.NewFormItem("Subdevice", s.sub))}
	default:
		s.fields.Objects = []fyne.CanvasObject{widget.NewForm(widget.NewFormItem("Device", s.device))}
	}
	s.fields.Refresh()
}

func (s *scratchScreen) picked() workspace.Protocol {
	for _, p := range workspace.Protocols {
		if p.Title == s.protocol.Selected {
			return p
		}
	}
	return workspace.Protocols[0]
}

// addRow adds a command row.
func (s *scratchScreen) addRow(name, code string) {
	r := &commandRow{name: widget.NewSelectEntry(workspace.CommonCommands), code: widget.NewEntry()}
	r.name.SetPlaceHolder("Name")
	r.name.SetText(name)
	r.name.OnChanged = func(string) { s.refreshPower() }
	r.code.SetPlaceHolder("Code")
	r.code.SetText(code)
	remove := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() { s.removeRow(r) })
	r.box = container.NewBorder(nil, nil, nil,
		container.NewHBox(container.New(layout.NewGridWrapLayout(fyne.NewSize(CodeWidth, r.code.MinSize().Height)), r.code), remove),
		r.name)
	s.rows = append(s.rows, r)
	s.list.Add(r.box)
}

func (s *scratchScreen) removeRow(r *commandRow) {
	for i, o := range s.rows {
		if o == r {
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
		}
	}
	s.list.Remove(r.box)
	s.refreshPower()
}

// refreshPower offers the commands' names for power on and off.
func (s *scratchScreen) refreshPower() {
	if s.powerOn == nil {
		return
	}
	var names []string
	for _, r := range s.rows {
		if n := strings.TrimSpace(r.name.Text); n != "" {
			names = append(names, n)
		}
	}
	options := append([]string{"None"}, names...)
	for _, sel := range []*widget.Select{s.powerOn, s.powerOf} {
		sel.Options = options
		if sel.Selected != "" && !contains(options, sel.Selected) {
			sel.ClearSelected()
		}
		sel.Refresh()
	}
	// Commands named for it are the usual pick.
	if s.powerOn.Selected == "" && contains(names, "Power On") && contains(names, "Power Off") {
		s.powerOn.SetSelected("Power On")
		s.powerOf.SetSelected("Power Off")
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (s *scratchScreen) spec() workspace.Spec {
	sp := workspace.Spec{
		Name: s.name.Text, Brand: s.brand.Text, Protocol: s.picked().ID,
		System: s.system.Text, Device: s.device.Text, Subdevice: s.sub.Text,
	}
	for _, r := range s.rows {
		sp.Commands = append(sp.Commands, workspace.CommandSpec{Name: r.name.Text, Code: r.code.Text})
	}
	for _, x := range []struct {
		sel *widget.Select
		to  *string
	}{{s.powerOn, &sp.PowerOn}, {s.powerOf, &sp.PowerOff}} {
		if x.sel.Selected != "None" {
			*x.to = x.sel.Selected
		}
	}
	return sp
}

// create adds the device to the workspace, or says what is wrong.
func (s *scratchScreen) create() {
	d, err := s.spec().Build()
	if err == nil {
		err = s.app.work.AddDevice(d)
	}
	if err != nil {
		var fe *workspace.FieldError
		if errors.As(err, &fe) && fe.Field == "command" && fe.Index < len(s.rows) {
			s.app.win.Canvas().Focus(s.rows[fe.Index].code)
		}
		s.problem.SetText(err.Error())
		s.problem.Show()
		return
	}
	keys := keyNames(d.Buttons)
	msg := fmt.Sprintf("Added %s.", d.Name)
	if len(keys) > 0 {
		msg += fmt.Sprintf(" Keys set from command names: %s.", strings.Join(keys, ", "))
	}
	b := s.app.showBrowse(s.app.hub)
	b.selectDevice(d.Name)
	b.setStatus(false, msg+" Review your changes to write them to the hub.")
}
