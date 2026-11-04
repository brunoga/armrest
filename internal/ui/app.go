// Package ui is the app's screens. It holds no protocol code: it reads
// and writes hubs through the library's client.
package ui

import (
	"context"
	"fmt"
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
	"github.com/brunoga/sofabaton/irsource"
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
	// hubsMenu is the hubs screen's menu.
	hubsMenu *fyne.Menu

	// watch calls dropped once c's connection ends; tests do without.
	watch func(c *hub.Client)
	// background is set while the app is not in the foreground: a phone
	// suspends its network then, and the hub's connection drops.
	background bool
	// working is set while the hub is read or written: those screens
	// handle a dropped connection themselves.
	working bool
	// reconnecting is set while connecting again to a dropped hub.
	reconnecting bool
	// retryWait is the wait between attempts to connect again.
	retryWait time.Duration

	// online is the online sources of IR codes' indexes; irIndex and
	// irFetch get an index and a file (tests fake them).
	online  online
	irIndex func(context.Context, irsource.Source) ([]irsource.Entry, error)
	irFetch func(context.Context, irsource.Entry) ([]byte, error)
}

// New returns the app for win.
func New(win fyne.Window, back backend.Backend) *App {
	// The screens pad themselves: the page's color reaches the edges.
	win.SetPadded(false)
	a := &App{win: win, back: back, async: func(f func()) { go f() }, now: time.Now, pickers: systemPickers, retryWait: 3 * time.Second}
	a.watch = a.watchConnection
	a.irIndex = a.cachedIndex
	a.irFetch = func(ctx context.Context, e irsource.Entry) ([]byte, error) { return irsource.Fetch(ctx, nil, e) }
	return a
}

// UseCache keeps each hub's configuration in c, to show the hub at once
// the next time.
func (a *App) UseCache(c *cache.Cache) { a.cache = c }

// Start shows the first screen and looks for hubs.
func (a *App) Start() {
	a.shortcuts()
	lc := fyne.CurrentApp().Lifecycle()
	lc.SetOnExitedForeground(func() { a.background = true })
	lc.SetOnEnteredForeground(func() {
		a.background = false
		a.resume()
	})
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
	a.working = on
	fyne.CurrentApp().Driver().SetDisableScreenBlanking(on)
}

// connect opens a client of h, with a progress dialog, and shows the home
// screen.
func (a *App) connect(h backend.Hub) {
	alive := newPulse()
	wait := dialog.NewCustomWithoutButtons("Connecting to "+h.Name,
		container.NewVBox(widget.NewLabel("The hub connects back to this device."), alive), a.win)
	wait.Show()
	alive.Start()
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
		defer cancel()
		c, err := a.back.Connect(ctx, h)
		fyne.Do(func() {
			alive.Stop()
			wait.Hide()
			if err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			a.Close()
			a.client, a.hub = c, h
			a.watch(c)
			a.showHome(h)
		})
	})
}

// disconnect ends the connection and goes back to the hubs.
func (a *App) disconnect() {
	a.Close()
	a.showHubs()
}

// watchConnection calls dropped once c's connection ends.
func (a *App) watchConnection(c *hub.Client) {
	done := c.Session().Done()
	go func() {
		<-done
		fyne.Do(func() { a.dropped(c) })
	}()
}

// dropped connects again to the hub whose connection, c's, ended: a phone
// that sleeps drops it. The hub as read and its edits stay meanwhile.
func (a *App) dropped(c *hub.Client) {
	if a.client != c || a.working {
		// Closed, or reading or writing, which say so with Try again.
		return
	}
	_ = c.Close()
	a.client = nil
	a.reconnect()
}

// resume connects again to a hub dropped while the app was away. A
// connection that seems alive is checked first: one dead while the phone
// slept is not seen as dead until used.
func (a *App) resume() {
	if a.work == nil || a.file != nil || a.working {
		return
	}
	c := a.client
	if c == nil {
		a.reconnect()
		return
	}
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
		defer cancel()
		if _, err := c.Devices(ctx); err != nil {
			fyne.Do(func() { a.dropped(c) })
		}
	})
}

// probeTimeout bounds the check of a connection on coming back: a hub
// answers in well under a second.
const probeTimeout = 5 * time.Second

// reconnect connects again to the hub shown, a few times if need be (the
// hub may take a while to see the old connection gone), and checks it
// for changes made meanwhile. In the background it waits for the app to
// come back.
func (a *App) reconnect() {
	if a.background || a.reconnecting || a.browse == nil {
		return
	}
	h, s := a.hub, a.browse
	a.reconnecting = true
	s.setStatus(true, fmt.Sprintf("The connection to %s dropped. Reconnecting…", h.Name))
	a.async(func() {
		var c *hub.Client
		var err error
		for try := 0; try < reconnectTries; try++ {
			if try > 0 {
				time.Sleep(a.retryWait)
			}
			ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
			c, err = a.back.Connect(ctx, h)
			cancel()
			if err == nil {
				break
			}
		}
		fyne.Do(func() {
			a.reconnecting = false
			if a.work == nil || a.client != nil || a.hub != h {
				// Left meanwhile, or connected again some other way.
				if c != nil {
					_ = c.Close()
				}
				return
			}
			if err != nil {
				a.browse.setStatus(false, fmt.Sprintf("Could not reconnect to %s: %v", h.Name, err))
				a.browse.offerReconnect()
				return
			}
			a.client = c
			a.watch(c)
			a.checkForChanges(h, a.browse, a.readAt)
		})
	})
}

// reconnectTries is how many times reconnect tries.
const reconnectTries = 4
