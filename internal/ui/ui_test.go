package ui

import (
	"context"
	"image/color"
	"net/netip"
	"os"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/cache"
	"github.com/brunoga/armrest/internal/demo"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/hubsim"
	"github.com/brunoga/sofabaton/irsource"
	"github.com/brunoga/sofabaton/profile"
	"github.com/brunoga/sofabaton/transport"
	"github.com/brunoga/sofabaton/wire"
)

var lounge = netip.MustParseAddr("192.168.1.20")

// newApp is the app in a test window, with work run in place and a
// network of one hub, the demo's configuration at lounge.
func newApp(t *testing.T, hubs ...backend.Hub) (*App, fyne.Window) {
	t.Helper()
	themedApp(t)
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(720, 640))
	back := backend.Backend{
		Discover: func(context.Context) ([]backend.Hub, error) { return hubs, nil },
		Dial: func(ctx context.Context, addr netip.Addr) (*hub.Client, error) {
			if addr != lounge {
				return nil, transport.ErrRefused
			}
			return demo.Open(ctx)
		},
	}
	u := New(w, back)
	u.watch, u.retryWait = func(*hub.Client) {}, 0 // the tests drop connections
	u.irIndex = func(context.Context, irsource.Source) ([]irsource.Entry, error) { return nil, nil }
	u.async = func(f func()) { f() }
	t.Cleanup(u.Close)
	u.Start()
	return u, w
}

// variant is the default theme fixed to one variant.
type variant struct {
	fyne.Theme
	v fyne.ThemeVariant
}

func (t variant) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return t.Theme.Color(n, t.v)
}

// sizes are the window sizes every screen is checked at: a phone, and a
// tablet or desktop.
var sizes = []struct {
	name string
	size fyne.Size
}{{"phone", fyne.NewSize(360, 740)}, {"wide", fyne.NewSize(1024, 768)}}

// assertScreens compares w with the masters testdata/<name>-<size>-<variant>.png,
// at each of sizes, in light and dark.
func assertScreens(t *testing.T, w fyne.Window, name string) {
	t.Helper()
	defer w.Resize(w.Canvas().Size())
	for _, sz := range sizes {
		w.Resize(sz.size)
		fyne.CurrentApp().Settings().SetTheme(Theme())
		if min := w.Content().MinSize(); min.Width > sz.size.Width {
			t.Errorf("%s at %s size: needs %v wide, more than the %v there is", name, sz.name, min.Width, sz.size.Width)
		}
		for _, v := range []struct {
			name string
			v    fyne.ThemeVariant
		}{{"light", theme.VariantLight}, {"dark", theme.VariantDark}} {
			fyne.CurrentApp().Settings().SetTheme(variant{Theme(), v.v})
			test.AssertRendersToImage(t, name+"-"+sz.name+"-"+v.name+".png", w.Canvas())
		}
	}
}

// walk calls fn on every object on w, overlays (dialogs) included, and on
// every tab's content.
func walk(w fyne.Window, fn func(fyne.CanvasObject)) {
	var visit func(o fyne.CanvasObject)
	visit = func(o fyne.CanvasObject) {
		fn(o)
		switch o := o.(type) {
		case *fyne.Container:
			for _, c := range o.Objects {
				visit(c)
			}
		case *container.AppTabs:
			for _, it := range o.Items {
				visit(it.Content)
			}
		case fyne.Widget:
			for _, c := range test.WidgetRenderer(o).Objects() {
				visit(c)
			}
		}
	}
	visit(w.Content())
	for _, o := range w.Canvas().Overlays().List() {
		visit(o)
	}
}

// texts is every label, button and tab text on w, in order.
func texts(w fyne.Window) string {
	var out []string
	walk(w, func(o fyne.CanvasObject) {
		switch o := o.(type) {
		case *widget.Label:
			out = append(out, o.Text)
		case *widget.Button:
			out = append(out, "["+o.Text+"]")
		case *widget.Card:
			out = append(out, o.Title)
		case *container.AppTabs:
			for _, it := range o.Items {
				out = append(out, "<"+it.Text+">")
			}
		}
	})
	return strings.Join(out, "\n")
}

func button(t *testing.T, w fyne.Window, text string) *widget.Button {
	t.Helper()
	var found *widget.Button
	walk(w, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == text {
			found = b
		}
	})
	if found == nil {
		t.Fatalf("no button %q in:\n%s", text, texts(w))
	}
	return found
}

