package ui

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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
				if e.Edits != nil {
					if err := w.LoadEdits(e.Edits); err != nil {
						fyne.LogError("the cached edits", err)
					}
				}
				a.work, a.readAt, a.saved = w, e.Saved, append([]byte{}, e.Edits...)
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

// remember caches w, the configuration of h just read, with its edits.
func (a *App) remember(h backend.Hub, w *workspace.Workspace) {
	a.readAt, a.saved = a.now(), nil
	a.saveEdits(h, w)
}

// saveEdits caches w's edits with the hub read at readAt, if they changed
// since last saved: the app can be closed at any time (a phone does it on
// its own), and they are back the next time.
func (a *App) saveEdits(h backend.Hub, w *workspace.Workspace) {
	if a.cache == nil || h.Demo || w == nil || a.readAt.IsZero() {
		return
	}
	edits, err := w.Edits()
	if err != nil {
		fyne.LogError("keeping the edits", err)
		return
	}
	if a.saved != nil && bytes.Equal(edits, a.saved) {
		return
	}
	if err := a.cache.Save(cache.Entry{Saved: a.readAt, Dump: w.Dump, Edits: edits}); err != nil {
		fyne.LogError("caching the hub", err)
		return
	}
	a.saved = append([]byte{}, edits...)
}

// checkForChanges reads the hub again in the background, reusing what the
// cached entry has, and updates s if the hub changed.
func (a *App) checkForChanges(h backend.Hub, s *browseScreen, e *cache.Entry) {
	a.checking = true
	s.showChanges()
	s.setStatus(true, fmt.Sprintf("Showing the hub as it was %s. Checking for changes…", ago(e.Saved, a.now())))
	c := a.client
	a.async(func() {
		// Every signal is read again: the hub keeps a command's record when
		// its signal is rewritten, so the cache cannot tell which changed.
		w, err := workspace.Read(context.Background(), c, nil)
		fyne.Do(func() {
			if a.client != c {
				return // disconnected meanwhile
			}
			a.checking = false
			s := a.browse
			switch {
			case err != nil:
				s.setStatus(false, fmt.Sprintf("Showing the hub as it was %s: checking it for changes failed. %s", ago(e.Saved, a.now()), readFailure(err)))
				s.showChanges()
				return
			case a.work.Same(w) || (a.work.Changed() && a.work.SameBase(w)):
				// Nothing changed on the hub: keep the edits, if any, on
				// the fresh read.
				w.KeepEdits(a.work)
				a.work = w
				a.remember(h, w)
				s.setStatus(false, "Up to date.")
			case !a.work.Changed():
				a.work = w
				a.remember(h, w)
				s.reload()
				s.setStatus(false, "Updated with the hub's changes.")
			default:
				// Remembered once the user chose: the edits are cached
				// until then.
				a.hubChangedUnderEdits(w)
			}
			s.showChanges()
		})
	})
}

// hubChangedUnderEdits asks what to do when the hub changed since it was
// cached and the user has edits made on the cached copy.
func (a *App) hubChangedUnderEdits(fresh *workspace.Workspace) {
	d := dialog.NewCustomConfirm("The hub was changed elsewhere",
		"Start over", "Keep my changes",
		wrapped("Since this device last saw it, the hub was changed by another app. Your changes were made on the older copy, and writing them could undo those changes. Start over from the hub as it is now (your changes are lost), or keep them?"),
		func(startOver bool) {
			if startOver {
				a.work = fresh
				a.remember(a.hub, fresh)
				a.showBrowse(a.hub).setStatus(false, "Showing the hub as it is now.")
				return
			}
			// The edits stay, now compared with the hub as it is.
			fresh.KeepEdits(a.work)
			a.work = fresh
			a.remember(a.hub, fresh)
			a.showBrowse(a.hub).setStatus(false, "Your changes are kept. Review them carefully before applying.")
		}, a.win)
	d.Show()
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
	review  *widget.Button
	pending *widget.Label
	// undoBtn and redoBtn take back and make again the last edit.
	undoBtn, redoBtn *widget.Button
	// root is the screen's content.
	root fyne.CanvasObject
	hub  backend.Hub
}

// browseRow is a row of the list: a heading, an activity, a device, or
// the row that adds a device.
type browseRow struct {
	heading     string
	activity    *profile.Activity
	device      *profile.Device
	add         bool
	addActivity bool
}

func (r browseRow) text() string {
	switch {
	case r.activity != nil:
		return r.activity.Name
	case r.device != nil:
		return r.device.Name
	case r.add:
		return "Add a device…"
	case r.addActivity:
		return "Add an activity…"
	}
	return r.heading
}

