package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/armrest/internal/workspace"
	"github.com/brunoga/sofabaton/irsource"
	"github.com/brunoga/sofabaton/profile"
)

// screen is a full-window page with a title, Back, and content.
func (a *App) screen(title string, back func(), content fyne.CanvasObject) {
	a.page(title, back, nil, content, backTo(back))
	// Wrapped text learns its height from its width: lay out again now
	// that the widths are known.
	content.Refresh()
}

// showAddDevice lets the user pick a template for a new device, or a
// device of the online sources of IR codes.
func (a *App) showAddDevice() {
	all := workspace.Templates()
	shown := all
	var found []irsource.Entry // online, after the templates
	var sofabatonFetched func(irsource.SofabatonBrand, error)
	search := widget.NewEntry()
	search.SetPlaceHolder(i18n.T("Search by brand or model"))
	list := widget.NewList(
		func() int { return len(shown) + len(found) },
		func() fyne.CanvasObject {
			name := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			name.Truncation = fyne.TextTruncateEllipsis
			sub := widget.NewLabel("")
			sub.Truncation = fyne.TextTruncateEllipsis
			from := newChip("", chipInfo)
			text := container.New(layout.NewCustomPaddedVBoxLayout(-2.5*theme.Padding()), name, quiet(sub))
			right := container.NewHBox(container.NewCenter(from), container.NewCenter(quiet(widget.NewIcon(chevronIcon))))
			return listRow(container.NewBorder(nil, nil, nil, right, text))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			row := o.(*fyne.Container).Objects[0].(*fyne.Container)
			text, right := row.Objects[0].(*fyne.Container), row.Objects[1].(*fyne.Container)
			name := text.Objects[0].(*widget.Label)
			sub := text.Objects[1].(*container.ThemeOverride).Content.(*widget.Label)
			from := right.Objects[0].(*fyne.Container).Objects[0].(*chip)
			if i < len(shown) {
				t := shown[i]
				name.SetText(t.Model)
				sub.SetText(i18n.N("%s · %d command", "%s · %d commands", t.Commands, t.Brand, t.Commands))
				from.SetText(i18n.T("Built in"), chipNew)
				return
			}
			e := found[i-len(shown)]
			name.SetText(e.Name())
			sub.SetText(onlineAbout(e))
			from.SetText(string(e.Source), chipInfo)
		})
	none := wrapped(i18n.T("No device matches. Try fewer words, or start from scratch."))
	none.TextStyle.Italic = true
	none.Hide()
	status := wrapped("")
	status.TextStyle.Italic = true
	status.Hide()
	alive := newPulse()
	alive.Hide()
	say := func(busy bool, text string) {
		status.SetText(text)
		status.Hidden = text == ""
		status.Refresh()
		if busy {
			alive.Start()
		} else {
			alive.Stop()
		}
	}
	// Narrowing by type and maker: the online sources' types, and their
	// makers of the type picked (all the makers would be too many to
	// pick from).
	anyType, anyMaker := i18n.T(anyType), i18n.T(anyMaker)
	kind := widget.NewSelect(append([]string{anyType}, irsource.Types...), nil)
	kind.SetSelected(anyType)
	maker := widget.NewSelect([]string{anyMaker}, nil)
	maker.SetSelected(anyMaker)
	maker.PlaceHolder = i18n.T("Maker")
	maker.Disable()
	more := wrapped("")
	more.TextStyle.Italic = true
	more.Hide()
	var update func()
	// sofabatonFetched shows a Sofabaton maker's models once fetched.
	sofabatonFetched = func(b irsource.SofabatonBrand, err error) {
		if err != nil {
			say(false, i18n.T("Getting %s's models from Sofabaton failed: %s", b.Name, err.Error()))
			return
		}
		update()
	}
	update = func() {
		q, typ, mk := search.Text, pick(kind), pick(maker)
		if typ == anyType {
			typ = ""
		}
		if mk == anyMaker {
			mk = ""
		}
		shown = nil
		for _, t := range all {
			// Templates have no type: a type narrows them only to its
			// maker.
			if t.Matches(q) && (typ == "" && mk == "" || mk != "" && strings.EqualFold(t.Brand, mk)) {
				shown = append(shown, t)
			}
		}
		found = nil
		n := 0
		if (len(strings.TrimSpace(q)) >= 2 || typ != "" || mk != "") && a.online.loaded {
			pool := a.online.entries
			if models := a.sofabatonModels(q, mk, sofabatonFetched); len(models) > 0 {
				pool = append(append([]irsource.Entry(nil), models...), pool...)
			}
			found, n = irsource.Browse(pool, typ, mk, q, onlineMax)
		}
		more.SetText(i18n.T("The first %d of %d: pick a maker, or search, to narrow them.", len(found), n))
		more.Hidden = n <= len(found)
		more.Refresh()
		none.Hidden = len(shown)+len(found) > 0
		none.Refresh()
		list.Refresh()
	}
	kind.OnChanged = func(t string) {
		if t == anyType {
			maker.Options = []string{anyMaker}
			maker.Disable()
		} else {
			maker.Options = append([]string{anyMaker}, irsource.Makers(a.online.entries, t)...)
			maker.Enable()
		}
		maker.SetSelected(anyMaker) // updates
		update()
	}
	maker.OnChanged = func(string) { update() }
	search.OnChanged = func(string) { update() }
	list.OnSelected = func(i widget.ListItemID) {
		list.UnselectAll()
		if i < len(shown) {
			a.confirmTemplate(shown[i])
			return
		}
		e := found[i-len(shown)]
		a.importOnline(e, func(busy bool) {
			if busy {
				say(true, i18n.T("Getting %s from %s…", e.Name(), string(e.Source)))
			} else {
				say(false, "")
			}
		})
	}
	say(true, i18n.T("Getting the online lists of models…"))
	a.loadOnline(func() {
		switch {
		case len(a.online.entries) == 0 && a.online.err != nil:
			say(false, i18n.T("Searching online failed: %s", a.online.err.Error()))
		default:
			say(false, "")
		}
		if t := pick(kind); t != anyType {
			maker.Options = append([]string{anyMaker}, irsource.Makers(a.online.entries, t)...)
		}
		update()
	})

	var on []string
	for _, s := range sources {
		if sourceOn(s) {
			on = append(on, string(s))
		}
	}
	names := strings.Join(on, ", ")
	if n := len(on); n > 1 {
		names = i18n.T("%s and %s", strings.Join(on[:n-1], ", "), on[n-1])
	}
	credit := wrapped(i18n.T("Online results come from %s: untested, and the About page says whose they are.", names))
	if len(on) == 0 {
		credit.SetText(i18n.T("No online source is on: the settings turn them on."))
	}
	credit.TextStyle.Italic = true
	narrow := container.NewGridWithColumns(2, kind, maker)
	intro := quiet(wrapped(i18n.T("Pick its model: its commands, keys and power settings come with it, to change before anything is written to the hub.")))
	top := container.NewVBox(intro, search, narrow, alive, status, more, none)
	models := container.NewBorder(top, quiet(credit), nil, nil, list)

	// The two ways, a model to search for or another way, one at a time:
	// the other ways would push the list off a phone's screen.
	ways := scrolled(a.otherWays())
	panes := container.NewStack(models, ways)
	show := func(i int) {
		a.addOther = i == 1
		if a.addOther {
			models.Hide()
			ways.Show()
		} else {
			ways.Hide()
			models.Show()
		}
	}
	pick := 0
	if a.addOther {
		pick = 1
	}
	picker := newSegmented([]string{i18n.T("Search models"), i18n.T("Other ways")}, pick, show)
	show(pick)
	a.screen(i18n.T("Add a device"), a.backToBrowse, column(container.NewPadded(container.NewBorder(picker, nil, nil, nil, panes))))
}

