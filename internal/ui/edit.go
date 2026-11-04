package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/armrest/internal/workspace"
	"github.com/brunoga/sofabaton/irsource"
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
			return listRow(container.NewVBox(name, sub))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			box := o.(*fyne.Container).Objects[0].(*fyne.Container)
			name, sub := box.Objects[0].(*widget.Label), box.Objects[1].(*widget.Label)
			if i < len(shown) {
				t := shown[i]
				name.SetText(t.Model)
				sub.SetText(i18n.N("%s · %d command", "%s · %d commands", t.Commands, t.Brand, t.Commands))
				return
			}
			e := found[i-len(shown)]
			name.SetText(e.Name())
			sub.SetText(onlineAbout(e))
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

	intro := wrapped(i18n.T("Pick your device's model. Its commands, keys and power settings come with it, and you can change them before anything is written to the hub."))
	scratch := widget.NewButtonWithIcon(i18n.T("From scratch"), theme.DocumentCreateIcon(), a.showScratch)
	fromFile := widget.NewButtonWithIcon(i18n.T("From a file…"), theme.FolderOpenIcon(), a.importIRFile)
	notListed := container.NewVBox(
		wrapped(i18n.T("Not listed? Make it from its IR codes, or from a file of them: Flipper Zero, irdb, SmartIR, LIRC or Girr files, or Pronto, Broadlink and other codes in a .txt.")),
		container.NewThemeOverride(container.NewGridWithColumns(2, scratch, fromFile), tightKeys{}))
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
	top := container.NewVBox(intro, notListed, search, narrow, alive, status, more, none)
	a.screen(i18n.T("Add a device"), a.backToBrowse, column(container.NewPadded(container.NewBorder(top, credit, nil, nil, list))))
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
		s.setStatus(false, i18n.T("Added %s. Review your changes to write them to the hub.", d.Name))
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
		s.setStatus(false, i18n.T("Added %s. Review your changes to write them to the hub, then test its commands.", d.Name))
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
	alive.Start()
	a.screen(i18n.T("Review changes"), a.backToBrowse, scrolled(body))
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
			a.showPlan(body, status, p)
		})
	})
}

func (a *App) showPlan(body *fyne.Container, status *widget.Label, p *workspace.Plan) {
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
		title := ch.Title
		if ch.Warning {
			title = "⚠ " + title
		}
		var detail fyne.CanvasObject = widget.NewLabel("")
		if len(ch.Details) > 0 {
			acc := widget.NewAccordion(widget.NewAccordionItem(
				i18n.N("%d step", "%d steps", len(ch.Details), len(ch.Details)), container.NewVBox(lines(ch.Details)...)))
			detail = acc
		}
		body.Add(card(title, detail))
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
	body.Add(container.NewVBox(apply, discard))
	body.Refresh()
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