func TestHubsScreen(t *testing.T) {
	_, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	got := texts(w)
	for _, want := range []string{"[Lounge (X2, 192.168.1.20)]", "1 hub found.", "[Hub not listed?]"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	assertScreens(t, w, "hubs")
}

func TestNoHubs(t *testing.T) {
	_, w := newApp(t)
	if got := texts(w); !strings.Contains(got, "No hub answered.") {
		t.Errorf("no explanation in:\n%s", got)
	}
}

func TestBrowse(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	got := texts(w)
	if !listed(u, "Watch Google TV") || !listed(u, "Receiver") {
		t.Error("not listed")
	}
	for _, want := range []string{"X2 HUB", "X2, firmware 9, at 192.168.1.20", "3 devices and 4 activities", "Pick a device to see"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	assertScreens(t, w, "browse")

	pickRow(t, u, "Watch Google TV")
	got = texts(w)
	for _, want := range []string{"When it starts", "When it ends", "Favorites", "Keys"} {
		if !strings.Contains(got, want) {
			t.Errorf("activity: missing %q in:\n%s", want, got)
		}
	}
	assertScreens(t, w, "activity")

	pickRow(t, u, "Projector/Google TV")
	if got := texts(w); !strings.Contains(got, "Test sends a command from the hub") || !strings.Contains(got, "Power") {
		t.Errorf("device:\n%s", got)
	}
	test.Tap(button(t, w, "Test"))
	if got := texts(w); !strings.Contains(got, ". Did Projector/Google TV respond?") {
		t.Errorf("send:\n%s", got)
	}
	assertScreens(t, w, "device")

	// On a phone, Back goes from the device to the list.
	w.Resize(sizes[0].size)
	for _, o := range u.browse.devices.split.bar.Objects {
		if b, ok := o.(*widget.Button); ok && b.Text == "Back" {
			test.Tap(b)
		}
	}
	if !u.browse.devices.split.list.Visible() {
		t.Error("the list is not back")
	}

	hubAction(t, u, "Disconnect")
	if got := texts(w); !strings.Contains(got, "Hubs on your network") {
		t.Errorf("not back at the hubs:\n%s", got)
	}
}

// pickRow shows the device or activity of that name, in its tab.
func pickRow(t *testing.T, u *App, text string) {
	t.Helper()
	for i, b := range []*browseList{u.browse.devices, u.browse.activities} {
		for j, r := range b.rows {
			if r.text() == text {
				u.browse.tabs.SelectIndex(devicesTab + i)
				b.list.Select(j)
				return
			}
		}
	}
	t.Fatalf("no row %q", text)
}

// A hub that stops answering while it is read (the phone slept, say) is
// explained, and Try again reads it again.
func TestReadFailure(t *testing.T) {
	themedApp(t)
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(720, 640))
	var dials int
	back := backend.Backend{
		Discover: func(context.Context) ([]backend.Hub, error) {
			return []backend.Hub{{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge}}, nil
		},
		Dial: func(ctx context.Context, _ netip.Addr) (*hub.Client, error) {
			dials++
			d, _ := demo.Dump()
			m, err := d.Model()
			if err != nil {
				return nil, err
			}
			h := hubsim.FromModel(m)
			if dials == 1 {
				h.Handle(wire.FamReqButtons, func(wire.Frame) []wire.Frame { return nil })
			}
			return hub.Open(ctx, hubsim.Pipe(h), hub.Options{Timeout: 10 * time.Millisecond})
		},
	}
	u := New(w, back)
	u.watch, u.retryWait = func(*hub.Client) {}, 0 // the tests drop connections
	u.irIndex = func(context.Context, irsource.Source) ([]irsource.Entry, error) { return nil, nil }
	u.async = func(f func()) { f() }
	t.Cleanup(u.Close)
	u.Start()

	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	if got := texts(w); !strings.Contains(got, "Could not read X2 HUB") || !strings.Contains(got, "connection to the hub was lost") {
		t.Fatalf("no explanation:\n%s", got)
	}
	assertScreens(t, w, "read-failure")
	test.Tap(button(t, w, "Try again"))
	if got := texts(w); !strings.Contains(got, "3 devices and 4 activities") {
		t.Errorf("not read again:\n%s", got)
	}
}

// TestReading is the screen before the hub's first answer: the bar at
// nothing, and the line under it sweeping.
func TestReading(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	// Connect, but not read.
	calls := 0
	u.async = func(f func()) {
		if calls++; calls == 1 {
			f()
		}
	}
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	var p *pulse
	walk(w, func(o fyne.CanvasObject) {
		if o, ok := o.(*pulse); ok {
			p = o
		}
	})
	if p == nil || !p.Visible() {
		t.Fatal("no line sweeping")
	}
	// Still, halfway, for the pictures.
	p.anim.Stop()
	p.sweep(0.5)
	assertScreens(t, w, "reading")
}

// cachedApp is the app with a cache, whose work runs when the test calls
// run. The hub's first activity is renamed when *renamed is set.
func cachedApp(t *testing.T) (u *App, w fyne.Window, run func(), step func(), renamed *bool, now *time.Time) {
	t.Helper()
	themedApp(t)
	w = test.NewWindow(nil)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(720, 640))
	renamed = new(bool)
	back := backend.Backend{
		Discover: func(context.Context) ([]backend.Hub, error) {
			return []backend.Hub{{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge}}, nil
		},
		Dial: func(ctx context.Context, _ netip.Addr) (*hub.Client, error) {
			d, _ := demo.Dump()
			m, err := d.Model()
			if err != nil {
				return nil, err
			}
			if *renamed {
				m.Activities[0].Name = "Movie Night"
			}
			return hub.Open(ctx, hubsim.Pipe(hubsim.FromModel(m)), hub.Options{})
		},
	}
	u = New(w, back)
	u.watch, u.retryWait = func(*hub.Client) {}, 0 // the tests drop connections
	u.irIndex = func(context.Context, irsource.Source) ([]irsource.Entry, error) { return nil, nil }
	var queue []func()
	u.async = func(f func()) { queue = append(queue, f) }
	step = func() {
		f := queue[0]
		queue = queue[1:]
		f()
	}
	run = func() {
		for len(queue) > 0 {
			step()
		}
	}
	now = new(time.Time)
	*now = time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	u.now = func() time.Time { return *now }
	u.UseCache(&cache.Cache{Dir: t.TempDir()})
	t.Cleanup(u.Close)
	u.Start()
	run()

	// First visit: read, then cached.
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	run()
	if !listed(u, "Watch Google TV") {
		t.Fatalf("first visit:\n%s", texts(w))
	}
	hubAction(t, u, "Disconnect")
	run()
	return u, w, run, step, renamed, now
}

