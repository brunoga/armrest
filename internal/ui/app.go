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
	"fyne.io/fyne/v2/driver/mobile"
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
	saved  []byte
	// tab is the browse screen's tab shown last, shown again on coming
	// back from an editor.
	tab int
	// goBack is what Back (Escape on desktop) does on the screen shown.
	// It reports false when there is nowhere to go back to: the app then
	// leaves, as Android apps do.
	goBack func() bool
	// shown is the screen shown.
	shown fyne.CanvasObject
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
	// The screens pad themselves: the page's color reaches the edges.
	win.SetPadded(false)
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
			if s := a.browse; s != nil && a.work != nil && a.shown == s.root {
				f(s)
			}
		}
	}
	undo, redo := on((*browseScreen).undo), on((*browseScreen).redo)
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierShortcutDefault}, undo)
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyZ, Modifier: fyne.KeyModifierShortcutDefault | fyne.KeyModifierShift}, redo)
	c.AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyY, Modifier: fyne.KeyModifierShortcutDefault}, redo)
	c.SetOnTypedKey(func(e *fyne.KeyEvent) {
		if e.Name == fyne.KeyEscape || e.Name == mobile.KeyBack {
			a.pressBack()
		}
	})
}

// show shows content, where Back does back (nil: nowhere to go back to).
func (a *App) show(content fyne.CanvasObject, back func() bool) {
	a.goBack, a.shown = back, content
	a.win.SetContent(container.NewStack(newFill(colorPage), content))
}

// backTo is a Back that always goes to f.
func backTo(f func()) func() bool {
	return func() bool {
		f()
		return true
	}
}

// stay is a Back that does nothing: while writing to the hub, say.
func stay() bool { return true }

// pressBack does what Back does on the screen shown. A dialog takes it
// first: its own buttons close it.
func (a *App) pressBack() {
	if a.win.Canvas().Overlays().Top() != nil {
		return
	}
	if a.goBack != nil && a.goBack() {
		return
	}
	if d, ok := fyne.CurrentApp().Driver().(interface{ GoBack() }); ok {
		d.GoBack()
	}
}

// awake keeps the screen on while the app writes to or reads the hub: a
// phone that sleeps drops the connection.
func (a *App) awake(on bool) {
	fyne.CurrentApp().Driver().SetDisableScreenBlanking(on)
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
