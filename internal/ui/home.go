package ui

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/cache"
	"github.com/brunoga/armrest/internal/workspace"
	"github.com/brunoga/sofabaton/profile"
	"github.com/brunoga/sofabaton/session"
)

// showHome shows the connected hub's activities and devices: at once
// from the cache, then checked against the hub in the background, or,
// for a hub not seen before, once read, showing how far it got.
func (a *App) showHome(h backend.Hub) {
	info := a.client.Info()
	if a.cache != nil && !h.Demo {
		if e := a.cache.Load(hex.EncodeToString(info.ID[:])); e != nil {
			if w, err := workspace.FromDump(context.Background(), e.Dump); err == nil {
				a.work = w
				a.checkForChanges(h, a.showBrowse(h), e)
				return
			}
		}
	}

	title := widget.NewLabelWithStyle("Reading "+info.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameHeadingText
	bar := widget.NewProgressBar()
	step := wrapped("Asking the hub what it has…")
	ctx, cancel := context.WithCancel(context.Background())
	cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		cancel()
		a.disconnect()
	})
	retry := widget.NewButtonWithIcon("Try again", theme.ViewRefreshIcon(), func() {
		a.Close()
		a.connect(h)
	})
	retry.Importance = widget.HighImportance
	retry.Hide()
	intro := wrapped("This takes a few seconds for each device and activity. Keep the app open until it is done.")
	a.win.SetContent(column(container.NewPadded(container.NewVBox(
		title, intro, bar, step, container.NewHBox(retry, cancelBtn)))))

	c := a.client
	a.async(func() {
		defer cancel()
		w, err := workspace.Read(ctx, c, func(done, total int, name string) {
			fyne.Do(func() {
				bar.SetValue(float64(done) / float64(max(total, 1)))
				switch name {
				case "":
				case workspace.Again:
					step.SetText("The hub gave an answer that does not add up: reading it again…")
				default:
					step.SetText(fmt.Sprintf("Reading %s (%d of %d)", name, done+1, total))
				}
			})
		})
		fyne.Do(func() {
			switch {
			case errors.Is(err, context.Canceled):
				// Cancel went back to the hubs already.
			case err != nil:
				title.SetText("Could not read " + info.Name)
				intro.Hide()
				bar.Hide()
				step.SetText(readFailure(err))
				cancelBtn.SetText("Back")
				cancelBtn.SetIcon(theme.NavigateBackIcon())
				retry.Show()
			default:
				a.work = w
				a.remember(h, w)
				a.showBrowse(h)
			}
		})
	})
}

// remember caches w, the configuration of h just read.
func (a *App) remember(h backend.Hub, w *workspace.Workspace) {
	if a.cache == nil || h.Demo {
		return
	}
	if err := a.cache.Save(w.Dump, a.now()); err != nil {
		fyne.LogError("caching the hub", err)
	}
}

// checkForChanges reads the hub again in the background, reusing what the
// cached entry has, and updates s if the hub changed.
func (a *App) checkForChanges(h backend.Hub, s *browseScreen, e *cache.Entry) {
	s.setStatus(true, fmt.Sprintf("Showing the hub as it was %s. Checking for changes…", ago(e.Saved, a.now())))
	c := a.client
	a.async(func() {
		w, err := workspace.Refresh(context.Background(), c, e.Dump, nil)
		fyne.Do(func() {
			if a.client != c {
				return // disconnected meanwhile
			}
			switch {
			case err != nil:
				s.setStatus(false, fmt.Sprintf("Showing the hub as it was %s: checking it for changes failed. %s", ago(e.Saved, a.now()), readFailure(err)))
			case a.work.Changed():
				// The user started editing: their edits stay. They were
				// made on the cached copy, which Review says if it matters.
				a.remember(h, w)
				s.setStatus(false, "Checked the hub. Your changes are kept.")
			case a.work.Same(w):
				a.work = w
				a.remember(h, w)
				s.setStatus(false, "Up to date.")
			default:
				a.work = w
				a.remember(h, w)
				s.reload()
				s.setStatus(false, "Updated with the hub's changes.")
			}
		})
	})
}