// A hub seen before shows at once from the cache, then is checked for
// changes in the background.
func TestCachedHub(t *testing.T) {
	u, w, run, step, renamed, now := cachedApp(t)
	// The hub changes; the next visit shows the cache at once.
	*renamed = true
	*now = now.Add(5 * time.Minute)
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	step() // connect
	got := texts(w)
	if !listed(u, "Watch Google TV") || !strings.Contains(got, "as it was 5 minutes ago. Checking for changes") {
		t.Fatalf("from the cache:\n%s", got)
	}
	assertScreens(t, w, "cached")
	run()
	got = texts(w)
	if !listed(u, "Movie Night") || listed(u, "Watch Google TV") || !strings.Contains(got, "Updated with the hub's changes.") {
		t.Errorf("after the check:\n%s", got)
	}
}

// Edits made on the cached copy wait for the check to be reviewed; if the
// hub changed meanwhile, the user picks what to keep.
func TestCachedHubEdited(t *testing.T) {
	u, w, run, step, renamed, _ := cachedApp(t)
	*renamed = true
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	step() // connect, showing the cache
	if err := u.work.Config.RenameDevice("3D Player", "Blu-ray"); err != nil {
		t.Fatal(err)
	}
	u.browse.showChanges()
	if !button(t, w, "Review changes").Disabled() {
		t.Error("review allowed before the hub was checked")
	}
	run()
	if got := texts(w); !strings.Contains(got, "The hub was changed elsewhere") {
		t.Fatalf("no choice offered:\n%s", got)
	}
	test.Tap(button(t, w, "Keep my changes"))
	got := texts(w)
	// The kept copy still has the activity's old name: applying would
	// rename it back, which the list shows as an edit.
	if !strings.Contains(got, "Your changes are kept") || !listed(u, "Blu-ray") || u.browse.state(browseRow{activity: u.work.Config.Activity("Watch Google TV")}) != "Edited" {
		t.Errorf("after keeping:\n%s", got)
	}
	if button(t, w, "Review changes").Disabled() {
		t.Error("review still disabled after the check")
	}
}

// Edits are undone and redone from the browse screen, and outlive the
// app: the next visit to the hub has them, checked against the hub.
func TestUndoAndKeptEdits(t *testing.T) {
	u, w, run, step, _, _ := cachedApp(t)
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	run()
	undo, redo := u.browse.undoBtn, u.browse.redoBtn
	if !undo.Disabled() || !redo.Disabled() {
		t.Fatal("undo or redo before any edit")
	}
	// As the activity editor's Delete does.
	if err := u.work.RemoveActivity("Play PC"); err != nil {
		t.Fatal(err)
	}
	u.backToBrowse()
	undo, redo = u.browse.undoBtn, u.browse.redoBtn
	if undo.Disabled() || !redo.Disabled() || listed(u, "Play PC") {
		t.Fatal("after deleting")
	}
	test.Tap(undo)
	if got := texts(w); !listed(u, "Play PC") || !strings.Contains(got, "Undone: deleting Play PC.") || u.browse.changes.Visible() {
		t.Fatalf("after undoing:\n%s", got)
	}
	if !undo.Disabled() || redo.Disabled() {
		t.Fatal("undo and redo after undoing")
	}
	test.Tap(redo)
	if got := texts(w); listed(u, "Play PC") || !strings.Contains(got, "Redone: deleting Play PC.") || !u.browse.changes.Visible() {
		t.Fatalf("after redoing:\n%s", got)
	}

	// The app is closed and opened again.
	u.Close()
	u.showHubs()
	run()
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	step() // connect, showing the cache
	if got := texts(w); listed(u, "Play PC") || !u.browse.changes.Visible() {
		t.Fatalf("the edits are not back:\n%s", got)
	}
	run()
	if got := texts(w); listed(u, "Play PC") || !strings.Contains(got, "Up to date.") || button(t, w, "Review changes").Disabled() {
		t.Fatalf("after the check:\n%s", got)
	}
}

