package ui

import (
	"context"
	"image/color"
	"net/netip"
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
	"github.com/brunoga/sofabaton/transport"
	"github.com/brunoga/sofabaton/wire"
)

var lounge = netip.MustParseAddr("192.168.1.20")

// newApp is the app in a test window, with work run in place and a
// network of one hub, the demo's configuration at lounge.
func newApp(t *testing.T, hubs ...backend.Hub) (*App, fyne.Window) {
	t.Helper()
	test.NewTempApp(t)
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
		if min := w.Content().MinSize(); min.Width > sz.size.Width {
			t.Errorf("%s at %s size: needs %v wide, more than the %v there is", name, sz.name, min.Width, sz.size.Width)
		}
		for _, v := range []struct {
			name string
			v    fyne.ThemeVariant
		}{{"light", theme.VariantLight}, {"dark", theme.VariantDark}} {
			fyne.CurrentApp().Settings().SetTheme(variant{theme.DefaultTheme(), v.v})
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
	for _, want := range []string{"[Lounge (X2, 192.168.1.20)]", "1 found.", "[Open the demo hub]"} {
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
	for _, want := range []string{"X2 HUB", "X2, firmware 9, at 192.168.1.20", "Activities (4)", "Watch Google TV",
		"Devices (3)", "Receiver", "Pick an activity or a device"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	assertScreens(t, w, "browse")

	l := list(t, w)
	l.Select(1) // Watch Google TV
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
	test.Tap(button(t, w, "Back"))
	if l.Visible() != true {
		t.Error("the list is not back")
	}

	test.Tap(button(t, w, "Disconnect"))
	if got := texts(w); !strings.Contains(got, "Hubs on your network") {
		t.Errorf("not back at the hubs:\n%s", got)
	}
}

// pickRow selects the browse list's row of that text.
func pickRow(t *testing.T, u *App, text string) {
	t.Helper()
	for i, r := range u.browse.rows {
		if r.text() == text {
			u.browse.list.Select(i)
			return
		}
	}
	t.Fatalf("no row %q", text)
}

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

// A hub that stops answering while it is read (the phone slept, say) is
// explained, and Try again reads it again.
func TestReadFailure(t *testing.T) {
	test.NewTempApp(t)
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
	u.async = func(f func()) { f() }
	t.Cleanup(u.Close)
	u.Start()

	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	if got := texts(w); !strings.Contains(got, "Could not read X2 HUB") || !strings.Contains(got, "connection to the hub was lost") {
		t.Fatalf("no explanation:\n%s", got)
	}
	assertScreens(t, w, "read-failure")
	test.Tap(button(t, w, "Try again"))
	if got := texts(w); !strings.Contains(got, "Activities (4)") {
		t.Errorf("not read again:\n%s", got)
	}
}

// cachedApp is the app with a cache, whose work runs when the test calls
// run. The hub's first activity is renamed when *renamed is set.
func cachedApp(t *testing.T) (u *App, w fyne.Window, run func(), step func(), renamed *bool, now *time.Time) {
	t.Helper()
	test.NewTempApp(t)
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
	if got := texts(w); !strings.Contains(got, "Watch Google TV") {
		t.Fatalf("first visit:\n%s", got)
	}
	test.Tap(button(t, w, "Disconnect"))
	run()
	return u, w, run, step, renamed, now
}

// A hub seen before shows at once from the cache, then is checked for
// changes in the background.
func TestCachedHub(t *testing.T) {
	_, w, run, step, renamed, now := cachedApp(t)
	// The hub changes; the next visit shows the cache at once.
	*renamed = true
	*now = now.Add(5 * time.Minute)
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	step() // connect
	got := texts(w)
	if !strings.Contains(got, "Watch Google TV") || !strings.Contains(got, "as it was 5 minutes ago. Checking for changes") {
		t.Fatalf("from the cache:\n%s", got)
	}
	assertScreens(t, w, "cached")
	run()
	got = texts(w)
	if !strings.Contains(got, "Movie Night") || strings.Contains(got, "Watch Google TV") || !strings.Contains(got, "Updated with the hub's changes.") {
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
	if !strings.Contains(got, "Your changes are kept") || !strings.Contains(got, "Blu-ray") || !strings.Contains(got, "Watch Google TV\nEdited") {
		t.Errorf("after keeping:\n%s", got)
	}
	if button(t, w, "Review changes").Disabled() {
		t.Error("review still disabled after the check")
	}
}

// A device added from a template is marked new, reviewed and applied,
// then is on the hub.
func TestAddDevice(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	l := list(t, w)
	l.Select(l.Length() - 1) // Add a device…
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
	l := list(t, w)
	l.Select(l.Length() - 1)
	test.Tap(button(t, w, "Start from scratch"))
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
	_, w := newApp(t)
	test.Tap(button(t, w, "Open the demo hub"))
	if got := texts(w); !strings.Contains(got, "Demo hub: a simulated X2, firmware 9") {
		t.Errorf("not the demo hub:\n%s", got)
	}
}

func TestConnectByAddress(t *testing.T) {
	for _, tc := range []struct{ addr, want string }{
		{"192.168.1.20", "Activities (4)"},
		{"192.168.1.99", "is busy with another app"},
		{"lounge", `"lounge" is not an IPv4 address`},
	} {
		t.Run(tc.addr, func(t *testing.T) {
			_, w := newApp(t)
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
	test.Tap(button(t, w, "Edit"))
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
		test.Tap(button(t, w, "Edit"))
		test.Tap(button(t, w, "Save"))
		if u.work.Changed() {
			t.Fatalf("saving %s untouched changed it", n)
		}
	}
	for _, n := range []string{"Receiver", "Projector/Google TV", "3D Player"} {
		pickRow(t, u, n)
		test.Tap(button(t, w, "Edit"))
		test.Tap(button(t, w, "Save"))
		if u.work.Changed() {
			t.Fatalf("saving %s untouched changed it", n)
		}
	}

	pickRow(t, u, "Play PC")
	test.Tap(button(t, w, "Edit"))
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