// ago is how long before now t was, in words.
func ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d/time.Minute), "minute") + " ago"
	case d < 24*time.Hour:
		return plural(int(d/time.Hour), "hour") + " ago"
	default:
		return plural(int(d/(24*time.Hour)), "day") + " ago"
	}
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// readFailure is why reading the hub failed, in words.
func readFailure(err error) string {
	if errors.Is(err, session.ErrClosed) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) {
		return "The connection to the hub was lost. This happens when the device sleeps or leaves the Wi-Fi network while reading. Try again, and keep the app open until it is done."
	}
	return "Reading the hub failed: " + err.Error()
}

// browseScreen lists the activities and devices, and shows the one
// picked.
type browseScreen struct {
	app   *App
	rows  []browseRow
	list  *widget.List
	split *listDetail
	// shown is the row whose detail is shown, if any.
	shown *browseRow

	status *widget.Label
	busy   *widget.Activity
	footer *fyne.Container
	// changes offers to review the edits, when there are any.
	changes *fyne.Container
}

// browseRow is a row of the list: a heading, an activity, a device, or
// the row that adds a device.
type browseRow struct {
	heading  string
	activity *profile.Activity
	device   *profile.Device
	add      bool
}

func (r browseRow) text() string {
	switch {
	case r.activity != nil:
		return r.activity.Name
	case r.device != nil:
		return r.device.Name
	case r.add:
		return "Add a device…"
	}
	return r.heading
}

func (a *App) showBrowse(h backend.Hub) *browseScreen {
	s := &browseScreen{app: a}
	a.win.SetContent(s.content(h))
	return s
}

// setStatus shows text below the list, with an activity indicator while
// busy.
func (s *browseScreen) setStatus(busy bool, text string) {
	s.status.SetText(text)
	if busy {
		s.busy.Show()
		s.busy.Start()
	} else {
		s.busy.Stop()
		s.busy.Hide()
	}
	s.footer.Show()
	s.footer.Refresh()
}

// buildRows lists the workspace's activities and devices.
func (s *browseScreen) buildRows() {
	cfg := s.app.work.Config
	s.rows = s.rows[:0]
	s.rows = append(s.rows, browseRow{heading: fmt.Sprintf("Activities (%d)", len(cfg.Activities))})
	for _, a := range cfg.Activities {
		s.rows = append(s.rows, browseRow{activity: a})
	}
	s.rows = append(s.rows, browseRow{heading: fmt.Sprintf("Devices (%d)", len(cfg.Devices))})
	for _, d := range cfg.Devices {
		s.rows = append(s.rows, browseRow{device: d})
	}
	s.rows = append(s.rows, browseRow{add: true})
}

// state is how the row's device or activity differs from the hub's, in
// words.
func (s *browseScreen) state(r browseRow) string {
	st := workspace.Unchanged
	switch {
	case r.activity != nil:
		st = s.app.work.ActivityState(r.activity)
	case r.device != nil:
		st = s.app.work.DeviceState(r.device)
	}
	switch st {
	case workspace.New:
		return "New"
	case workspace.Edited:
		return "Edited"
	}
	return ""
}

// selectDevice shows the device named name.
func (s *browseScreen) selectDevice(name string) {
	for i, r := range s.rows {
		if r.device != nil && r.device.Name == name {
			s.list.Select(i)
			return
		}
	}
}

// showChanges shows the pending-changes bar if the workspace has edits.
func (s *browseScreen) showChanges() {
	if s.app.work.Changed() {
		s.changes.Show()
	} else {
		s.changes.Hide()
	}
}