// hubAction taps the hub tab's button labelled label.
func hubAction(t *testing.T, u *App, label string) {
	t.Helper()
	u.browse.tabs.SelectIndex(hubTab)
	test.Tap(button(t, u.win, label))
}

// listed reports whether the browse list has a row named name.
func listed(u *App, name string) bool {
	for _, b := range []*browseList{u.browse.devices, u.browse.activities} {
		for _, r := range b.rows {
			if r.text() == name {
				return true
			}
		}
	}
	return false
}

// A device added from a template is marked new, reviewed and applied,
// then is on the hub.
func TestAddDevice(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Add a device…")
	got := texts(w)
	if !strings.Contains(got, "Add a device") || !strings.Contains(got, "Pick your device's model") {
		t.Fatalf("not the add screen:\n%s", got)
	}
	var search *widget.Entry
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && search == nil {
			search = e
		}
	})
	test.Type(search, "ugoos")
	assertScreens(t, w, "add-device")
	templates := list(t, w)
	if templates.Length() != 1 {
		t.Fatalf("%d templates match ugoos", templates.Length())
	}
	templates.Select(0)
	test.Tap(button(t, w, "Add"))

	got = texts(w)
	for _, want := range []string{"Ugoos AM6B Plus", "New", "Review changes", "Added Ugoos AM6B Plus."} {
		if !strings.Contains(got, want) {
			t.Errorf("after adding: missing %q in:\n%s", want, got)
		}
	}
	assertScreens(t, w, "device-added")

	test.Tap(button(t, w, "Review changes"))
	if got := texts(w); !strings.Contains(got, "Add device Ugoos AM6B Plus") || !strings.Contains(got, "Nothing has been written yet") {
		t.Fatalf("review:\n%s", got)
	}
	assertScreens(t, w, "review")
	test.Tap(button(t, w, "Apply to the hub"))
	got = texts(w)
	if !strings.Contains(got, "Your changes are on the hub.") || !strings.Contains(got, "Ugoos AM6B Plus") || strings.Contains(got, "\nNew\n") {
		t.Errorf("after applying:\n%s", got)
	}
	if u.work.Changed() {
		t.Error("changes still pending after applying")
	}
}

// A device made from scratch says what is wrong until it is right, then
// is added with its keys set from its command names.
func TestScratchDevice(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "From scratch"))
	if got := texts(w); !strings.Contains(got, "How does the device's remote send its codes?") {
		t.Fatalf("not the scratch screen:\n%s", got)
	}
	var entries []*widget.Entry
	var names []*widget.SelectEntry
	walk(w, func(o fyne.CanvasObject) {
		switch e := o.(type) {
		case *widget.SelectEntry:
			names = append(names, e)
		case *widget.Entry:
			entries = append(entries, e)
		}
	})
	// Name, brand, system code, then the command's code (the Denon
	// fields are not shown).
	if len(entries) < 4 || len(names) != 1 {
		t.Fatalf("%d entries, %d name entries", len(entries), len(names))
	}
	test.Tap(button(t, w, "Add the device"))
	if got := texts(w); !strings.Contains(got, "The device needs a name.") {
		t.Errorf("no problem shown:\n%s", got)
	}
	test.Type(entries[0], "Projector")
	test.Type(entries[2], "0x00CE")
	names[0].SetText("Volume Up")
	test.Type(entries[3], "0x10")
	test.Tap(button(t, w, "Add a command"))
	assertScreens(t, w, "scratch")
	test.Tap(button(t, w, "Add the device"))
	got := texts(w)
	if !strings.Contains(got, "Added Projector. Keys set from command names: Vol+.") || !strings.Contains(got, "New") {
		t.Fatalf("not added:\n%s", got)
	}
	if d := u.work.Config.Device("Projector"); d == nil || len(d.Commands) != 1 {
		t.Errorf("device: %+v", d)
	}
}

func TestDemoHub(t *testing.T) {
	u, w := newApp(t)
	menuItem(t, u, "Try the demo hub")
	if got := texts(w); !strings.Contains(got, "demo hub: a simulated X2, firmware 9") {
		t.Errorf("not the demo hub:\n%s", got)
	}
}

