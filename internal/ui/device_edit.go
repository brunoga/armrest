package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/workspace"
	"github.com/brunoga/sofabaton/profile"
	"github.com/brunoga/sofabaton/record"
)

// none is the choice of no command in a picker.
const none = "Nothing"

// deviceEditor edits a device of the workspace.
type deviceEditor struct {
	app *App
	e   workspace.DeviceEdit

	name, brand *widget.Entry
	icon, color *widget.Select

	cmds    []*cmdRow
	cmdList *fyne.Container

	pad *keyPad

	powerOn, powerOff *widget.Select

	inputs    []*widget.Select
	inputList *fyne.Container

	macros    []*macroBox
	macroList *fyne.Container

	// pickers are every command picker, refreshed when commands change;
	// keyPickers offer the macros too.
	pickers    []*widget.Select
	keyPickers []*widget.Select
	problem    *widget.Label
}

// macroBox is a macro in the editor.
type macroBox struct {
	original string
	name     *widget.Entry
	steps    []*stepRow
	list     *fyne.Container
	box      fyne.CanvasObject
	last     string
}

// stepRow is a macro step: a command picker, or a wait in seconds.
type stepRow struct {
	cmd  *widget.Select
	wait *widget.Entry
	hold uint8
	box  fyne.CanvasObject
}

type cmdRow struct {
	edit workspace.CommandEdit
	name *widget.Entry
	code *widget.Entry
	box  fyne.CanvasObject
	// last is the name last seen, to follow renames in the pickers.
	last string
}

func (a *App) showEditDevice(name string) {
	e, err := a.work.EditDevice(name)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	ed := &deviceEditor{app: a, e: e}
	a.screen("Edit "+e.Name, a.backToBrowse, container.NewVScroll(column(container.NewPadded(ed.content()))))
}

func (ed *deviceEditor) content() fyne.CanvasObject {
	e := ed.e
	ed.name = widget.NewEntry()
	ed.name.SetText(e.Name)
	ed.brand = widget.NewEntry()
	ed.brand.SetText(e.Brand)
	ed.brand.SetPlaceHolder("Optional")
	ed.icon = widget.NewSelect(iconChoices(), nil)
	ed.icon.SetSelected(lookChoice(e.Icon.String(), e.Icon.IsZero()))
	ed.color = widget.NewSelect(colorChoices(), nil)
	ed.color.SetSelected(lookChoice(e.Color.String(), e.Color.IsZero()))
	device := widget.NewCard("Device", "", widget.NewForm(
		widget.NewFormItem("Name", ed.name),
		widget.NewFormItem("Brand", ed.brand),
		widget.NewFormItem("Icon", ed.icon),
		widget.NewFormItem("Color", ed.color)))

	ed.problem = wrapped("")
	ed.problem.Importance = widget.DangerImportance
	ed.problem.Hide()

	// Commands come first: the pickers below offer them.
	ed.cmdList = container.NewVBox()
	for _, c := range e.Commands {
		ed.addCmd(c)
	}
	add := widget.NewButtonWithIcon("Add a command", theme.ContentAddIcon(), func() {
		ed.addCmd(workspace.CommandEdit{})
		ed.refreshPickers()
	})
	var addNote fyne.CanvasObject = container.NewHBox(add)
	if !ed.app.work.CanAddCommands(e.Original) {
		add.Disable()
		n := wrapped("This device has no IR protocol (its commands were learned), so new commands cannot be given as codes.")
		n.TextStyle.Italic = true
		addNote = container.NewVBox(n, container.NewHBox(add))
	}
	commands := widget.NewCard("Commands", "", container.NewVBox(
		wrapped("In the order the remote lists them. Renaming a command renames it in the activities too."),
		ed.cmdList, addNote))

	ed.pad = newKeyPad(ed.app.win, ed.app.work.Dump.Info.Model, e.Keys, ed.keyPicker, ed.refreshPickers)
	keys := widget.NewCard("Keys", "", container.NewVBox(
		wrapped("What the remote's keys do while this device's page is shown."),
		ed.pad.box))

	ed.powerOn = ed.picker(e.PowerOn)
	ed.powerOff = ed.picker(e.PowerOff)
	power := widget.NewCard("Power", "", container.NewVBox(
		wrapped("The commands activities use to turn the device on and off; the same one twice for a single power key."),
		widget.NewForm(widget.NewFormItem("On", ed.powerOn), widget.NewFormItem("Off", ed.powerOff))))

	ed.inputList = container.NewVBox()
	for _, in := range e.Inputs {
		ed.addInput(in)
	}
	addInput := widget.NewButtonWithIcon("Add an input", theme.ContentAddIcon(), func() {
		ed.addInput("")
	})
	inputs := widget.NewCard("Inputs", "", container.NewVBox(
		wrapped("The commands that switch the device to each of its inputs, in order. Activities pick one."),
		ed.inputList, container.NewHBox(addInput)))

	ed.macroList = container.NewVBox()
	for _, m := range e.Macros {
		ed.addMacro(m)
	}
	addMacro := widget.NewButtonWithIcon("Add a macro", theme.ContentAddIcon(), func() {
		ed.addMacro(workspace.MacroEdit{})
	})
	macros := widget.NewCard("Macros", "", container.NewVBox(
		wrapped("Sequences of this device's commands. Activities with the device can run them from their keys."),
		ed.macroList, container.NewHBox(addMacro)))

	save := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), ed.save)
	save.Importance = widget.HighImportance
	cancel := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), ed.app.backToBrowse)
	remove := widget.NewButtonWithIcon("Delete this device", theme.DeleteIcon(), ed.remove)
	remove.Importance = widget.DangerImportance
	note := wrapped("Saving keeps your changes in the app; review and apply them to write them to the hub.")
	note.TextStyle.Italic = true

	ed.refreshPickers()
	return container.NewVBox(device, commands, keys, power, inputs, macros, ed.problem,
		container.NewGridWithColumns(2, save, cancel), note, widget.NewSeparator(), remove)
}

