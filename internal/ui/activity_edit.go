package ui

import (
	"errors"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/armrest/internal/workspace"
	"github.com/brunoga/sofabaton/profile"
)

// Step kinds, as the editor names them.
const (
	stepSend   = "Send a command"
	stepWait   = "Wait"
	stepOn     = "Turn on"
	stepOff    = "Turn off"
	stepInput  = "Switch to its input"
	usual      = "The usual steps"
	usualOn    = "Turn each device on, a second apart, then switch each to its input."
	usualOff   = "Turn each device off, a second apart."
	waitSuffix = "seconds"
)

var stepKinds = []string{i18n.Mark(stepSend), i18n.Mark(stepWait), i18n.Mark(stepOn), i18n.Mark(stepOff), i18n.Mark(stepInput)}

// stepKindChoices are the step kinds in the user's language.
func stepKindChoices() []string {
	out := make([]string, len(stepKinds))
	for i, k := range stepKinds {
		out[i] = i18n.T(k)
	}
	return out
}

// stepKind is the step kind shown as shown, in the user's language.
func stepKind(shown string) string {
	for _, k := range stepKinds {
		if i18n.T(k) == shown {
			return k
		}
	}
	return shown
}

// activityEditor edits an activity of the workspace, or a new one.
type activityEditor struct {
	app *App
	e   workspace.ActivityEdit

	name         *widget.Entry
	icon, color  *widget.Select
	members      []*memberRow
	memberList   *fyne.Container
	pad          *keyPad
	powerOn      *stepsEditor
	powerOff     *stepsEditor
	macros       []*actMacroBox
	macroList    *fyne.Container
	favorites    []*favRow
	favoriteList *fyne.Container
	problem      *widget.Label

	// Pickers by what they offer: the devices' commands, the activity's
	// devices, and what a key can run (commands and macros).
	cmdPickers []*widget.Select
	devPickers []*widget.Select
	keyPickers []*widget.Select
}

type memberRow struct {
	device, input *widget.Select
	box           fyne.CanvasObject
}

type favRow struct {
	cmd *widget.Select
	box fyne.CanvasObject
}

type actMacroBox struct {
	original string
	name     *widget.Entry
	steps    *stepsEditor
	box      fyne.CanvasObject
	last     string
}

// stepsEditor edits a list of steps; for starting and ending, the usual
// steps can stand in for them.
type stepsEditor struct {
	ed    *activityEditor
	rows  []*actStepRow
	list  *fyne.Container
	usual *widget.Check
	on    bool
	box   fyne.CanvasObject
	// original are the steps the editor started with, kept when the usual
	// steps stand for them.
	original []profile.Step
}

type actStepRow struct {
	kind        *widget.Select
	cmd, device *widget.Select
	secs        *widget.Entry
	wait        fyne.CanvasObject
	hold        uint8
	box         fyne.CanvasObject
}

func (a *App) showEditActivity(name string) {
	e := a.work.NewActivity()
	title := i18n.T("New activity")
	if name != "" {
		var err error
		if e, err = a.work.EditActivity(name); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		title = i18n.T("Edit %s", e.Name)
	}
	ed := &activityEditor{app: a, e: e}
	a.screen(title, a.backToBrowse, scrolled(ed.content()))
}