func TestConnectByAddress(t *testing.T) {
	for _, tc := range []struct{ addr, want string }{
		{"192.168.1.20", "3 devices and 4 activities"},
		{"192.168.1.99", "is busy with another app"},
		{"lounge", `"lounge" is not an IPv4 address`},
	} {
		t.Run(tc.addr, func(t *testing.T) {
			_, w := newApp(t)
			test.Tap(button(t, w, "Hub not listed?"))
			var entry *widget.Entry
			walk(w, func(o fyne.CanvasObject) {
				if e, ok := o.(*widget.Entry); ok && entry == nil {
					entry = e
				}
			})
			test.Type(entry, tc.addr)
			test.Tap(button(t, w, "Connect"))
			if got := texts(w); !strings.Contains(got, tc.want) {
				t.Errorf("missing %q in:\n%s", tc.want, got)
			}
		})
	}
}

// A device is edited: renamed, a command renamed, saved, and marked
// edited; the activities follow.
func TestEditDevice(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Receiver")
	test.Tap(editButton(t, w))
	if got := texts(w); !strings.Contains(got, "Edit Receiver") || !strings.Contains(got, "What the remote's keys do") {
		t.Fatalf("not the editor:\n%s", got)
	}
	var entries []*widget.Entry
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok {
			entries = append(entries, e)
		}
	})
	entries[0].SetText("AVR") // the device's name
	for _, e := range entries[2:] {
		if e.Text == "Mute" {
			e.SetText("Silence")
		}
	}
	assertScreens(t, w, "edit-device")

	// A macro of one step.
	test.Tap(button(t, w, "Add a macro"))
	test.Tap(button(t, w, "Command"))
	var macroName *widget.Entry
	var lastPicker *widget.Select
	walk(w, func(o fyne.CanvasObject) {
		switch x := o.(type) {
		case *widget.Entry:
			if x.PlaceHolder == "Macro name" {
				macroName = x
			}
		case *widget.Select:
			lastPicker = x
		}
	})
	macroName.SetText("Quiet")
	lastPicker.SetSelected("Silence")
	test.Tap(button(t, w, "Save"))
	got := texts(w)
	if !strings.Contains(got, "Saved. Review your changes") || !strings.Contains(got, "AVR") || !strings.Contains(got, "Edited") {
		t.Fatalf("after saving:\n%s", got)
	}
	for _, a := range u.work.Config.Activities {
		for _, k := range a.Buttons {
			if strings.HasPrefix(k.Short, "Receiver:") || k.Short == "AVR: Mute" {
				t.Errorf("%s still runs %q", a.Name, k.Short)
			}
		}
	}
	d := u.work.Config.Device("AVR")
	if m := d.Macros[len(d.Macros)-1]; m.Name != "Quiet" || len(m.Steps) != 1 || m.Steps[0].Command != "Silence" {
		t.Errorf("macro: %+v", m)
	}
}

// An activity is edited and saved; a new one is made with the usual
// start and end.
func TestEditActivity(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))

	// Saving what the editor shows, untouched, changes nothing.
	for _, n := range []string{"Watch Google TV", "Play Switch 2", "Watch 3D Movies", "Play PC"} {
		pickRow(t, u, n)
		test.Tap(editButton(t, w))
		test.Tap(button(t, w, "Save"))
		if u.work.Changed() {
			t.Fatalf("saving %s untouched changed it", n)
		}
	}
	for _, n := range []string{"Receiver", "Projector/Google TV", "3D Player"} {
		pickRow(t, u, n)
		test.Tap(editButton(t, w))
		test.Tap(button(t, w, "Save"))
		if u.work.Changed() {
			t.Fatalf("saving %s untouched changed it", n)
		}
	}

	pickRow(t, u, "Play PC")
	test.Tap(editButton(t, w))
	got := texts(w)
	for _, want := range []string{"Edit Play PC", "When it starts", "Favorites", "Add a device"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in the editor:\n%s", want, got)
		}
	}
	assertScreens(t, w, "edit-activity")
	var name *widget.Entry
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && name == nil {
			name = e
		}
	})
	name.SetText("PC Games")
	test.Tap(button(t, w, "Save"))
	if got := texts(w); !strings.Contains(got, "PC Games") || !strings.Contains(got, "Saved.") {
		t.Fatalf("after saving:\n%s", got)
	}
	if u.work.Config.Activity("PC Games") == nil {
		t.Fatal("not renamed")
	}

	// A new activity: one device, the usual start and end.
	pickRow(t, u, "Add an activity…")
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && e.PlaceHolder == "Watch TV" {
			e.SetText("Movie Night")
		}
	})
	test.Tap(button(t, w, "Save"))
	if got := texts(w); !strings.Contains(got, "An activity needs at least one device.") {
		t.Errorf("no device, no problem shown:\n%s", got)
	}
	test.Tap(button(t, w, "Add a device"))
	var device *widget.Select
	walk(w, func(o fyne.CanvasObject) {
		if s, ok := o.(*widget.Select); ok && s.PlaceHolder == "Pick a device" {
			device = s
		}
	})
	device.SetSelected("3D Player")
	test.Tap(button(t, w, "Save"))
	a := u.work.Config.Activity("Movie Night")
	if a == nil || len(a.Devices) != 1 || a.PowerOn != nil {
		t.Fatalf("Movie Night: %+v\n%s", a, texts(w))
	}
	if got := texts(w); !strings.Contains(got, "Movie Night\nNew") {
		t.Errorf("not listed as new:\n%s", got)
	}
}