// iconChoices are the icons' names, "Unchanged" first for no icon set.
func iconChoices() []string {
	return lookChoices(func(i uint8) string { return profile.Icon(i).String() })
}

func colorChoices() []string {
	return lookChoices(func(i uint8) string { return profile.Color(i).String() })
}

func lookChoices(name func(uint8) string) []string {
	out := []string{"Unchanged"}
	for i := uint8(1); i != 0; i++ {
		n := name(i)
		if _, err := strconv.Atoi(n); err == nil {
			break // past the named ones
		}
		out = append(out, n)
	}
	return out
}

func lookChoice(name string, zero bool) string {
	if zero {
		return "Unchanged"
	}
	return name
}

func parseIcon(s string) profile.Icon {
	for i := uint8(1); i != 0; i++ {
		if profile.Icon(i).String() == s {
			return profile.Icon(i)
		}
	}
	return 0
}

func parseColor(s string) profile.Color {
	for i := uint8(1); i != 0; i++ {
		if profile.Color(i).String() == s {
			return profile.Color(i)
		}
	}
	return 0
}

// addCmd adds a command row.
func (ed *deviceEditor) addCmd(c workspace.CommandEdit) {
	r := &cmdRow{edit: c, name: widget.NewEntry(), code: widget.NewEntry(), last: c.Name}
	r.name.SetText(c.Name)
	r.name.SetPlaceHolder("Name")
	r.name.OnChanged = func(s string) {
		ed.renamed(r.last, s)
		r.last = s
	}
	r.code.SetText(c.Code)
	r.code.SetPlaceHolder("Code")
	if c.Fixed {
		r.code.SetText("learned")
		r.code.Disable()
	}
	more := widget.NewButtonWithIcon("", theme.MoreVerticalIcon(), nil)
	more.OnTapped = func() {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem("Move up", func() { ed.moveCmd(r, -1) }),
			fyne.NewMenuItem("Move down", func() { ed.moveCmd(r, 1) }),
			fyne.NewMenuItem("Delete", func() { ed.removeCmd(r) }))
		widget.ShowPopUpMenuAtRelativePosition(menu, ed.app.win.Canvas(), fyne.NewPos(0, more.Size().Height), more)
	}
	r.box = container.NewBorder(nil, nil, nil,
		container.NewHBox(container.New(layout.NewGridWrapLayout(fyne.NewSize(CodeWidth*Scale(), r.code.MinSize().Height)), r.code), more),
		r.name)
	ed.cmds = append(ed.cmds, r)
	ed.cmdList.Add(r.box)
}

func (ed *deviceEditor) moveCmd(r *cmdRow, by int) {
	for i, o := range ed.cmds {
		if o != r {
			continue
		}
		j := i + by
		if j < 0 || j >= len(ed.cmds) {
			return
		}
		ed.cmds[i], ed.cmds[j] = ed.cmds[j], ed.cmds[i]
		break
	}
	ed.cmdList.Objects = nil
	for _, o := range ed.cmds {
		ed.cmdList.Objects = append(ed.cmdList.Objects, o.box)
	}
	ed.cmdList.Refresh()
}

func (ed *deviceEditor) removeCmd(r *cmdRow) {
	for i, o := range ed.cmds {
		if o == r {
			ed.cmds = append(ed.cmds[:i], ed.cmds[i+1:]...)
			break
		}
	}
	ed.cmdList.Remove(r.box)
	ed.renamed(r.last, "")
}