// otherWays are the ways to add a device besides a model's: making it
// from IR codes, from a file of them, as a Bluetooth or a network device,
// or from a service the hub has an integration of.
func (a *App) otherWays() fyne.CanvasObject {
	glyph := func(name string, c color.NRGBA, path string) fyne.Resource { return glyphTile(name, c, path) }
	return container.NewVBox(
		group(i18n.T("Make it yourself"),
			newNavRow(glyph("way-scratch.svg", teal, pencilGlyph), i18n.T("From scratch"), i18n.T("Its IR codes, from its manual or a code chart."), a.showScratch),
			newNavRow(glyph("way-file.svg", slate, folderGlyph), i18n.T("From a file…"), i18n.T("A file of IR codes: Flipper, irdb, LIRC, Pronto and more."), a.importIRFile),
			newNavRow(glyph("way-bluetooth.svg", blue, bluetoothGlyph), i18n.T("Bluetooth"), i18n.T("A device the hub pairs with, as a keyboard: a TV box, a console."), a.showBluetooth),
			newNavRow(glyph("way-network.svg", green, globeGlyph), i18n.T("Network"), i18n.T("Web requests the hub sends: a smart plug, a webhook, a media server."), a.showNetwork)),
		group(i18n.T("Services"),
			newNavRow(glyph("way-hue.svg", amber, bulbGlyph), "Philips Hue", i18n.T("Rooms and lights, through your Hue bridge."), a.showHue),
			newNavRow(glyph("way-roku.svg", purple, tvGlyph), "Roku", i18n.T("Its remote's keys and its channels."), a.showRoku),
			newNavRow(glyph("way-sonos.svg", slate, speakerGlyph), "Sonos", i18n.T("Play, pause, mute and volume of a speaker."), a.showSonos),
			newNavRow(glyph("way-govee.svg", cyan, bulbGlyph), "Govee", i18n.T("Lights, through Govee's cloud."), a.showGovee),
			newNavRow(glyph("way-lifx.svg", violet, bulbGlyph), "LIFX", i18n.T("Lights, through LIFX's cloud."), a.showLIFX)),
		quiet(wrapped(i18n.T("Services' devices are sent their commands by the hub, over your network."))))
}