// overlaySelects are the pickers of the dialog on top.
func overlaySelects(w fyne.Window) []*widget.Select {
	var out []*widget.Select
	var visit func(o fyne.CanvasObject)
	visit = func(o fyne.CanvasObject) {
		switch o := o.(type) {
		case *widget.Select:
			out = append(out, o)
		case *fyne.Container:
			for _, c := range o.Objects {
				visit(c)
			}
		case fyne.Widget:
			for _, c := range test.WidgetRenderer(o).Objects() {
				visit(c)
			}
		}
	}
	visit(w.Canvas().Overlays().Top())
	return out
}

// Keys are set and cleared from the drawing of the remote.
func TestKeyPad(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Receiver")
	test.Tap(editButton(t, w))
	d := u.work.Config.Device("Receiver")
	var free, set string
	for _, row := range remoteRows {
		for _, k := range row {
			if k == "" {
				continue
			}
			if _, ok := d.Buttons[k]; ok {
				set = k
			} else if free == "" {
				free = k
			}
		}
	}
	if free == "" || set == "" {
		t.Fatalf("keys of the Receiver: %v", d.Buttons)
	}
	cmd := d.Commands[0].Name
	drawing(t, u, d.Buttons)

	test.Tap(button(t, w, free))
	ps := overlaySelects(w)
	if len(ps) != 2 {
		t.Fatalf("%d pickers in the key's dialog", len(ps))
	}
	ps[0].SetSelected(cmd)
	assertScreens(t, w, "key")
	test.Tap(button(t, w, "Done"))
	if !strings.Contains(texts(w), free+": "+cmd) {
		t.Fatalf("%s not listed:\n%s", free, texts(w))
	}

	test.Tap(button(t, w, set))
	test.Tap(button(t, w, "Clear"))
	if strings.Contains(texts(w), set+": ") {
		t.Fatalf("%s still listed:\n%s", set, texts(w))
	}

	test.Tap(button(t, w, "Save"))
	d = u.work.Config.Device("Receiver")
	if k := d.Buttons[free]; k.Short != cmd {
		t.Errorf("%s runs %q, not %q", free, k.Short, cmd)
	}
	if _, ok := d.Buttons[set]; ok {
		t.Errorf("%s still set", set)
	}
}

// drawing checks the drawing of the remote, with keys set.
func drawing(t *testing.T, u *App, keys profile.Buttons) {
	t.Helper()
	w := test.NewWindow(nil)
	defer w.Close()
	var cmds []string
	for _, k := range keys {
		cmds = append(cmds, k.Short, k.Long)
	}
	p := newKeyPad(w, "X2", keys, func(s string) *widget.Select {
		return widget.NewSelect(cmds, nil)
	}, func() {})
	for _, r := range p.keys {
		r.short.Selected, r.long.Selected = keys[r.key].Short, keys[r.key].Long
	}
	p.refresh()
	w.SetContent(container.NewVScroll(column(container.NewPadded(p.box))))
	assertScreens(t, w, "keys")
}

// list is the last list on w.
func list(t *testing.T, w fyne.Window) *widget.List {
	t.Helper()
	var l *widget.List
	walk(w, func(o fyne.CanvasObject) {
		if x, ok := o.(*widget.List); ok {
			l = x
		}
	})
	if l == nil {
		t.Fatal("no list")
	}
	return l
}

// Back (Escape on desktop) leaves an editor, then the device shown on a
// phone, then goes to the hub's tab.
func TestBack(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	w.Resize(sizes[0].size)
	back := func() { w.Canvas().OnTypedKey()(&fyne.KeyEvent{Name: fyne.KeyEscape}) }

	pickRow(t, u, "Receiver")
	test.Tap(editButton(t, w))
	back()
	if u.shown != u.browse.root {
		t.Fatal("still in the editor")
	}
	pickRow(t, u, "Receiver")
	back()
	if u.browse.devices.split.showing || u.browse.tabs.SelectedIndex() != devicesTab {
		t.Fatal("still showing the device")
	}
	back()
	if u.browse.tabs.SelectedIndex() != hubTab {
		t.Fatal("not on the hub's tab")
	}
	back() // nowhere to go: the app leaves, on phones
	if u.shown != u.browse.root {
		t.Fatal("left the hub")
	}
}

