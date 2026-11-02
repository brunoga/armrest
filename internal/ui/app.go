// Package ui is the app's screens. It holds no protocol code: it reads
// and writes hubs through the library's client.
package ui

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/cache"
	"github.com/brunoga/armrest/internal/workspace"
	"github.com/brunoga/sofabaton/hub"
)

// ConnectTimeout bounds a connection attempt: a hub answers CALL_ME and
// connects back within a few seconds.
const ConnectTimeout = 15 * time.Second

// App is the app's state: one window, at most one hub connection.
type App struct {
	win  fyne.Window
	back backend.Backend
	// async runs work off the UI thread; the work hands its results back
	// with fyne.Do. Tests run it in place.
	async func(func())

	// cache keeps each hub's last configuration; nil caches nothing.
	cache *cache.Cache
	// readAt is when the workspace's hub was read, and saved the edits
	// cached with it.
	readAt time.Time
	// tab is the browse screen's tab shown last, shown again on coming
	// back from an editor.
	tab   int
	saved []byte
	// now is the time; tests fix it.
	now func() time.Time

	client *hub.Client
	// hub is the hub client is connected to.
	hub backend.Hub
	// work is the connected hub's configuration, once read.
	work *workspace.Workspace
	// checking is set while the hub is checked against the cache.
	checking bool
	// browse is the browse screen last shown.
	browse *browseScreen
	// file is the workspace file opened, until it is applied to its hub.
	file *openedFile
	// pickers pick files to open and save.
	pickers pickers
}

// New returns the app for win.
func New(win fyne.Window, back backend.Backend) *App {
	return &App{win: win, back: back, async: func(f func()) { go f() }, now: time.Now, pickers: systemPickers}
}

// UseCache keeps each hub's configuration in c, to show the hub at once
// the next time.
func (a *App) UseCache(c *cache.Cache) { a.cache = c }

// Start shows the first screen and looks for hubs.
func (a *App) Start() {
	a.shortcuts()
	a.showHubs()
}

// Close ends the hub connection, if any. A file opened stays.
func (a *App) Close() {
	if a.client != nil {
		_ = a.client.Close()
		a.client = nil
	}
	a.work = nil
	a.checking = false
	a.readAt, a.saved = time.Time{}, nil
	a.tab = hubTab
}

// shortcuts are the window's keyboard shortcuts: undo and redo, on the
// browse screen.
func (a *App) shortcuts() {
	c := a.win.Canvas()
	on := func(f func(*browseScreen)) func(fyne.Shortcut) {
		return func(fyne.Shortcut) {
			if s := a.browse; s != nil && a.work != nil && a.win.Content() == s.root {
				f(s)
			}
		}
	}
	undo, redo := on((*browseScreen).undo), on((*browseScreen).redo)
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierShortcutDefault}, undo)
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift}, redo)
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyY, Modifier: fyne.KeyModifierShortcutDefault}, redo)
}

// connect opens a client of h, with a progress dialog, and shows the home
// screen.
func (a *App) connect(h backend.Hub) {
	bar := widget.NewProgressBarInfinite()
	wait := dialog.NewCustomWithoutButtons("Connecting to "+h.Name,
		container.NewVBox(widget.NewLabel("The hub connects back to this device."), bar), a.win)
	wait.Show()
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
		defer cancel()
		c, err := a.back.Connect(ctx, h)
		fyne.Do(func() {
			bar.Stop()
			wait.Hide()
			if err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			a.Close()
			a.client, a.hub = c, h
			a.showHome(h)
		})
	})
}

// disconnect ends the connection and goes back to the hubs.
func (a *App) disconnect() {
	a.Close()
	a.showHubs()
}
