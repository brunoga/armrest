package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/workspace"
)

// screen is a full-window page with a title, Back, and content.
func (a *App) screen(title string, back func(), content fyne.CanvasObject) {
	t := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	t.Truncation = fyne.TextTruncateEllipsis
	bar := container.NewBorder(nil, widget.NewSeparator(),
		widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), back), nil, t)
	a.win.SetContent(container.NewBorder(bar, nil, nil, nil, content))
}

// showAddDevice lets the user pick a template for a new device.
func (a *App) showAddDevice() {
	all := workspace.Templates()
	shown := all
	search := widget.NewEntry()
	search.SetPlaceHolder("Search by brand or model")
	list := widget.NewList(
		func() int { return len(shown) },
		func() fyne.CanvasObject {
			name := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
			name.Truncation = fyne.TextTruncateEllipsis
			sub := widget.NewLabel("")
			sub.Truncation = fyne.TextTruncateEllipsis
			return container.NewVBox(name, sub)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			t := shown[i]
			box := o.(*fyne.Container)
			box.Objects[0].(*widget.Label).SetText(t.Model)
			box.Objects[1].(*widget.Label).SetText(fmt.Sprintf("%s · %d commands", t.Brand, t.Commands))
		})
	none := wrapped("No device matches. Try fewer words, or start from scratch.")
	none.TextStyle.Italic = true
	none.Hide()
	search.OnChanged = func(s string) {
		shown = nil
		for _, t := range all {
			if t.Matches(s) {
				shown = append(shown, t)
			}
		}
		none.Hidden = len(shown) > 0
		none.Refresh()
		list.Refresh()
	}
	list.OnSelected = func(i widget.ListItemID) {
		list.UnselectAll()
		a.confirmTemplate(shown[i])
	}

	intro := wrapped("Pick your device's model. Its commands, keys and power settings come with it, and you can change them before anything is written to the hub.")
	scratch := widget.NewButtonWithIcon("Start from scratch", theme.DocumentCreateIcon(), a.showScratch)
	notListed := container.NewBorder(nil, nil, nil, container.NewCenter(scratch), wrapped("Not listed? Make it from its IR codes."))
	top := container.NewVBox(intro, notListed, search, none)
	a.screen("Add a device", a.backToBrowse, column(container.NewPadded(container.NewBorder(top, nil, nil, nil, list))))
}

// confirmTemplate asks for the new device's name, then adds it.
func (a *App) confirmTemplate(t workspace.Template) {
	name := widget.NewEntry()
	name.SetText(t.Model)
	name.Validator = func(s string) error {
		if strings.TrimSpace(s) == "" {
			return errors.New("the device needs a name")
		}
		if o := a.work.Config.Device(s); o != nil {
			return fmt.Errorf("there is a device named %q already", o.Name)
		}
		return nil
	}
	items := []*widget.FormItem{widget.NewFormItem("Name", name)}
	if t.Summary != "" {
		items = append(items, widget.NewFormItem("", wrapped(t.Summary)))
	}
	form := dialog.NewForm("Add "+t.Model, "Add", "Cancel", items, func(ok bool) {
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
		s.setStatus(false, fmt.Sprintf("Added %s. Review your changes to write them to the hub.", d.Name))
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
	status := wrapped("Working out the changes…")
	body := container.NewVBox(status)
	a.screen("Review changes", a.backToBrowse, container.NewVScroll(column(container.NewPadded(body))))
	w := a.work
	a.async(func() {
		p, err := w.Plan(context.Background())
		fyne.Do(func() {
			if a.work != w {
				return
			}
			if err != nil {
				status.SetText("These changes cannot be written: " + err.Error())
				return
			}
			a.showPlan(body, status, p)
		})
	})
}

func (a *App) showPlan(body *fyne.Container, status *widget.Label, p *workspace.Plan) {
	if len(p.Changes) == 0 {
		status.SetText("Nothing to write: the hub already has all of this.")
		return
	}
	status.SetText(fmt.Sprintf("Applying makes these changes on %s, in %d steps. Nothing has been written yet.", a.client.Info().Name, p.Writes))
	for _, ch := range p.Changes {
		title := ch.Title
		if ch.Warning {
			title = "⚠ " + title
		}
		var detail fyne.CanvasObject = widget.NewLabel("")
		if len(ch.Details) > 0 {
			acc := widget.NewAccordion(widget.NewAccordionItem(
				fmt.Sprintf("%d steps", len(ch.Details)), container.NewVBox(lines(ch.Details)...)))
			detail = acc
		}
		body.Add(card(title, detail))
	}
	apply := widget.NewButtonWithIcon("Apply to the hub", theme.ConfirmIcon(), func() { a.applyPlan(p) })
	apply.Importance = widget.HighImportance
	discard := widget.NewButtonWithIcon("Discard changes", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("Discard changes?", "Your changes will be lost. The hub is not touched.", func(ok bool) {
			if !ok {
				return
			}
			if err := a.work.Discard(context.Background()); err != nil {
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
	title := widget.NewLabelWithStyle("Writing to "+a.client.Info().Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameHeadingText
	bar := widget.NewProgressBar()
	step := wrapped("Starting…")
	note := wrapped("Keep the app open until it is done. Adding or deleting a device takes the hub a minute or two.")
	ctx, cancel := context.WithCancel(context.Background())
	stop := widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), cancel)
	a.win.SetContent(column(container.NewPadded(container.NewVBox(title, note, bar, step, container.NewHBox(stop)))))

	c, w, h := a.client, a.work, a.hub
	a.async(func() {
		defer cancel()
		err := w.Apply(ctx, c, p.Writes, func(done, total int, s string) {
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
				title.SetText("Writing stopped")
				if errors.Is(err, context.Canceled) {
					step.SetText("Stopped. What was written before stays on the hub; reading the hub to show where it is…")
				} else {
					step.SetText("Writing failed: " + err.Error() + ". What was written before stays on the hub; reading the hub to show where it is…")
				}
			} else {
				title.SetText("Written")
				step.SetText("Reading the hub back…")
			}
			a.readBack(h, w, err)
		})
	})
}

// readBack reads the hub after writing, and shows it.
func (a *App) readBack(h backend.Hub, old *workspace.Workspace, writeErr error) {
	c := a.client
	a.async(func() {
		w, err := workspace.Refresh(context.Background(), c, old.Dump, nil)
		fyne.Do(func() {
			if a.client != c {
				return
			}
			if err != nil {
				a.win.SetContent(column(container.NewPadded(container.NewVBox(
					wrapped("Reading the hub back failed: "+readFailure(err)),
					container.NewHBox(widget.NewButtonWithIcon("Reconnect", theme.ViewRefreshIcon(), func() {
						a.Close()
						a.connect(h)
					}))))))
				return
			}
			a.work = w
			a.remember(h, w)
			s := a.showBrowse(h)
			if writeErr != nil {
				s.setStatus(false, "Writing stopped part way. This is the hub as it is now; your other changes were not kept.")
			} else {
				s.setStatus(false, "Your changes are on the hub.")
			}
		})
	})
}