// onlineMax is the most online results shown.
const onlineMax = 100

// The type and maker pickers' choice of any, in English: shown as
// i18n.T(anyType) and i18n.T(anyMaker).
const (
	anyType  = "Any type"
	anyMaker = "Any maker"
)

// confirmTemplate asks for the new device's name, then adds it.
func (a *App) confirmTemplate(t workspace.Template) {
	name := widget.NewEntry()
	name.SetText(t.Model)
	name.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New(i18n.T("the device needs a name"))
		}
		if o := a.work.Config.Device(s); o != nil {
			return errors.New(i18n.T("there is a device named %q already", o.Name))
		}
		return nil
	}
	items := []*widget.FormItem{widget.NewFormItem(i18n.T("Name"), name)}
	if t.Summary != "" {
		items = append(items, widget.NewFormItem("", wrapped(t.Summary)))
	}
	form := dialog.NewForm(i18n.T("Add %s", t.Model), i18n.T("Add"), i18n.T("Cancel"), items, func(ok bool) {
		if !ok {
			return
		}
		d, err := t.Device(name.Text)
		if err == nil {
			err = a.work.AddDevice(d)
		}
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		s := a.showBrowse(a.hub)
		s.selectDevice(d.Name)
		s.setStatus(false, i18n.T("Added %s.", d.Name))
	}, a.win)
	form.Resize(fyne.NewSize(min(a.win.Canvas().Size().Width-32, 480), 0))
	form.Show()
}

