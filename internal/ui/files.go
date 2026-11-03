package ui

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/workspace"
)

// openedFile is a workspace opened from a file: the app shows it without
// a hub until it is applied to its hub.
type openedFile struct {
	name  string
	saved time.Time
	work  *workspace.Workspace
}

// pickers are the system's file pickers; tests replace them. Each calls
// back on the UI thread, or not at all when the user cancels. open shows
// files of the extensions given (all of them on phones).
type pickers struct {
	open func(win fyne.Window, exts []string, done func(r io.ReadCloser, name string, err error))
	save func(win fyne.Window, name string, done func(w io.WriteCloser, err error))
}

var systemPickers = pickers{
	open: func(win fyne.Window, exts []string, done func(io.ReadCloser, string, error)) {
		d := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
			switch {
			case err != nil:
				done(nil, "", err)
			case r != nil:
				done(r, r.URI().Name(), nil)
			}
		}, win)
		// Phones' pickers know files by type, and have no type for
		// these: they show every file there.
		if !fyne.CurrentDevice().IsMobile() {
			d.SetFilter(storage.NewExtensionFileFilter(exts))
		}
		d.Show()
	},
	save: func(win fyne.Window, name string, done func(io.WriteCloser, error)) {
		d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			switch {
			case err != nil:
				done(nil, err)
			case w != nil:
				done(w, nil)
			}
		}, win)
		d.SetFileName(name)
		d.Show()
	},
}

// fileName is a name to save the workspace under: the hub's and the day.
func (a *App) fileName() string {
	if a.file != nil && strings.HasSuffix(a.file.name, workspace.Ext) {
		return a.file.name
	}
	name := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, a.work.Dump.Info.Name)
	return fmt.Sprintf("%s %s%s", name, a.now().Format("2006-01-02"), workspace.Ext)
}

// saveToFile saves the workspace, edits included, to a file the user
// picks.
func (a *App) saveToFile() {
	w, s := a.work, a.browse
	a.pickers.save(a.win, a.fileName(), func(out io.WriteCloser, err error) {
		if err == nil {
			err = w.Save(out, a.now())
			if cerr := out.Close(); err == nil {
				err = cerr
			}
		}
		if err != nil {
			dialog.ShowError(fmt.Errorf("saving failed: %w", err), a.win)
			return
		}
		if s != nil && a.browse == s {
			what := "the hub"
			if w.Changed() {
				what = "the hub and your changes"
			}
			s.setStatus(false, fmt.Sprintf("Saved %s to a file.", what))
		}
	})
}

// openFile opens a workspace file, or a hub's dump, the user picks, and
// shows it without a hub.
func (a *App) openFile() {
	a.pickers.open(a.win, []string{workspace.Ext, ".json"}, func(r io.ReadCloser, name string, err error) {
		if err != nil {
			dialog.ShowError(fmt.Errorf("opening failed: %w", err), a.win)
			return
		}
		a.async(func() {
			defer func() { _ = r.Close() }()
			w, saved, err := workspace.Open(context.Background(), r)
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(fmt.Errorf("%s: %w", name, err), a.win)
					return
				}
				a.Close()
				a.file = &openedFile{name: name, saved: saved, work: w}
				a.showFile()
			})
		})
	})
}

// showFile shows the opened file.
func (a *App) showFile() {
	f := a.file
	a.work, a.hub = f.work, backend.Hub{Name: f.work.Dump.Info.Name}
	status := fmt.Sprintf("Opened %s. Changes stay in the app until you apply them to the hub.", f.name)
	a.showBrowse(a.hub).setStatus(false, status)
}

// closeFile forgets the opened file and goes back to the hubs.
func (a *App) closeFile() {
	a.file = nil
	a.disconnect()
}

// offline reports whether the app shows a file rather than a hub.
func (a *App) offline() bool { return a.client == nil && a.file != nil }

// applyFile goes to the hubs, to connect to the file's hub and apply the
// file's changes to it.
func (a *App) applyFile() {
	a.Close()
	a.showHubs()
}

// adoptFile takes the opened file onto w, its hub (or another) just read
// from h, and reviews what applying it writes. On the hub it was saved
// from, as it was then, that is the file's changes; on a hub changed
// since, or on another, applying it restores the file there, which the
// user agrees to first.
func (a *App) adoptFile(h backend.Hub, w *workspace.Workspace) {
	f := a.file
	info := a.client.Info()
	sameHub := hex.EncodeToString(info.ID[:]) == f.work.Dump.Info.ID
	if sameHub && f.work.SameBase(w) {
		a.file = nil
		w.KeepEdits(f.work)
		a.work = w
		a.remember(h, w)
		a.showBrowse(h)
		if w.Changed() {
			a.showReview()
		} else {
			a.browse.setStatus(false, "The hub is as the file has it: there is nothing to apply.")
		}
		return
	}
	title := "The hub changed since"
	text := fmt.Sprintf("%s was changed since %s was saved. Applying the file makes the hub as the file has it again, undoing those changes too.", info.Name, f.name)
	if !sameHub {
		title = "Restore onto another hub"
		text = fmt.Sprintf("%s was saved from another hub, %s. Applying it makes %s hold what the file has: devices and activities are matched by name, those it lacks are added, and its others are deleted.", f.name, f.work.Dump.Info.Name, info.Name)
	}
	text += " You see every change before anything is written."
	dialog.NewCustomConfirm(title, "Review", "Cancel", wrapped(text), func(ok bool) {
		if !ok {
			// Back to the file, as it was.
			a.Close()
			a.showFile()
			return
		}
		if err := w.Restore(f.work.Config, sameHub); err != nil {
			dialog.ShowError(fmt.Errorf("the file cannot be restored: %w", err), a.win)
			return
		}
		a.file = nil
		a.work = w
		a.remember(h, w)
		a.showBrowse(h)
		a.showReview()
	}, a.win).Show()
}
