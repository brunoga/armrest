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
				a.checkForChanges(h, a.showBrowse(h), e)
				return
			}
		}
	}

	title := widget.NewLabelWithStyle("Reading "+info.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameHeadingText
	// The hub's first answers take a few seconds before the bar moves:
	// the line under it shows the read is alive meanwhile, and while each
	// device takes its time.
	bar := widget.NewProgressBar()
	alive := newPulse()
	alive.Start()
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
		title, intro, bar, alive, step, container.NewHBox(retry, cancelBtn)))))

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
			alive.Stop()
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
		wrapped("The hub was changed by another app since your copy of it was read. Your changes were made on the older copy, and writing them could undo those changes. Start over from the hub as it is now (your changes are lost), or keep them?"),
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