// importIRFile adds a device made of a file of IR codes the user picks,
// once they confirm its name, seeing what it has and what was left out.
func (a *App) importIRFile() {
	a.pickers.open(a.win, workspace.IRFileExts, func(r io.ReadCloser, file string, err error) {
		if err != nil {
			dialog.ShowError(fmt.Errorf("%s: %w", i18n.T("opening failed"), err), a.win)
			return
		}
		im, err := workspace.FromIRFile(file, r)
		_ = r.Close()
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.confirmImport(i18n.T("Add from %s", filepath.Base(file)), im)
	})
}

// confirmImport asks for the name of a device made of a file of IR
// codes, titled title, saying what it has and what was left out, and
// adds it.
func (a *App) confirmImport(title string, im *workspace.Imported) {
	d := im.Device
	name := widget.NewEntry()
	name.SetText(d.Name)
	name.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New(i18n.T("the device needs a name"))
		}
		if o := a.work.Config.Device(s); o != nil {
			return errors.New(i18n.T("there is a device named %q already", o.Name))
		}
		return nil
	}
	about := i18n.T("%d commands, %d of them on the remote's keys.", len(d.Commands), len(d.Buttons))
	if d.Power != nil {
		if d.Power.Off != d.Power.On {
			about += " " + i18n.T("Power: %s and %s.", d.Power.On, d.Power.Off)
		} else {
			about += " " + i18n.T("Power: %s.", d.Power.On)
		}
	}
	items := []*widget.FormItem{widget.NewFormItem(i18n.T("Name"), name), widget.NewFormItem("", wrapped(about))}
	if n := len(im.Skipped); n > 0 {
		const shown = 5
		var lines []string
		for i, s := range im.Skipped {
			if i == shown {
				lines = append(lines, i18n.T("and %d more.", n-shown))
				break
			}
			lines = append(lines, s.Name+": "+s.Why)
		}
		left := wrapped(i18n.T("Left out, as the hub cannot send them yet:\n%s", strings.Join(lines, "\n")))
		left.TextStyle.Italic = true
		items = append(items, widget.NewFormItem("", left))
	}
	items = append(items, widget.NewFormItem("", wrapped(i18n.T("Untested codes: check each one with Test once the device is on the hub."))))
	form := dialog.NewForm(title, i18n.T("Add"), i18n.T("Cancel"), items, func(ok bool) {
		if !ok {
			return
		}
		d.Name = strings.TrimSpace(name.Text)
		if err := a.work.AddDevice(d); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		s := a.showBrowse(a.hub)
		s.selectDevice(d.Name)
		s.setStatus(false, i18n.T("Added %s. Once it is on the hub, test its commands.", d.Name))
	}, a.win)
	form.Resize(fyne.NewSize(min(a.win.Canvas().Size().Width-32, 480), 0))
	form.Show()
}

// backToBrowse shows the browse screen again.
func (a *App) backToBrowse() {
	a.showBrowse(a.hub)
}

// showReview works out and shows what applying the edits writes.
func (a *App) showReview() {
	status := wrapped(i18n.T("Working out the changes…"))
	alive := newPulse()
	body := container.NewVBox(alive, status)
	actions := container.NewVBox()
	alive.Start()
	a.screen(i18n.T("Review changes"), a.backToBrowse, container.NewBorder(nil, actions, nil, nil, scrolled(body)))
	w := a.work
	a.async(func() {
		p, err := w.Plan(context.Background())
		fyne.Do(func() {
			alive.Stop()
			if a.work != w {
				return
			}
			if err != nil {
				status.SetText(i18n.T("These changes cannot be written: %s", err.Error()))
				return
			}
			a.showPlan(body, actions, status, p)
		})
	})
}