func (ed *activityEditor) content() fyne.CanvasObject {
	e := ed.e
	ed.name = widget.NewEntry()
	ed.name.SetText(e.Name)
	ed.name.SetPlaceHolder(i18n.T("Watch TV"))
	ed.icon = widget.NewSelect(iconChoices(), nil)
	ed.icon.SetSelected(lookChoice(e.Icon.String(), e.Icon.IsZero()))
	ed.color = widget.NewSelect(colorChoices(), nil)
	ed.color.SetSelected(lookChoice(e.Color.String(), e.Color.IsZero()))
	activity := widget.NewCard(i18n.T("Activity"), "", widget.NewForm(
		widget.NewFormItem(i18n.T("Name"), ed.name),
		widget.NewFormItem(i18n.T("Icon"), ed.icon),
		widget.NewFormItem(i18n.T("Color"), ed.color)))

	ed.memberList = container.NewVBox()
	for _, m := range e.Devices {
		ed.addMember(m)
	}
	addDevice := widget.NewButtonWithIcon(i18n.T("Add a device"), theme.ContentAddIcon(), func() {
		ed.addMember(profile.Member{})
		ed.refresh()
	})
	devices := widget.NewCard(i18n.T("Devices"), "", container.NewVBox(
		wrapped(i18n.T("The devices the activity uses, and the input each is switched to.")),
		ed.memberList, container.NewHBox(addDevice)))

	ed.pad = newKeyPad(ed.app.win, ed.app.work.Dump.Info.Model, e.Keys, func(selected string) *widget.Select {
		return ed.picker(&ed.keyPickers, selected)
	}, ed.refresh)
	keys := widget.NewCard(i18n.T("Keys"), "", container.NewVBox(
		wrapped(i18n.T("What the remote's keys do during the activity.")),
		ed.pad.box))

	ed.powerOn = ed.newSteps(e.PowerOn, true, true)
	ed.powerOff = ed.newSteps(e.PowerOff, true, false)
	starts := widget.NewCard(i18n.T("When it starts"), "", ed.powerOn.box)
	ends := widget.NewCard(i18n.T("When it ends"), "", ed.powerOff.box)

	ed.macroList = container.NewVBox()
	for _, m := range e.Macros {
		ed.addMacro(m)
	}
	addMacro := widget.NewButtonWithIcon(i18n.T("Add a macro"), theme.ContentAddIcon(), func() {
		ed.addMacro(workspace.ActivityMacroEdit{})
		ed.refresh()
	})
	macros := widget.NewCard(i18n.T("Macros"), "", container.NewVBox(
		wrapped(i18n.T("Sequences of the devices' commands, run from a key or the remote's list.")+" "+i18n.T(dragHint)),
		ed.macroList, container.NewHBox(addMacro)))

	ed.favoriteList = container.NewVBox()
	for _, f := range e.Favorites {
		ed.addFavorite(f)
	}
	addFav := widget.NewButtonWithIcon(i18n.T("Add a favorite"), theme.ContentAddIcon(), func() {
		ed.addFavorite("")
		ed.refresh()
	})
	favorites := widget.NewCard(i18n.T("Favorites"), "", container.NewVBox(
		wrapped(i18n.T("Commands on the remote's list for the activity, after its macros.")+" "+i18n.T(dragHint)),
		ed.favoriteList, container.NewHBox(addFav)))

	ed.problem = wrapped("")
	ed.problem.Importance = widget.DangerImportance
	ed.problem.Hide()
	save := widget.NewButtonWithIcon(i18n.T("Save"), theme.DocumentSaveIcon(), ed.save)
	save.Importance = widget.HighImportance
	cancel := widget.NewButtonWithIcon(i18n.T("Cancel"), theme.CancelIcon(), ed.app.backToBrowse)
	note := wrapped(i18n.T("Saving keeps your changes in the app; review and apply them to write them to the hub."))
	note.TextStyle.Italic = true
	parts := []fyne.CanvasObject{activity, devices, keys, starts, ends, macros, favorites, ed.problem,
		container.NewGridWithColumns(2, save, cancel), note}
	if e.Original != "" {
		remove := widget.NewButtonWithIcon(i18n.T("Delete this activity"), theme.DeleteIcon(), ed.remove)
		remove.Importance = widget.DangerImportance
		parts = append(parts, widget.NewSeparator(), remove)
	}
	ed.refresh()
	return container.NewVBox(parts...)
}

// memberNames are the devices picked, in order.
func (ed *activityEditor) memberNames() []string {
	var out []string
	for _, m := range ed.members {
		if m.device.Selected != "" {
			out = append(out, m.device.Selected)
		}
	}
	return out
}

func (ed *activityEditor) macroNames() []string {
	var out []string
	for _, m := range ed.macros {
		if n := strings.TrimSpace(m.name.Text); n != "" {
			out = append(out, n)
		}
	}
	return out
}

// refresh offers each picker what the activity has now.
func (ed *activityEditor) refresh() {
	members := ed.memberNames()
	cmds := ed.app.work.Commands(members)
	set := func(ps []*widget.Select, options []string) {
		for _, p := range ps {
			p.Options = append([]string{noneChoice()}, options...)
			if p.Selected == noneChoice() {
				p.Selected = ""
			}
			p.Refresh()
		}
	}
	set(ed.cmdPickers, cmds)
	set(ed.devPickers, members)
	set(ed.keyPickers, append(append(cmds, ed.app.work.DeviceMacros(members)...), ed.macroNames()...))
	for _, m := range ed.members {
		inputs := ed.app.work.Inputs(m.device.Selected)
		m.input.Options = append([]string{noneChoice()}, inputs...)
		if len(inputs) == 0 {
			m.input.Hide()
		} else {
			m.input.Show()
		}
		m.input.Refresh()
	}
	if ed.pad != nil {
		ed.pad.refresh()
	}
}