func TestAbout(t *testing.T) {
	buildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef"}},
			Deps: []*debug.Module{
				{Path: library, Version: "v0.1.1-0.20261102235402-222d9b8f6d5b"},
				{Path: "fyne.io/fyne/v2", Version: "v2.8.2"},
			},
		}, true
	}
	t.Cleanup(func() { buildInfo = debug.ReadBuildInfo })
	u, w := newApp(t)
	menuItem(t, u, "About Armrest")
	got := texts(w)
	for _, want := range []string{"Built from the source, at 0123456", "Bruno Albuquerque", "The Sofabaton library at 222d9b8 (github.com/brunoga/sofabaton), and Fyne v2.8.2."} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	assertScreens(t, w, "about")

	var size *widget.Select
	walk(w, func(o fyne.CanvasObject) {
		if s, ok := o.(*widget.Select); ok {
			size = s
		}
	})
	normal := Scale()
	size.SetSelected("Larger")
	if Scale() <= normal || textSize() != 2 {
		t.Errorf("text size %d, scale %v after Larger, from %v", textSize(), Scale(), normal)
	}
	if !strings.Contains(texts(w), "Bruno Albuquerque") {
		t.Error("left the About page")
	}
	u.pressBack()
	if !strings.Contains(texts(w), "[Hub not listed?]") {
		t.Error("Back did not go to the hubs")
	}
}

func TestAboutTab(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	u.browse.tabs.SelectIndex(aboutTab)
	if !strings.Contains(texts(w), "Bruno Albuquerque") {
		t.Error("no About tab")
	}
	// The tabs' names show where they fit, else only their icons.
	for _, sz := range sizes {
		w.Resize(sz.size)
		named := u.browse.tabs.Items[aboutTab].Text == "About"
		if want := sz.name == "wide"; named != want {
			t.Errorf("at %s size, tabs named: %v", sz.name, named)
		}
	}
	u.pressBack()
	if u.browse.tabs.SelectedIndex() != hubTab {
		t.Error("Back did not go to the hub's tab")
	}
}

// themedApp is a test app in the app's theme: screens are built in it, as
// in the app.
func themedApp(t *testing.T) {
	t.Helper()
	test.NewTempApp(t)
	fyne.CurrentApp().Settings().SetTheme(Theme())
	// Timers do not fire on their own: they would touch the screens from
	// their goroutines. Tests call what they would.
	afterFunc = func(time.Duration, func()) *time.Timer { return time.NewTimer(time.Hour) }
	t.Cleanup(func() { afterFunc = time.AfterFunc })
	// The test app has no icon: the app's.
	icon, err := os.ReadFile("../../cmd/armrest/Icon.png")
	if err != nil {
		t.Fatal(err)
	}
	appIcon = func() fyne.Resource { return fyne.NewStaticResource("Icon.png", icon) }
	t.Cleanup(func() { appIcon = func() fyne.Resource { return fyne.CurrentApp().Icon() } })
}

// Commands and favorites move by dragging their rows' handles.
func TestDragToReorder(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	grips := func() []*dragHandle {
		var out []*dragHandle
		walk(w, func(o fyne.CanvasObject) {
			if h, ok := o.(*dragHandle); ok {
				out = append(out, h)
			}
		})
		return out
	}
	// drag drags h's row down past n rows, or up for a negative n.
	drag := func(h *dragHandle, n int) {
		step := h.row.Size().Height + theme.Padding()
		h.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DY: float32(n) * step}})
		h.DragEnd()
	}

	pickRow(t, u, "Receiver")
	test.Tap(editButton(t, w))
	before := u.work.Config.Device("Receiver").Commands
	first, second, third := before[0].Name, before[1].Name, before[2].Name
	drag(grips()[0], 2)
	test.Tap(button(t, w, "Save"))
	after := u.work.Config.Device("Receiver").Commands
	if got := []string{after[0].Name, after[1].Name, after[2].Name}; got[0] != second || got[1] != third || got[2] != first {
		t.Errorf("after dragging %s down two: %v", first, got)
	}

	pickRow(t, u, "Watch Google TV")
	test.Tap(editButton(t, w))
	test.Tap(button(t, w, "Add a favorite"))
	test.Tap(button(t, w, "Add a favorite"))
	var pickers []*widget.Select
	walk(w, func(o fyne.CanvasObject) {
		if s, ok := o.(*widget.Select); ok {
			pickers = append(pickers, s)
		}
	})
	// The favorites are the editor's last rows.
	a, b := pickers[len(pickers)-2], pickers[len(pickers)-1]
	a.SetSelected(a.Options[1])
	b.SetSelected(a.Options[2])
	all := grips()
	drag(all[len(all)-1], -1)
	test.Tap(button(t, w, "Save"))
	got := u.work.Config.Activity("Watch Google TV").Favorites
	if len(got) != 2 || got[0] != a.Options[2] || got[1] != a.Options[1] {
		t.Errorf("after dragging the last favorite up one: %v, from %v", got, a.Options[1:3])
	}
}