// showPlan shows the plan's changes, each with what it does as a chip,
// and Apply and Discard in a bar that stays at the bottom.
func (a *App) showPlan(body, actions *fyne.Container, status *widget.Label, p *workspace.Plan) {
	if len(p.Changes) == 0 {
		status.SetText(i18n.T("Nothing to write: the hub already has all of this."))
		return
	}
	name := a.work.Dump.Info.Name
	if a.offline() {
		status.SetText(i18n.N("Applying makes these changes on %s, in about %d step, if it is as the file has it. Nothing has been written yet.", "Applying makes these changes on %s, in about %d steps, if it is as the file has it. Nothing has been written yet.", p.Writes, name, p.Writes))
	} else {
		status.SetText(i18n.N("Applying makes these changes on %s, in %d step. Nothing has been written yet.", "Applying makes these changes on %s, in %d steps. Nothing has been written yet.", p.Writes, name, p.Writes))
	}
	for _, ch := range p.Changes {
		body.Add(a.changeCard(ch))
	}
	apply := widget.NewButtonWithIcon(i18n.T("Apply to the hub"), theme.ConfirmIcon(), func() { a.applyPlan(p) })
	if a.offline() {
		apply = widget.NewButtonWithIcon(i18n.T("Connect to the hub and apply"), theme.LoginIcon(), a.applyFile)
	}
	apply.Importance = widget.HighImportance
	discard := widget.NewButtonWithIcon(i18n.T("Discard changes"), theme.DeleteIcon(), func() {
		dialog.ShowConfirm(i18n.T("Discard changes?"), i18n.T("The hub is not touched. Undo brings your changes back."), func(ok bool) {
			if !ok {
				return
			}
			if err := a.work.Discard(); err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			a.backToBrowse()
		}, a.win)
	})
	discard.Importance = widget.LowImportance
	p2, g := theme.Padding(), gutter()
	actions.Objects = []fyne.CanvasObject{widget.NewSeparator(), column(container.New(layout.NewCustomPaddedLayout(p2, p2, g, g),
		container.New(&rowOrStack{}, discard, apply)))}
	actions.Refresh()
	body.Refresh()
}

// changeCard is a change: the device's or activity's tile and name, what
// is done to it as a chip, and the hub's writes, folded.
func (a *App) changeCard(ch workspace.Change) fyne.CanvasObject {
	var lead fyne.Resource
	sub := i18n.T("Device")
	if ch.Activity {
		sub = i18n.T("Activity")
	}
	switch {
	case ch.Kind == workspace.Ordered:
		lead = theme.NewPrimaryThemedResource(gripIcon)
		sub = i18n.T("The order the remote lists them in")
	case ch.Activity:
		lead = glyphTile("deleted-activity.svg", slate, profile.Icon(activityIcon).Path())
		if act := a.work.Config.Activity(ch.Name); act != nil {
			lead = tile(act.Icon, activityIcon, act.Color)
		}
	case ch.Name != "":
		lead = glyphTile("deleted-device.svg", slate, profile.Icon(deviceIcon).Path())
		if d := a.work.Config.Device(ch.Name); d != nil {
			lead = tile(d.Icon, deviceIcon, d.Color)
		}
	}
	title := ch.Name
	if title == "" || ch.Kind == workspace.Other {
		title = ch.Title
	}
	var kind *chip
	switch ch.Kind {
	case workspace.Added:
		kind = newChip(i18n.T("Added"), chipNew)
	case workspace.Changed:
		kind = newChip(i18n.T("Changed"), chipEdited)
	case workspace.Deleted:
		kind = newChip(i18n.T("Deleted"), chipRemoved)
	case workspace.Ordered:
		kind = newChip(i18n.T("Reordered"), chipEdited)
	case workspace.Skipped:
		kind = newChip(i18n.T("Not added"), chipRemoved)
		sub = ch.Title
	}
	if len(ch.Details) > 1 {
		sub += " · " + i18n.N("%d step", "%d steps", len(ch.Details), len(ch.Details))
	}
	row := newNavRow(lead, title, sub, nil)
	if kind != nil {
		row.trailing = kind
	}
	parts := []fyne.CanvasObject{row}
	if len(ch.Details) > 1 {
		parts = append(parts, widget.NewAccordion(widget.NewAccordionItem(i18n.T("What the hub is sent"),
			quiet(container.NewVBox(lines(ch.Details)...)))))
	}
	return widget.NewCard("", "", container.NewVBox(parts...))
}