func (ed *activityEditor) picker(list *[]*widget.Select, selected string) *widget.Select {
	s := widget.NewSelect(nil, nil)
	s.PlaceHolder = noneChoice()
	s.Selected = selected
	*list = append(*list, s)
	return s
}

func (ed *activityEditor) addMember(m profile.Member) {
	r := &memberRow{}
	r.device = widget.NewSelect(ed.app.work.DeviceNames(), func(string) { ed.refresh() })
	r.device.PlaceHolder = i18n.T("Pick a device")
	r.device.Selected = m.Device
	r.input = widget.NewSelect(nil, nil)
	r.input.PlaceHolder = i18n.T("No input")
	r.input.Selected = m.Input
	remove := newIconButton(i18n.T("Delete"), theme.DeleteIcon(), nil)
	r.box = container.NewVBox(container.NewBorder(nil, nil, nil, remove, r.device),
		widget.NewForm(widget.NewFormItem(i18n.T("Input"), r.input)))
	remove.OnTapped = func() {
		for i, o := range ed.members {
			if o == r {
				ed.members = append(ed.members[:i], ed.members[i+1:]...)
				break
			}
		}
		ed.memberList.Remove(r.box)
		ed.refresh()
	}
	ed.members = append(ed.members, r)
	ed.memberList.Add(r.box)
}

// newSteps makes a steps editor; for starting and ending (power), nil
// steps are the usual ones.
func (ed *activityEditor) newSteps(steps []profile.Step, power, on bool) *stepsEditor {
	s := &stepsEditor{ed: ed, list: container.NewVBox(), on: on, original: steps}
	isUsual := power && (steps == nil || workspace.IsUsual(steps, ed.memberNames(), on))
	if !isUsual {
		for _, st := range steps {
			s.add(st)
		}
	}
	addBtn := widget.NewButtonWithIcon(i18n.T("Add a step"), theme.ContentAddIcon(), func() {
		s.add(profile.Step{Kind: profile.StepCommand})
		ed.refresh()
	})
	custom := container.NewVBox(s.list, container.NewHBox(addBtn))
	if !power {
		s.box = custom
		return s
	}
	// The macros' card says how steps move; the power steps have a card
	// each.
	custom.Objects = append([]fyne.CanvasObject{wrapped(i18n.T(dragHint))}, custom.Objects...)
	about := wrapped(i18n.T(usualOff))
	if on {
		about.SetText(i18n.T(usualOn))
	}
	about.TextStyle.Italic = true
	s.usual = widget.NewCheck(i18n.T(usual), func(checked bool) {
		if checked {
			custom.Hide()
			about.Show()
			return
		}
		about.Hide()
		if len(s.rows) == 0 {
			s.fillUsual()
		}
		custom.Show()
	})
	s.usual.SetChecked(isUsual)
	if isUsual {
		custom.Hide()
	} else {
		about.Hide()
	}
	s.box = container.NewVBox(s.usual, about, custom)
	return s
}

// fillUsual starts the custom steps from the usual ones.
func (s *stepsEditor) fillUsual() {
	for _, st := range workspace.UsualSteps(s.ed.memberNames(), s.on) {
		s.add(st)
	}
	s.ed.refresh()
}

