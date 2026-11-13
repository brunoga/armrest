package ui

import (
	"errors"
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

// urlHint is the address entries' placeholder.
const urlHint = "http://192.168.1.50:8080/lights/on"

// newURLEntry is an entry for a request's address.
func newURLEntry(h *profile.HTTP) *widget.Entry {
	u := widget.NewEntry()
	u.SetPlaceHolder(urlHint)
	u.SetText(h.URL)
	u.OnChanged = func(s string) { h.URL = s }
	return u
}

// editRequest edits the rest of a request: its method, headers, content
// type and body.
func (a *App) editRequest(name string, h *profile.HTTP) {
	method := widget.NewSelect(workspace.Methods, nil)
	method.SetSelected("GET")
	if h.Method != "" {
		method.SetSelected(strings.ToUpper(h.Method))
	}
	headers := widget.NewMultiLineEntry()
	headers.SetPlaceHolder("Authorization: Bearer abc")
	headers.SetText(strings.Join(h.Headers, "\n"))
	headers.SetMinRowsVisible(2)
	kind := widget.NewEntry()
	kind.SetPlaceHolder("application/json")
	kind.SetText(h.ContentType)
	body := widget.NewMultiLineEntry()
	body.SetPlaceHolder(`{"on": true}`)
	body.SetText(h.Body)
	body.SetMinRowsVisible(3)
	form := widget.NewForm(
		widget.NewFormItem(i18n.T("Method"), method),
		widget.NewFormItem(i18n.T("Headers"), headers),
		widget.NewFormItem(i18n.T("Content type"), kind),
		widget.NewFormItem(i18n.T("Body"), body))
	note := wrapped(i18n.T("One header a line. Most devices need only the address; a webhook may want POST, a content type and a body."))
	note.TextStyle.Italic = true
	title := i18n.T("The request")
	if strings.TrimSpace(name) != "" {
		title = i18n.T("%s's request", strings.TrimSpace(name))
	}
	d := dialog.NewCustomConfirm(title, i18n.T("OK"), i18n.T("Cancel"), container.NewVBox(note, form), func(ok bool) {
		if !ok {
			return
		}
		h.Method = method.Selected
		h.Headers = strings.Split(headers.Text, "\n")
		h.ContentType, h.Body = kind.Text, body.Text
	}, a.win)
	d.Resize(fyne.NewSize(min(ReadableWidth*Scale()*0.7, a.win.Canvas().Size().Width), d.MinSize().Height))
	d.Show()
}

// networkScreen makes a network device: one whose commands are HTTP
// requests the hub sends.
type networkScreen struct {
	app *App

	name, brand *widget.Entry
	rows        []*requestRow
	list        *fyne.Container
	powerOn     *widget.Select
	powerOf     *widget.Select
	problem     *widget.Label
}

type requestRow struct {
	name *widget.SelectEntry
	url  *widget.Entry
	http *profile.HTTP
	box  fyne.CanvasObject
}

func (a *App) showNetwork() {
	s := &networkScreen{app: a}
	a.screen(i18n.T("New network device"), a.showAddDevice, scrolled(s.content()))
}

func (s *networkScreen) content() fyne.CanvasObject {
	s.name = widget.NewEntry()
	s.name.SetPlaceHolder(i18n.T("Living room lights"))
	s.brand = widget.NewEntry()
	s.brand.SetPlaceHolder(i18n.T("Optional"))
	device := titled(i18n.T("Device"), container.NewVBox(
		wrapped(i18n.T("A device the hub controls over your network: for each command, the hub sends an HTTP request, as to a smart plug, a home automation webhook or a media server.")),
		widget.NewForm(
			widget.NewFormItem(i18n.T("Name"), s.name),
			widget.NewFormItem(i18n.T("Brand"), s.brand))))

	s.list = container.NewVBox()
	add := widget.NewButtonWithIcon(i18n.T("Add a command"), theme.ContentAddIcon(), func() { s.addRow() })
	s.addRow()
	commands := titled(i18n.T("Commands"), container.NewVBox(
		wrapped(i18n.T("A name and an address for each; More sets the rest of the request. The hub sends plain http:// requests to IPv4 addresses only. Commands named like a key of the remote (OK, Up, Volume Up…) are put on that key.")+" "+i18n.T(dragHint)),
		s.list, container.NewHBox(add)))

	s.powerOn = widget.NewSelect(nil, nil)
	s.powerOf = widget.NewSelect(nil, nil)
	s.powerOn.PlaceHolder = i18n.T("None")
	s.powerOf.PlaceHolder = i18n.T("None")
	s.refreshPower()
	power := titled(i18n.T("Power"), container.NewVBox(
		wrapped(i18n.T("The commands activities use to turn the device on and off. Pick the same one twice if it has a single power key.")),
		widget.NewForm(widget.NewFormItem(i18n.T("On"), s.powerOn), widget.NewFormItem(i18n.T("Off"), s.powerOf))))

	s.problem = wrapped("")
	s.problem.Importance = widget.DangerImportance
	s.problem.Hide()
	create := widget.NewButtonWithIcon(i18n.T("Add the device"), theme.ConfirmIcon(), s.create)
	create.Importance = widget.HighImportance
	note := wrapped(i18n.T("Nothing is written to the hub until you review and apply your changes. Once it is on the hub, you can test each command."))
	note.TextStyle.Italic = true
	return container.NewVBox(device, commands, power, s.problem, create, note)
}

// addRow adds a command row.
func (s *networkScreen) addRow() {
	r := &requestRow{name: widget.NewSelectEntry(workspace.CommonCommands), http: &profile.HTTP{}}
	r.name.SetPlaceHolder(i18n.T("Name"))
	r.name.OnChanged = func(string) { s.refreshPower() }
	r.url = newURLEntry(r.http)
	more := newIconButton(i18n.T("More"), theme.MoreVerticalIcon(), nil)
	more.OnTapped = func() {
		menu := fyne.NewMenu("",
			fyne.NewMenuItem(i18n.T("Request…"), func() { s.app.editRequest(r.name.Text, r.http) }),
			fyne.NewMenuItem(i18n.T("Delete"), func() { s.removeRow(r) }))
		widget.ShowPopUpMenuAtRelativePosition(menu, s.app.win.Canvas(), fyne.NewPos(0, more.Size().Height), more)
	}
	grip := newDragHandle(s.list, func(i, j int) { s.rows[i], s.rows[j] = s.rows[j], s.rows[i] }, s.refreshPower)
	r.box = container.NewBorder(nil, nil, grip, more, container.NewVBox(r.name, r.url))
	grip.row = r.box
	s.rows = append(s.rows, r)
	s.list.Add(r.box)
}

func (s *networkScreen) removeRow(r *requestRow) {
	for i, o := range s.rows {
		if o == r {
			s.rows = append(s.rows[:i], s.rows[i+1:]...)
		}
	}
	s.list.Remove(r.box)
	s.refreshPower()
}

// refreshPower offers the commands' names for power on and off.
func (s *networkScreen) refreshPower() {
	if s.powerOn == nil {
		return
	}
	var names []string
	for _, r := range s.rows {
		if n := strings.TrimSpace(r.name.Text); n != "" {
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

func (s *networkScreen) spec() workspace.NetworkSpec {
	sp := workspace.NetworkSpec{Name: s.name.Text, Brand: s.brand.Text}
	for _, r := range s.rows {
		sp.Commands = append(sp.Commands, workspace.NetworkCommand{Name: r.name.Text, HTTP: *r.http})
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
func (s *networkScreen) create() {
	d, err := s.spec().Build()
	if err == nil {
		err = s.app.work.AddDevice(d)
	}
	if err != nil {
		var fe *workspace.FieldError
		if errors.As(err, &fe) && fe.Field == "command" && fe.Index < len(s.rows) {
			s.app.win.Canvas().Focus(s.rows[fe.Index].url)
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