// editButton is the Edit button of the device or activity shown.
func editButton(t *testing.T, w fyne.Window) *iconButton {
	t.Helper()
	var found *iconButton
	walk(w, func(o fyne.CanvasObject) {
		if b, ok := o.(*iconButton); ok && b.Visible() && b.Icon == theme.DocumentCreateIcon() && b.Text == "" && b.Importance == widget.HighImportance {
			found = b
		}
	})
	if found == nil {
		t.Fatalf("no Edit button in:\n%s", texts(w))
	}
	return found
}

// A device's and an activity's remote press their keys on the hub.
func TestRemote(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	key := func(k string) *remoteKey {
		t.Helper()
		var found *remoteKey
		walk(w, func(o fyne.CanvasObject) {
			if r, ok := o.(*remoteKey); ok && r.Text == k {
				found = r
			}
		})
		if found == nil {
			t.Fatalf("no key %s", k)
		}
		return found
	}
	status := func() string { return u.browse.status.Text }

	pickRow(t, u, "Receiver")
	// Held down, a key is lit; let go, it is as it was.
	k := key("Mute")
	k.MouseDown(nil)
	if k.Importance != widget.WarningImportance {
		t.Error("Mute held down is not lit")
	}
	k.since = k.since.Add(-time.Second) // lit long enough
	k.MouseUp(nil)
	if k.Importance != widget.HighImportance {
		t.Errorf("Mute let go: importance %v", k.Importance)
	}
	// A key that works says nothing, as on the remote.
	before := status()
	test.Tap(key("Mute"))
	if status() != before {
		t.Errorf("after Mute: %q", status())
	}
	// Held down long enough, a key is held there and then; letting go
	// does not press it too.
	k = key("Mute")
	k.MouseDown(nil)
	k.holding() // as the timer does
	if status() != "Could not hold Mute: it does nothing on the hub" {
		t.Errorf("Mute held down: %q", status())
	}
	k.MouseUp(nil)
	test.Tap(k)
	if status() != "Could not hold Mute: it does nothing on the hub" {
		t.Errorf("letting go of Mute held: %q", status())
	}
	key("Mute").TappedSecondary(nil) // nothing held: it says so
	if status() != "Could not hold Mute: it does nothing on the hub" {
		t.Errorf("after holding Mute: %q", status())
	}
	if !key("A").Disabled() {
		t.Error("A does nothing, yet is enabled")
	}

	pickRow(t, u, "Watch Google TV")
	// After a failure, a key that works clears it.
	test.Tap(key("Vol+"))
	if status() != "" || u.browse.footer.Visible() {
		t.Errorf("after Vol+: %q", status())
	}
	if !strings.Contains(texts(w), "No favorites.") {
		t.Error("no favorites pane")
	}
}

// A dropped connection is made again, keeping the edits; in the
// background, once the app is back; Reconnect is offered when it fails.
func TestReconnect(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	e, err := u.work.EditActivity("Play PC")
	if err != nil {
		t.Fatal(err)
	}
	e.Name = "Play the PC"
	if err := u.work.SaveActivity(e); err != nil {
		t.Fatal(err)
	}
	status := func() string { return u.browse.status.Text }

	old := u.client
	_ = old.Close()
	u.dropped(old)
	if u.client == nil || u.client == old {
		t.Fatal("not connected again")
	}
	if status() != "Up to date." || u.work.Config.Activity("Play the PC") == nil {
		t.Errorf("after reconnecting: %q, edits kept: %v", status(), u.work.Config.Activity("Play the PC") != nil)
	}

	// Away, it waits for the app to be back.
	u.background = true
	old = u.client
	_ = old.Close()
	u.dropped(old)
	if u.client != nil {
		t.Fatal("connected again in the background")
	}
	u.background = false
	u.resume()
	if u.client == nil {
		t.Fatal("not connected again once back")
	}

	// Back with a connection that died unseen: it is checked, and made
	// again.
	old = u.client
	_ = old.Session().Close()
	u.resume()
	if u.client == nil || u.client == old {
		t.Fatal("a dead connection was not made again on coming back")
	}

	// The hub does not answer: Reconnect is offered, and works once it
	// does.
	dial := u.back.Dial
	u.back.Dial = func(context.Context, netip.Addr) (*hub.Client, error) { return nil, transport.ErrRefused }
	old = u.client
	_ = old.Close()
	u.dropped(old)
	if u.client != nil || !strings.HasPrefix(status(), "Could not reconnect to Lounge") || !u.browse.retry.Visible() {
		t.Fatalf("after failing: %q, Reconnect shown: %v", status(), u.browse.retry.Visible())
	}
	u.back.Dial = dial
	test.Tap(u.browse.retry.Objects[0].(*widget.Button))
	if u.client == nil || status() != "Up to date." {
		t.Errorf("after Reconnect: %q", status())
	}
}

// menuItem picks the item labelled label of the hubs screen's menu.
func menuItem(t *testing.T, u *App, label string) {
	t.Helper()
	for _, it := range u.hubsMenu.Items {
		if it.Label == label {
			it.Action()
			return
		}
	}
	t.Fatalf("no %q in the menu", label)
}