func (s *stepsEditor) add(st profile.Step) {
	ed := s.ed
	r := &actStepRow{hold: st.Hold}
	r.cmd = ed.picker(&ed.cmdPickers, "")
	r.device = ed.picker(&ed.devPickers, "")
	r.secs = widget.NewEntry()
	r.wait = container.NewBorder(nil, nil, nil, widget.NewLabel(i18n.T(waitSuffix)), r.secs)
	var kind string
	switch st.Kind {
	case profile.StepWait:
		kind = stepWait
		r.secs.SetText(strconv.Itoa(int(st.Seconds)))
	case profile.StepPowerOn:
		kind = stepOn
		r.device.Selected = st.Device
	case profile.StepPowerOff:
		kind = stepOff
		r.device.Selected = st.Device
	case profile.StepInput:
		kind = stepInput
		r.device.Selected = st.Device
	default:
		kind = stepSend
		if st.Device != "" {
			r.cmd.Selected = st.Device + ": " + st.Command
		}
	}
	r.kind = widget.NewSelect(stepKindChoices(), func(k string) { r.show(stepKind(k)) })
	r.kind.Selected = i18n.T(kind)
	r.show(kind)
	more := newIconButton(i18n.T("More"), theme.MoreVerticalIcon(), nil)
	more.OnTapped = func() {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem(i18n.T("Move up"), func() { s.move(r, -1) }),
			fyne.NewMenuItem(i18n.T("Move down"), func() { s.move(r, 1) }),
			fyne.NewMenuItem(i18n.T("Delete"), func() { s.move(r, 0) }))
		widget.ShowPopUpMenuAtRelativePosition(menu, ed.app.win.Canvas(), fyne.NewPos(0, more.Size().Height), more)
	}
	grip := newDragHandle(s.list, func(i, j int) { s.rows[i], s.rows[j] = s.rows[j], s.rows[i] }, ed.refresh)
	r.box = container.NewBorder(nil, nil, grip, nil, container.NewVBox(container.NewBorder(nil, nil, nil, more, r.kind),
		container.NewStack(r.cmd, r.device, r.wait)))
	grip.row = r.box
	s.rows = append(s.rows, r)
	s.list.Add(r.box)
}

// show shows the step's target for its kind.
func (r *actStepRow) show(kind string) {
	r.cmd.Hide()
	r.device.Hide()
	r.wait.Hide()
	switch kind {
	case stepSend:
		r.cmd.Show()
	case stepWait:
		if r.secs.Text == "" {
			r.secs.SetText("1")
		}
		r.wait.Show()
	default:
		r.device.Show()
	}
}

func (s *stepsEditor) move(r *actStepRow, by int) {
	for i, o := range s.rows {
		if o != r {
			continue
		}
		if by == 0 {
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
			break
		}
		if j := i + by; j >= 0 && j < len(s.rows) {
			s.rows[i], s.rows[j] = s.rows[j], s.rows[i]
		}
		break
	}
	s.list.Objects = nil
	for _, o := range s.rows {
		s.list.Objects = append(s.list.Objects, o.box)
	}
	s.list.Refresh()
}

// steps are the editor's steps, or nil for the usual ones (the steps it
// started with when they are the usual ones for the devices as they are).
func (s *stepsEditor) steps() []profile.Step {
	if s.usual != nil && s.usual.Checked {
		if workspace.IsUsual(s.original, s.ed.memberNames(), s.on) {
			return s.original
		}
		return nil
	}
	out := []profile.Step{}
	for _, r := range s.rows {
		st := profile.Step{Hold: r.hold}
		kind := stepKind(r.kind.Selected)
		switch kind {
		case stepWait:
			st.Kind = profile.StepWait
			if v, err := strconv.ParseUint(strings.TrimSpace(r.secs.Text), 10, 8); err == nil {
				st.Seconds = uint8(v)
			}
		case stepOn, stepOff, stepInput:
			st.Kind = map[string]profile.StepKind{stepOn: profile.StepPowerOn, stepOff: profile.StepPowerOff, stepInput: profile.StepInput}[kind]
			st.Device = pick(r.device)
		default:
			st.Kind = profile.StepCommand
			st.Device, st.Command, _ = strings.Cut(pick(r.cmd), ": ")
		}
		out = append(out, st)
	}
	return out
}

func (ed *activityEditor) addMacro(m workspace.ActivityMacroEdit) {
	b := &actMacroBox{original: m.Original, name: widget.NewEntry(), last: m.Name}
	b.name.SetText(m.Name)
	b.name.SetPlaceHolder(i18n.T("Macro name"))
	b.name.OnChanged = func(s string) {
		for _, p := range ed.keyPickers {
			if b.last != "" && p.Selected == b.last {
				p.Selected = s
			}
		}
		b.last = s
		ed.refresh()
	}
	b.steps = ed.newSteps(m.Steps, false, false)
	remove := newIconButton(i18n.T("Delete"), theme.DeleteIcon(), nil)
	b.box = container.NewVBox(widget.NewSeparator(), container.NewBorder(nil, nil, nil, remove, b.name), b.steps.box)
	remove.OnTapped = func() {
		for i, o := range ed.macros {
			if o == b {
				ed.macros = append(ed.macros[:i], ed.macros[i+1:]...)
				break
			}
		}
		ed.macroList.Remove(b.box)
		ed.refresh()
	}
	ed.macros = append(ed.macros, b)
	ed.macroList.Add(b.box)
}

