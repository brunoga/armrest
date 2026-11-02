package ui

import (
	"image/color"
	"context"
	"net/netip"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/demo"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/transport"
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