func (a *App) showBrowse(h backend.Hub) *browseScreen {
	a.saveEdits(h, a.work)
	s := &browseScreen{app: a, hub: h}
	a.browse = s
	s.root = s.content(h)
	a.win.SetContent(s.root)
	return s
}

// undo takes back the last edit.
func (s *browseScreen) undo() {
	if what := s.app.work.Undo(); what != "" {
		s.edited("Undone: " + what + ".")
	}
}

// redo makes the last edit undone again.
func (s *browseScreen) redo() {
	if what := s.app.work.Redo(); what != "" {
		s.edited("Redone: " + what + ".")
	}
}

// edited shows the workspace after an edit made here, saying what it was.
func (s *browseScreen) edited(status string) {
	s.app.saveEdits(s.hub, s.app.work)
	s.reload()
	s.showChanges()
	s.setStatus(false, status)
}

// showUndo enables Undo and Redo when they have something to do.
func (s *browseScreen) showUndo() {
	for _, b := range []struct {
		btn *widget.Button
		can bool
	}{{s.undoBtn, s.app.work.CanUndo()}, {s.redoBtn, s.app.work.CanRedo()}} {
		if b.can {
			b.btn.Enable()
		} else {
			b.btn.Disable()
		}
	}
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
	s.rows = append(s.rows, browseRow{addActivity: true})
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

// sender sends d's commands from the hub, or is nil while d is not on
// the hub.
func (s *browseScreen) sender(d *profile.Device) func(string) {
	if s.app.work.DeviceState(d) == workspace.New {
		return nil
	}
	return func(command string) {
		w, c := s.app.work, s.app.client
		s.setStatus(true, fmt.Sprintf("Sending %s…", command))
		s.app.async(func() {
			ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
			defer cancel()
			err := w.Send(ctx, c, d.Name, command)
			fyne.Do(func() {
				if err != nil {
					s.setStatus(false, fmt.Sprintf("Could not send %s: %v", command, err))
					return
				}
				s.setStatus(false, fmt.Sprintf("Sent %s. Did %s respond?", command, d.Name))
			})
		})
	}
}

// selectActivity shows the activity named name.
func (s *browseScreen) selectActivity(name string) {
	for i, r := range s.rows {
		if r.activity != nil && r.activity.Name == name {
			s.list.Select(i)
			return
		}
	}
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
	s.showUndo()
	if !s.app.work.Changed() {
		s.changes.Hide()
		return
	}
	// Changes are planned against the hub as it is: not before the
	// background check read it.
	if s.app.checking {
		s.review.Disable()
		s.pending.SetText("You have changes that are not on the hub yet. Review them once the hub is checked.")
	} else {
		s.review.Enable()
		s.pending.SetText("You have changes that are not on the hub yet.")
	}
	s.changes.Show()
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
			case r.add, r.addActivity:
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
		if r.addActivity {
			s.list.Unselect(i)
			if len(s.app.work.Config.Devices) == 0 {
				s.setStatus(false, "Add a device first: an activity needs at least one.")
				return
			}
			s.app.showEditActivity("")
			return
		}
		if r.heading == "" {
			s.shown = &r
		}
		switch {
		case r.activity != nil:
			act := r.activity
			s.split.show(act.Name, activityView(act, func() { s.app.showEditActivity(act.Name) }))
		case r.device != nil:
			d := r.device
			s.split.show(d.Name, deviceView(d, s.sender(d), func() { s.app.showEditDevice(d.Name) }))
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

	s.review = widget.NewButtonWithIcon("Review changes", theme.DocumentIcon(), s.app.showReview)
	s.review.Importance = widget.HighImportance
	s.pending = wrapped("")
	s.changes = container.NewBorder(widget.NewSeparator(), nil, nil, container.NewCenter(s.review), s.pending)
	s.undoBtn = widget.NewButtonWithIcon("", theme.ContentUndoIcon(), s.undo)
	s.redoBtn = widget.NewButtonWithIcon("", theme.ContentRedoIcon(), s.redo)
	s.showChanges()
	return container.NewBorder(s.header(h), container.NewVBox(s.changes, s.footer), nil, nil, s.split)
}

func (s *browseScreen) header(h backend.Hub) fyne.CanvasObject {
	info := s.app.client.Info()
	name := widget.NewLabelWithStyle(info.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	name.Truncation = fyne.TextTruncateEllipsis
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
			container.NewVBox(container.NewHBox(s.undoBtn, s.redoBtn,
				widget.NewButtonWithIcon("Disconnect", theme.LogoutIcon(), s.app.disconnect))),
			container.NewVBox(name, sub)),
		widget.NewSeparator())
}