// applyPlan writes the edits to the hub, then reads it back.
func (a *App) applyPlan(p *workspace.Plan) {
	bar := widget.NewProgressBar()
	alive := newPulse()
	step := wrapped(i18n.T("Starting…"))
	note := wrapped(i18n.T("Keep the app open until it is done. The hub takes about a quarter of a second for each command or key, and up to 15 seconds to delete a device."))
	ctx, cancel := context.WithCancel(context.Background())
	stop := widget.NewButtonWithIcon(i18n.T("Stop"), theme.MediaStopIcon(), cancel)
	title := a.page(i18n.T("Writing to the hub"), nil, nil, column(container.NewPadded(container.NewVBox(note, bar, alive, step, container.NewHBox(stop)))), stay)
	a.awake(true)
	alive.Start()

	c, w, h := a.client, a.work, a.hub
	start := a.now()
	a.async(func() {
		defer cancel()
		next, err := w.Apply(ctx, c, p.Writes, func(done, total int, s string) {
			fyne.Do(func() {
				bar.SetValue(float64(done) / float64(max(total, 1)))
				step.SetText(s)
			})
		})
		fyne.Do(func() {
			if a.client != c {
				return
			}
			stop.Hide()
			note.Hide()
			bar.Hide()
			if err != nil {
				title.SetText(i18n.T("Writing stopped"))
				if errors.Is(err, context.Canceled) {
					step.SetText(i18n.T("Stopped. What was written before stays on the hub; reading the hub to show where it is…"))
				} else {
					step.SetText(i18n.T("Writing failed: %s. What was written before stays on the hub; reading the hub to show where it is…", err.Error()))
				}
			} else if next != nil {
				// The hub answered as expected: it holds next.
				alive.Stop()
				a.awake(false)
				a.written(h, next, i18n.T("Your changes are on the hub. Writing took %s.", took(a.now().Sub(start))))
				return
			} else {
				title.SetText(i18n.T("Written"))
				step.SetText(i18n.T("Reading the hub back…"))
			}
			a.readBack(h, w, err, a.now().Sub(start), alive.Stop)
		})
	})
}

// readBack reads the hub after writing, which took wrote, and shows it;
// done is called once read.
func (a *App) readBack(h backend.Hub, old *workspace.Workspace, writeErr error, wrote time.Duration, done func()) {
	c := a.client
	start := a.now()
	a.async(func() {
		w, err := old.ReadBack(context.Background(), c, nil)
		read := a.now().Sub(start)
		fyne.Do(func() {
			done()
			a.awake(false)
			if a.client != c {
				return
			}
			if err != nil {
				a.page(i18n.T("Could not read the hub"), nil, nil, column(container.NewPadded(container.NewVBox(
					wrapped(i18n.T("Reading the hub back failed: %s", readFailure(err))),
					container.NewHBox(widget.NewButtonWithIcon(i18n.T("Reconnect"), theme.ViewRefreshIcon(), func() {
						a.Close()
						a.connect(h)
					}))))), backTo(a.disconnect))
				return
			}
			if writeErr != nil {
				a.written(h, w, i18n.T("Writing stopped part way. This is the hub as it is now; your other changes were not kept."))
			} else {
				a.written(h, w, i18n.T("Your changes are on the hub. Writing took %s, reading back %s.", took(wrote), took(read)))
			}
		})
	})
}

// written shows the hub as it is after writing, w, saying status.
func (a *App) written(h backend.Hub, w *workspace.Workspace, status string) {
	a.work = w
	a.remember(h, w)
	a.showBrowse(h).setStatus(false, status)
}
