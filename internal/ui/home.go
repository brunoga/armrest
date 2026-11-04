package ui

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"slices"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/cache"
	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/armrest/internal/workspace"
	"github.com/brunoga/sofabaton/session"
)

// showHome shows the connected hub's activities and devices: at once
// from the cache, then checked against the hub in the background, or,
// for a hub not seen before, once read, showing how far it got.
func (a *App) showHome(h backend.Hub) {
	info := a.client.Info()
	// A file's changes are applied to the hub as it is now: it is read
	// whole.
	if a.cache != nil && !h.Demo && a.file == nil {
		if e := a.cache.Load(hex.EncodeToString(info.ID[:])); e != nil {
			if w, err := workspace.FromDump(context.Background(), e.Dump); err == nil {
				if e.Edits != nil {
					if err := w.LoadEdits(e.Edits); err != nil {
						fyne.LogError("the cached edits", err)
					}
				}
				a.work, a.readAt, a.saved = w, e.Saved, append([]byte{}, e.Edits...)
				a.checkForChanges(h, a.showBrowse(h), e.Saved)
				return
			}
		}
	}

	// The hub's first answers take a few seconds before the bar moves:
	// the line under it shows the read is alive meanwhile, and while each
	// device takes its time.
	bar := widget.NewProgressBar()
	alive := newPulse()
	alive.Start()
	step := wrapped(i18n.T("Asking the hub what it has…"))
	ctx, cancel := context.WithCancel(context.Background())
	cancelBtn := widget.NewButtonWithIcon(i18n.T("Cancel"), theme.CancelIcon(), func() {
		cancel()
		a.disconnect()
	})
	retry := widget.NewButtonWithIcon(i18n.T("Try again"), theme.ViewRefreshIcon(), func() {
		a.Close()
		a.connect(h)
	})
	retry.Importance = widget.HighImportance
	retry.Hide()
	intro := wrapped(i18n.T("This takes a few seconds for each device and activity. Keep the app open until it is done."))
	title := a.page(i18n.T("Reading the hub"), nil, nil, column(container.NewPadded(container.NewVBox(
		intro, bar, alive, step, container.NewHBox(retry, cancelBtn)))), backTo(cancelBtn.OnTapped))
	a.awake(true)

	c := a.client
	a.async(func() {
		defer cancel()
		w, err := workspace.Read(ctx, c, func(done, total int, name string) {
			fyne.Do(func() {
				bar.SetValue(float64(done) / float64(max(total, 1)))
				switch name {
				case "":
				case workspace.Again:
					step.SetText(i18n.T("The hub gave an answer that does not add up: reading it again…"))
				default:
					step.SetText(i18n.T("Reading %s (%d of %d)", name, done+1, total))
				}
			})
		})
		fyne.Do(func() {
			a.awake(false)
			alive.Stop()
			if err == nil {
				a.allChecked(w)
			}
			switch {
			case errors.Is(err, context.Canceled):
				// Cancel went back to the hubs already.
			case err != nil:
				title.SetText(i18n.T("Could not read the hub"))
				intro.Hide()
				bar.Hide()
				step.SetText(readFailure(err))
				cancelBtn.SetText(i18n.T("Back"))
				cancelBtn.SetIcon(theme.NavigateBackIcon())
				retry.Show()
			case a.file != nil:
				a.adoptFile(h, w)
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

// checkForChanges reads the hub's lists in the background and updates s
// if they changed since the hub was read, at since: the devices and
// activities whose row changed are read again, the others are checked as
// they are opened (see check).
func (a *App) checkForChanges(h backend.Hub, s *browseScreen, since time.Time) {
	a.checking = true
	s.showChanges()
	s.setStatus(true, i18n.T("Showing the hub as it was %s. Checking for changes…", ago(since, a.now())))
	c, old := a.client, a.work
	a.async(func() {
		// Writing is safe whatever the cache: an apply reads the devices
		// it edits from the hub, and reads the hub back if it answered
		// otherwise than the cached copy.
		w, err := workspace.RefreshRows(context.Background(), c, old.Dump)
		fyne.Do(func() {
			if a.client != c {
				return // disconnected meanwhile
			}
			a.checking = false
			s := a.browse
			if err != nil {
				s.setStatus(false, i18n.T("Showing the hub as it was %s: checking it for changes failed. %s", ago(since, a.now()), readFailure(err)))
				s.showChanges()
				return
			}
			switch a.adopt(h, w) {
			case adoptedSame:
				s.setStatus(false, i18n.T("Up to date."))
			case adoptedChanged:
				s.setStatus(false, i18n.T("Updated with the hub's changes."))
			}
			s.showChanges()
			a.nextCheck()
		})
	})
}

// How adopt took a fresh read of the hub.
const (
	adoptedSame    = iota // as the workspace had it
	adoptedChanged        // the hub changed: shown as it is
	adoptedAsking         // the hub changed under edits: the user is asked
)

// adopt makes w, the hub read again, the workspace: keeping the edits if
// the hub did not change under them, asking otherwise.
func (a *App) adopt(h backend.Hub, w *workspace.Workspace) int {
	switch {
	case a.work.Same(w) || (a.work.Changed() && a.work.SameBase(w)):
		// Nothing changed on the hub: keep the edits, if any, on the
		// fresh read.
		w.KeepEdits(a.work)
		a.work = w
		a.remember(h, w)
		return adoptedSame
	case !a.work.Changed():
		a.work = w
		a.remember(h, w)
		a.browse.reload()
		return adoptedChanged
	}
	// Remembered once the user chose: the edits are cached until then.
	a.hubChangedUnderEdits(w)
	return adoptedAsking
}

// check reads the device or activity named name from the hub in the
// background, if it was not on this connection, and shows it again if it
// changed: an activity first, then its devices. The cache shows it
// meanwhile.
func (a *App) check(name string) {
	if a.client == nil || a.hub.Demo || a.file != nil || a.work == nil {
		return
	}
	if a.checked == nil {
		a.forgetChecks()
	}
	for _, id := range a.work.IDs(name) {
		if !a.checked[id] && !slices.Contains(a.toCheck, id) {
			a.toCheck = append(a.toCheck, id)
		}
	}
	a.nextCheck()
}

// nextCheck reads the next device or activity waiting to be checked,
// unless the hub is being checked, read or written already.
func (a *App) nextCheck() {
	if a.checkRunning || a.checking || a.working || a.client == nil {
		return
	}
	for len(a.toCheck) > 0 && a.checked[a.toCheck[0]] {
		a.toCheck = a.toCheck[1:]
	}
	if len(a.toCheck) == 0 {
		return
	}
	id := a.toCheck[0]
	a.checkRunning = true
	c, w, h := a.client, a.work, a.hub
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
		defer cancel()
		next, err := w.Check(ctx, c, map[byte]bool{id: true})
		fyne.Do(func() {
			if a.client != c {
				return // disconnected meanwhile: the checks start over
			}
			a.checkRunning = false
			switch {
			case err != nil:
				// Shown from the cache still; opening it again tries again.
				fyne.LogError("checking the hub", err)
				a.toCheck = nil
				return
			case a.work == nil || a.work.Dump != w.Dump:
				// Written or read meanwhile: read again what was read.
				a.nextCheck()
				return
			}
			a.checked[id] = true
			a.toCheck = a.toCheck[1:]
			if a.adopt(h, next) == adoptedChanged {
				a.browse.setStatus(false, i18n.T("Updated with the hub's changes."))
			}
			a.nextCheck()
		})
	})
}

// checkTimeout bounds reading one device or activity: one with many
// commands takes a few seconds.
const checkTimeout = time.Minute

// forgetChecks starts the checks over, for a new connection.
func (a *App) forgetChecks() {
	a.checked, a.toCheck, a.checkRunning = map[byte]bool{}, nil, false
}

// allChecked marks w's devices and activities checked: read whole on this
// connection.
func (a *App) allChecked(w *workspace.Workspace) {
	if a.checked == nil {
		a.forgetChecks()
	}
	for _, e := range w.Dump.Entities {
		a.checked[e.ID] = true
	}
}

// hubChangedUnderEdits asks what to do when the hub changed since it was
// cached and the user has edits made on the cached copy.
func (a *App) hubChangedUnderEdits(fresh *workspace.Workspace) {
	d := dialog.NewCustomConfirm(i18n.T("The hub was changed elsewhere"),
		i18n.T("Start over"), i18n.T("Keep my changes"),
		wrapped(i18n.T("The hub was changed by another app since your copy of it was read. Your changes were made on the older copy, and writing them could undo those changes. Start over from the hub as it is now (your changes are lost), or keep them?")),
		func(startOver bool) {
			if startOver {
				a.work = fresh
				a.remember(a.hub, fresh)
				a.showBrowse(a.hub).setStatus(false, i18n.T("Showing the hub as it is now."))
				return
			}
			// The edits stay, now compared with the hub as it is.
			fresh.KeepEdits(a.work)
			a.work = fresh
			a.remember(a.hub, fresh)
			a.showBrowse(a.hub).setStatus(false, i18n.T("Your changes are kept. Review them carefully before applying."))
		}, a.win)
	d.Show()
}

// ago is how long before now t was, in words.
func ago(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return i18n.T("just now")
	case d < time.Hour:
		n := int(d / time.Minute)
		return i18n.N("%d minute ago", "%d minutes ago", n, n)
	case d < 24*time.Hour:
		n := int(d / time.Hour)
		return i18n.N("%d hour ago", "%d hours ago", n, n)
	default:
		n := int(d / (24 * time.Hour))
		return i18n.N("%d day ago", "%d days ago", n, n)
	}
}

// readFailure is why reading the hub failed, in words.
func readFailure(err error) string {
	if errors.Is(err, session.ErrClosed) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) {
		return i18n.T("The connection to the hub was lost. This happens when the device sleeps or leaves the Wi-Fi network while reading. Try again, and keep the app open until it is done.")
	}
	return i18n.T("Reading the hub failed: %s", err.Error())
}
