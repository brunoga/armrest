package ui

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/sofabaton/hue"
)

// A Hue bridge is found, linked once its button is pressed, and its
// rooms (and lights, if picked) make a device's commands.
func TestHueBridge(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	asked := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api", func(w http.ResponseWriter, r *http.Request) {
		if asked++; asked < 2 {
			_, _ = io.WriteString(w, `[{"error":{"type":101,"description":"link button not pressed"}}]`)
			return
		}
		_, _ = io.WriteString(w, `[{"success":{"username":"KEY"}}]`)
	})
	mux.HandleFunc("GET /api/KEY/groups", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"81":{"name":"Cinema","type":"Room"},"3":{"name":"Backyard","type":"Room"}}`)
	})
	mux.HandleFunc("GET /api/KEY/lights", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"27":{"name":"Pool Table"},"4":{"name":"Arcade"}}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	u.hueHTTP = &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	}}}
	u.hueBridges = func(context.Context) ([]hue.Bridge, error) {
		return []hue.Bridge{{Addr: netip.MustParseAddr("192.168.1.183")}}, nil
	}
	hueLinkPoll = time.Millisecond
	t.Cleanup(func() { hueLinkPoll = 2 * time.Second })

	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "Services"))
	test.Tap(button(t, w, "Philips Hue"))
	test.Tap(button(t, w, "192.168.1.183"))
	if asked != 2 {
		t.Fatalf("asked %d times:\n%s", asked, texts(w))
	}
	checks := func() string {
		var out []string
		walk(w, func(o fyne.CanvasObject) {
			if c, ok := o.(*widget.Check); ok {
				out = append(out, c.Text)
			}
		})
		return strings.Join(out, "|")
	}
	if got := checks(); got != "Backyard|Cinema|Arcade|Pool Table|On and off|10% brightness|50% brightness|80% brightness" || !strings.Contains(texts(w), "10 commands.") {
		t.Fatalf("%s\n%s", got, texts(w))
	}
	// Arcade too, without 10% and 80%.
	walk(w, func(o fyne.CanvasObject) {
		if c, ok := o.(*widget.Check); ok && (c.Text == "Arcade" || c.Text == "Backyard") {
			c.SetChecked(c.Text == "Arcade")
		}
		if c, ok := o.(*widget.Check); ok && (c.Text == "10% brightness" || c.Text == "80% brightness") {
			c.SetChecked(false)
		}
	})
	if !strings.Contains(texts(w), "6 commands.") {
		t.Fatalf("not counted:\n%s", texts(w))
	}
	assertScreens(t, w, "hue")
	test.Tap(button(t, w, "Add the device"))
	d := u.work.Config.Device("Hue Bridge (192.168.1.183)")
	if d == nil || d.Hue == nil || d.Hue.User != "KEY" || len(d.Commands) != 6 || d.Commands[0].Name != "Cinema on" || d.Commands[3].Name != "Arcade on" {
		t.Fatalf("device %+v\n%s", d, texts(w))
	}

	// Another device of the same bridge needs no button: the hub's
	// device has a user for it.
	asked = 0
	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "Services"))
	test.Tap(button(t, w, "Philips Hue"))
	test.Tap(button(t, w, "192.168.1.183"))
	if asked != 0 || !strings.Contains(checks(), "Pool Table") {
		t.Fatalf("linked again (%d):\n%s", asked, texts(w))
	}
	test.Tap(button(t, w, "Add the device"))
	if u.work.Config.Device("Hue Bridge (192.168.1.183) 2") == nil {
		t.Errorf("no second device:\n%s", texts(w))
	}
}
