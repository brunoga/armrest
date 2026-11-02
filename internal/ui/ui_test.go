package ui

import (
	"image/color"
	"context"
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
	_, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
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

	l.Select(7) // Projector/Google TV
	if got := texts(w); !strings.Contains(got, "Commands (") || !strings.Contains(got, "Power") {
		t.Errorf("device:\n%s", got)
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

// A hub seen before shows at once from the cache, then is checked for
// changes in the background.
func TestCachedHub(t *testing.T) {
	test.NewTempApp(t)
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(720, 640))
	renamed := false
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
			if renamed {
				m.Activities[0].Name = "Movie Night"
			}
			return hub.Open(ctx, hubsim.Pipe(hubsim.FromModel(m)), hub.Options{})
		},
	}
	u := New(w, back)
	// Work runs when the test says, so the screen can be seen before the
	// check is done.
	var queue []func()
	u.async = func(f func()) { queue = append(queue, f) }
	run := func() {
		for len(queue) > 0 {
			f := queue[0]
			queue = queue[1:]
			f()
		}
	}
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	u.now = func() time.Time { return now }
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

	// The hub changes; the next visit shows the cache at once.
	renamed = true
	now = now.Add(5 * time.Minute)
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	queue[0]() // connect
	queue = queue[1:]
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