// names are the commands' names, as typed.
func (ed *deviceEditor) names() []string {
	var out []string
	for _, r := range ed.cmds {
		if n := strings.TrimSpace(r.name.Text); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// picker is a command picker, first showing selected.
func (ed *deviceEditor) picker(selected string) *widget.Select {
	s := widget.NewSelect(nil, nil)
	s.PlaceHolder = none
	if selected != "" {
		s.Selected = selected
	}
	ed.pickers = append(ed.pickers, s)
	return s
}

// keyPicker is a picker of a command or a macro, for a key.
func (ed *deviceEditor) keyPicker(selected string) *widget.Select {
	s := widget.NewSelect(nil, nil)
	s.PlaceHolder = none
	s.Selected = selected
	ed.keyPickers = append(ed.keyPickers, s)
	return s
}

// renamed follows a command's or macro's rename (or removal, to "") in
// the pickers.
func (ed *deviceEditor) renamed(from, to string) {
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)
	for _, p := range append(append([]*widget.Select(nil), ed.pickers...), ed.keyPickers...) {
		if from != "" && p.Selected == from {
			p.Selected = to
		}
	}
	ed.refreshPickers()
}

func (ed *deviceEditor) refreshPickers() {
	options := append([]string{none}, ed.names()...)
	var macros []string
	for _, m := range ed.macros {
		if n := strings.TrimSpace(m.name.Text); n != "" {
			macros = append(macros, n)
		}
	}
	refresh := func(ps []*widget.Select, options []string) {
		for _, p := range ps {
			p.Options = options
			if p.Selected == none {
				p.Selected = ""
			}
			p.Refresh()
		}
	}
	refresh(ed.pickers, options)
	refresh(ed.keyPickers, append(options, macros...))
	if ed.pad != nil {
		ed.pad.refresh()
	}
}

// addMacro adds a macro's box.
func (ed *deviceEditor) addMacro(m workspace.MacroEdit) {
	b := &macroBox{original: m.Original, name: widget.NewEntry(), list: container.NewVBox(), last: m.Name}
	b.name.SetText(m.Name)
	b.name.SetPlaceHolder("Macro name")
	b.name.OnChanged = func(s string) {
		ed.renamed(b.last, s)
		b.last = s
	}
	for _, st := range m.Steps {
		ed.addStep(b, st)
	}
	addCmd := widget.NewButtonWithIcon("Command", theme.ContentAddIcon(), func() {
		ed.addStep(b, profile.DeviceStep{})
		ed.refreshPickers()
	})
	addWait := widget.NewButtonWithIcon("Wait", theme.ContentAddIcon(), func() {
		ed.addStep(b, profile.DeviceStep{Seconds: 1})
	})
	remove := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		for i, o := range ed.macros {
			if o == b {
				ed.macros = append(ed.macros[:i], ed.macros[i+1:]...)
				break
			}
		}
		ed.macroList.Remove(b.box)
		ed.renamed(b.last, "")
	})
	b.box = container.NewVBox(widget.NewSeparator(),
		container.NewBorder(nil, nil, nil, remove, b.name),
		b.list,
		container.NewHBox(widget.NewLabel("Add a step:"), addCmd, addWait))
	ed.macros = append(ed.macros, b)
	ed.macroList.Add(b.box)
}

// addStep adds a step to a macro.
func (ed *deviceEditor) addStep(b *macroBox, st profile.DeviceStep) {
	r := &stepRow{hold: st.Hold}
	var main fyne.CanvasObject
	if st.Command != "" || st.Seconds == 0 {
		r.cmd = ed.picker(st.Command)
		main = r.cmd
	} else {
		r.wait = widget.NewEntry()
		r.wait.SetText(strconv.Itoa(int(st.Seconds)))
		main = container.NewBorder(nil, nil, widget.NewLabel("Wait"), widget.NewLabel("seconds"), r.wait)
	}
	more := widget.NewButtonWithIcon("", theme.MoreVerticalIcon(), nil)
	more.OnTapped = func() {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem("Move up", func() { ed.moveStep(b, r, -1) }),
			fyne.NewMenuItem("Move down", func() { ed.moveStep(b, r, 1) }),
			fyne.NewMenuItem("Delete", func() { ed.moveStep(b, r, 0) }))
		widget.ShowPopUpMenuAtRelativePosition(menu, ed.app.win.Canvas(), fyne.NewPos(0, more.Size().Height), more)
	}
	r.box = container.NewBorder(nil, nil, nil, more, main)
	b.steps = append(b.steps, r)
	b.list.Add(r.box)
}