func (ed *activityEditor) addFavorite(cmd string) {
	r := &favRow{cmd: ed.picker(&ed.cmdPickers, cmd)}
	more := newIconButton(i18n.T("More"), theme.MoreVerticalIcon(), nil)
	grip := newDragHandle(ed.favoriteList, func(i, j int) { ed.favorites[i], ed.favorites[j] = ed.favorites[j], ed.favorites[i] }, ed.refresh)
	r.box = container.NewBorder(nil, nil, grip, more, r.cmd)
	grip.row = r.box
	move := func(by int) {
		for i, o := range ed.favorites {
			if o != r {
				continue
			}
			if by == 0 {
				ed.favorites = append(ed.favorites[:i], ed.favorites[i+1:]...)
				break
			}
			if j := i + by; j >= 0 && j < len(ed.favorites) {
				ed.favorites[i], ed.favorites[j] = ed.favorites[j], ed.favorites[i]
			}
			break
		}
		ed.favoriteList.Objects = nil
		for _, o := range ed.favorites {
			ed.favoriteList.Objects = append(ed.favoriteList.Objects, o.box)
		}
		ed.favoriteList.Refresh()
	}
	more.OnTapped = func() {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem(i18n.T("Move up"), func() { move(-1) }),
			fyne.NewMenuItem(i18n.T("Move down"), func() { move(1) }),
			fyne.NewMenuItem(i18n.T("Delete"), func() { move(0) }))
		widget.ShowPopUpMenuAtRelativePosition(menu, ed.app.win.Canvas(), fyne.NewPos(0, more.Size().Height), more)
	}
	ed.favorites = append(ed.favorites, r)
	ed.favoriteList.Add(r.box)
}

// edit is the editor's state as a workspace edit.
func (ed *activityEditor) edit() workspace.ActivityEdit {
	e := workspace.ActivityEdit{
		Original: ed.e.Original, Name: ed.name.Text,
		Icon: parseIcon(ed.icon.Selected), Color: parseColor(ed.color.Selected),
		Keys:    profile.Buttons{},
		PowerOn: ed.powerOn.steps(), PowerOff: ed.powerOff.steps(),
	}
	for _, m := range ed.members {
		e.Devices = append(e.Devices, profile.Member{Device: m.device.Selected, Input: pick(m.input)})
	}
	// The key map's rows that are not keys of the remote stay as they are.
	for k, v := range ed.e.Keys {
		if !isRemoteKey(k) {
			e.Keys[k] = v
		}
	}
	ed.pad.set(e.Keys)
	for _, b := range ed.macros {
		e.Macros = append(e.Macros, workspace.ActivityMacroEdit{Original: b.original, Name: b.name.Text, Steps: b.steps.steps()})
	}
	if ed.e.Favorites != nil || len(ed.favorites) > 0 {
		e.Favorites = []string{}
		for _, f := range ed.favorites {
			if s := pick(f.cmd); s != "" {
				e.Favorites = append(e.Favorites, s)
			}
		}
	}
	return e
}

func (ed *activityEditor) save() {
	e := ed.edit()
	if err := ed.app.work.SaveActivity(e); err != nil {
		var fe *workspace.FieldError
		if errors.As(err, &fe) && fe.Field == "macro" && fe.Index < len(ed.macros) {
			ed.app.win.Canvas().Focus(ed.macros[fe.Index].name)
		}
		if errors.As(err, &fe) && fe.Field == "name" {
			ed.app.win.Canvas().Focus(ed.name)
		}
		ed.problem.SetText(err.Error())
		ed.problem.Show()
		return
	}
	b := ed.app.showBrowse(ed.app.hub)
	b.selectActivity(strings.TrimSpace(e.Name))
	b.setStatus(false, i18n.T("Saved. Review your changes to write them to the hub."))
}

func (ed *activityEditor) remove() {
	name := ed.e.Original
	dialog.ShowConfirm(i18n.T("Delete %s?", name),
		i18n.T("%s will be deleted from the hub when you apply your changes. Its devices stay.", name),
		func(ok bool) {
			if !ok {
				return
			}
			if err := ed.app.work.RemoveActivity(name); err != nil {
				dialog.ShowError(err, ed.app.win)
				return
			}
			ed.app.showBrowse(ed.app.hub).setStatus(false, i18n.T("Deleted %s. Review your changes to write them to the hub.", name))
		}, ed.app.win)
}