// reload shows the workspace again after it changed, keeping the detail
// shown if it is still there.
func (s *browseScreen) reload() {
	shown := s.shown
	s.buildRows()
	s.list.UnselectAll()
	s.list.Refresh()
	if shown == nil {
		return
	}
	for i, r := range s.rows {
		if r.heading == "" && (r.activity != nil) == (shown.activity != nil) && r.text() == shown.text() {
			s.list.Select(i)
			return
		}
	}
	s.split.clear()
}

func (s *browseScreen) content(h backend.Hub) fyne.CanvasObject {
	s.buildRows()
	s.list = widget.NewList(
		func() int { return len(s.rows) },
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.MediaPlayIcon())
			l := widget.NewLabel("")
			l.Truncation = fyne.TextTruncateEllipsis
			state := widget.NewLabel("")
			state.TextStyle.Italic = true
			return container.NewBorder(nil, nil, icon, state, l)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			r := s.rows[i]
			row := o.(*fyne.Container)
			l, icon, state := row.Objects[0].(*widget.Label), row.Objects[1].(*widget.Icon), row.Objects[2].(*widget.Label)
			l.SetText(r.text())
			l.TextStyle.Bold = r.heading != ""
			state.SetText(s.state(r))
			switch {
			case r.activity != nil:
				icon.SetResource(theme.MediaPlayIcon())
				icon.Show()
			case r.device != nil:
				icon.SetResource(theme.ComputerIcon())
				icon.Show()
			case r.add:
				icon.SetResource(theme.ContentAddIcon())
				icon.Show()
			default:
				icon.Hide()
			}
			l.Refresh()
		})
	s.list.OnSelected = func(i widget.ListItemID) {
		r := s.rows[i]
		if r.add {
			s.list.Unselect(i)
			s.app.showAddDevice()
			return
		}
		if r.heading == "" {
			s.shown = &r
		}
		switch {
		case r.activity != nil:
			s.split.show(r.activity.Name, activityView(r.activity))
		case r.device != nil:
			s.split.show(r.device.Name, deviceView(r.device))
		default:
			s.list.Unselect(i)
		}
	}

	s.split = newListDetail(s.list)
	s.split.onBack = func() {
		s.shown = nil
		s.list.UnselectAll()
	}
	hint := wrapped("Pick an activity or a device to see what it does.")
	hint.TextStyle.Italic = true
	s.split.placeholder = container.NewPadded(container.NewVBox(hint))
	s.split.clear()

	s.status = wrapped("")
	s.status.TextStyle.Italic = true
	s.busy = widget.NewActivity()
	s.footer = container.NewBorder(widget.NewSeparator(), nil, s.busy, nil, s.status)
	s.footer.Hide()

	review := widget.NewButtonWithIcon("Review changes", theme.DocumentIcon(), s.app.showReview)
	review.Importance = widget.HighImportance
	pending := wrapped("You have changes that are not on the hub yet.")
	s.changes = container.NewBorder(widget.NewSeparator(), nil, nil, review, pending)
	s.showChanges()
	return container.NewBorder(s.header(h), container.NewVBox(s.changes, s.footer), nil, nil, s.split)
}

func (s *browseScreen) header(h backend.Hub) fyne.CanvasObject {
	info := s.app.client.Info()
	name := widget.NewLabelWithStyle(info.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	detail := fmt.Sprintf("%v, firmware %d", info.Variant, info.Firmware)
	if h.Demo {
		detail = "Demo hub: a simulated " + detail
	} else {
		detail += ", at " + h.Addr.String()
	}
	sub := widget.NewLabel(detail)
	sub.Truncation = fyne.TextTruncateEllipsis
	return container.NewVBox(
		container.NewBorder(nil, nil, nil,
			container.NewVBox(widget.NewButtonWithIcon("Disconnect", theme.LogoutIcon(), s.app.disconnect)),
			container.NewVBox(name, sub)),
		widget.NewSeparator())
}