// moveStep moves a step by one, or deletes it (by 0).
func (ed *deviceEditor) moveStep(b *macroBox, r *stepRow, by int) {
	for i, o := range b.steps {
		if o != r {
			continue
		}
		if by == 0 {
			b.steps = append(b.steps[:i], b.steps[i+1:]...)
			break
		}
		j := i + by
		if j >= 0 && j < len(b.steps) {
			b.steps[i], b.steps[j] = b.steps[j], b.steps[i]
		}
		break
	}
	b.list.Objects = nil
	for _, o := range b.steps {
		b.list.Objects = append(b.list.Objects, o.box)
	}
	b.list.Refresh()
}

func pick(s *widget.Select) string {
	if s.Selected == none {
		return ""
	}
	return s.Selected
}

// isRemoteKey reports whether a key name is a key of the remote. A
// device's key map also has Power On and Power Off rows, which the app
// binds to its power commands; the editor keeps them as they are.
func isRemoteKey(name string) bool {
	b, err := record.ParseButton(name)
	return err == nil && b.IsHardKey() && b != record.ButtonPowerOn && b != record.ButtonPowerOff
}

func (ed *deviceEditor) addInput(selected string) {
	p := ed.picker(selected)
	var box fyne.CanvasObject
	remove := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		for i, o := range ed.inputs {
			if o == p {
				ed.inputs = append(ed.inputs[:i], ed.inputs[i+1:]...)
				break
			}
		}
		ed.inputList.Remove(box)
	})
	box = container.NewBorder(nil, nil, nil, remove, p)
	ed.inputs = append(ed.inputs, p)
	ed.inputList.Add(box)
	ed.refreshPickers()
}

// edit is the editor's state as a workspace edit.
func (ed *deviceEditor) edit() workspace.DeviceEdit {
	e := workspace.DeviceEdit{
		Original: ed.e.Original, Name: ed.name.Text, Brand: ed.brand.Text,
		Icon: parseIcon(ed.icon.Selected), Color: parseColor(ed.color.Selected),
		Keys:    profile.Buttons{},
		PowerOn: pick(ed.powerOn), PowerOff: pick(ed.powerOff),
		NoInputs: ed.e.NoInputs && len(ed.inputs) == 0,
	}
	for _, r := range ed.cmds {
		c := r.edit
		c.Name = r.name.Text
		if !c.Fixed {
			c.Code = r.code.Text
		}
		e.Commands = append(e.Commands, c)
	}
	for k, v := range ed.e.Keys {
		if !isRemoteKey(k) {
			e.Keys[k] = v
		}
	}
	ed.pad.set(e.Keys)
	for _, p := range ed.inputs {
		if s := pick(p); s != "" {
			e.Inputs = append(e.Inputs, s)
		}
	}
	for _, b := range ed.macros {
		m := workspace.MacroEdit{Original: b.original, Name: b.name.Text}
		for _, r := range b.steps {
			if r.cmd != nil {
				m.Steps = append(m.Steps, profile.DeviceStep{Command: pick(r.cmd), Hold: r.hold})
				continue
			}
			v, err := strconv.ParseUint(strings.TrimSpace(r.wait.Text), 10, 8)
			if err != nil {
				v = 0 // reported as a step with nothing to do
			}
			m.Steps = append(m.Steps, profile.DeviceStep{Seconds: uint8(v)})
		}
		e.Macros = append(e.Macros, m)
	}
	return e
}

func (ed *deviceEditor) save() {
	e := ed.edit()
	if err := ed.app.work.SaveDevice(e); err != nil {
		var fe *workspace.FieldError
		if errors.As(err, &fe) && fe.Field == "command" && fe.Index < len(ed.cmds) {
			ed.app.win.Canvas().Focus(ed.cmds[fe.Index].name)
		}
		if errors.As(err, &fe) && fe.Field == "macro" && fe.Index < len(ed.macros) {
			ed.app.win.Canvas().Focus(ed.macros[fe.Index].name)
		}
		ed.problem.SetText(err.Error())
		ed.problem.Show()
		return
	}
	b := ed.app.showBrowse(ed.app.hub)
	b.selectDevice(strings.TrimSpace(e.Name))
	b.setStatus(false, "Saved. Review your changes to write them to the hub.")
}

func (ed *deviceEditor) remove() {
	name := ed.e.Original
	msg := fmt.Sprintf("%s will be deleted from the hub when you apply your changes.", name)
	if users := ed.app.work.ActivitiesUsing(name); len(users) > 0 {
		msg += fmt.Sprintf(" It is used by %s, which will no longer turn it on or use its commands.", strings.Join(users, ", "))
	}
	dialog.ShowConfirm("Delete "+name+"?", msg, func(ok bool) {
		if !ok {
			return
		}
		if err := ed.app.work.RemoveDevice(name); err != nil {
			dialog.ShowError(err, ed.app.win)
			return
		}
		b := ed.app.showBrowse(ed.app.hub)
		b.setStatus(false, fmt.Sprintf("Deleted %s. Review your changes to write them to the hub.", name))
	}, ed.app.win)
}
