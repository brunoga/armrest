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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/sofabaton/roku"
	"github.com/brunoga/sofabaton/sonos"
)

// fakeWiFi serves the Rokus', speakers' and clouds' requests, wherever
// they are for.
func fakeWiFi(t *testing.T, u *App) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /query/device-info", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<device-info><user-device-name>Den</user-device-name></device-info>`)
	})
	mux.HandleFunc("GET /query/apps", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<apps><app id="12">Netflix</app><app id="837">YouTube</app></apps>`)
	})
	mux.HandleFunc("GET /xml/device_description.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<root><device><roomName>Kitchen</roomName></device></root>`)
	})
	mux.HandleFunc("POST /MediaRenderer/RenderingControl/Control", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<s:Envelope><s:Body><u:GetVolumeResponse><CurrentVolume>17</CurrentVolume></u:GetVolumeResponse></s:Body></s:Envelope>`)
	})
	mux.HandleFunc("GET /v1/devices", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Govee-API-Key") != "GK" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `{"code":200,"message":"Success","data":{"devices":[{"device":"AA","model":"H1","deviceName":"Lamp","controllable":true},{"device":"BB","model":"H1","deviceName":"Strip","controllable":true}]}}`)
	})
	mux.HandleFunc("GET /v1/lights/all", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer LT" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, `[{"id":"1","label":"Desk","group":{"id":"g","name":"Office"}},{"id":"2","label":"Bed","group":{"id":"h","name":"Bedroom"}}]`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u.wifiHTTP = &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		},
		// The clouds' https, served plain here.
		DialTLSContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		},
	}}
	u.findRokus = func(context.Context) ([]roku.Player, error) {
		return []roku.Player{{Addr: netip.MustParseAddr("192.168.1.50"), Name: "Den"}}, nil
	}
	u.findSonos = func(context.Context) ([]sonos.Speaker, error) { return nil, nil }
}

func checksOf(w fyne.Window) string {
	var out []string
	walk(w, func(o fyne.CanvasObject) {
		if c, ok := o.(*widget.Check); ok {
			out = append(out, c.Text)
		}
	})
	return strings.Join(out, "|")
}

func setCheck(w fyne.Window, text string, on bool) {
	walk(w, func(o fyne.CanvasObject) {
		if c, ok := o.(*widget.Check); ok && c.Text == text {
			c.SetChecked(on)
		}
	})
}

func setEntry(t *testing.T, w fyne.Window, text string) {
	t.Helper()
	var e *widget.Entry
	walk(w, func(o fyne.CanvasObject) {
		if x, ok := o.(*widget.Entry); ok && e == nil {
			e = x
		}
	})
	if e == nil {
		t.Fatalf("no entry:\n%s", texts(w))
	}
	e.SetText(text)
}

// A Roku found on the network, with one of its channels, and a Sonos
// speaker by its address, become devices.
func TestRokuAndSonos(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	fakeWiFi(t, u)
	test.Tap(row(t, w, "Lounge"))
	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "Services"))
	assertScreens(t, w, "services")
	test.Tap(button(t, w, "Roku"))
	test.Tap(button(t, w, "Den (192.168.1.50)"))
	if got := checksOf(w); got != "Netflix|YouTube" {
		t.Fatalf("%s\n%s", got, texts(w))
	}
	setCheck(w, "YouTube", false)
	assertScreens(t, w, "roku")
	test.Tap(button(t, w, "Add the device"))
	d := u.work.Config.Device("Den")
	if d == nil || d.Roku == nil || d.Roku.Address != "192.168.1.50" || d.Commands[len(d.Commands)-1].Roku != "launch/12" || d.Power == nil {
		t.Fatalf("Roku %+v\n%s", d, texts(w))
	}

	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "Services"))
	test.Tap(button(t, w, "Sonos"))
	if !strings.Contains(texts(w), "No speaker answered") {
		t.Fatalf("found one:\n%s", texts(w))
	}
	setEntry(t, w, "192.168.1.60")
	test.Tap(button(t, w, "Use this address"))
	test.Tap(button(t, w, "Add the device"))
	d = u.work.Config.Device("Kitchen")
	if d == nil || d.Sonos == nil || d.Sonos.Address != "192.168.1.60" || d.Commands[len(d.Commands)-1].Sonos.Body != "17" {
		t.Fatalf("Sonos %+v\n%s", d, texts(w))
	}
	if ed, err := u.work.EditDevice("Kitchen"); err != nil || !ed.Commands[0].Fixed {
		t.Errorf("Sonos edit %+v, %v", ed, err)
	}
}

// Govee's and LIFX's lights, read with the account's key, become devices
// of their commands; a second device takes the key the first holds.
func TestGoveeAndLIFX(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	fakeWiFi(t, u)
	test.Tap(row(t, w, "Lounge"))
	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "Services"))
	test.Tap(button(t, w, "Govee"))
	setEntry(t, w, "bad")
	test.Tap(button(t, w, "Read my lights"))
	if !strings.Contains(texts(w), "Govee did not take the key.") {
		t.Fatalf("bad key:\n%s", texts(w))
	}
	test.Tap(button(t, w, "Another key"))
	setEntry(t, w, "GK")
	test.Tap(button(t, w, "Read my lights"))
	if got := checksOf(w); got != "Lamp|Strip|On and off|10% brightness|50% brightness|80% brightness|100% brightness" || !strings.Contains(texts(w), "12 commands.") {
		t.Fatalf("%s\n%s", got, texts(w))
	}
	setCheck(w, "Strip", false)
	setCheck(w, "10% brightness", false)
	if !strings.Contains(texts(w), "5 commands.") {
		t.Fatalf("not counted:\n%s", texts(w))
	}
	assertScreens(t, w, "govee")
	test.Tap(button(t, w, "Add the device"))
	d := u.work.Config.Device("Govee")
	if d == nil || len(d.Commands) != 5 || d.Commands[0].Name != "Lamp on" || !d.Commands[0].HTTP.Newer() {
		t.Fatalf("Govee %+v\n%s", d, texts(w))
	}
	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "Services"))
	test.Tap(button(t, w, "Govee"))
	test.Tap(button(t, w, "Read my lights")) // the key the first holds
	test.Tap(button(t, w, "Add the device"))
	if u.work.Config.Device("Govee 2") == nil {
		t.Fatalf("no second device:\n%s", texts(w))
	}

	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "Services"))
	test.Tap(button(t, w, "LIFX"))
	setEntry(t, w, "LT")
	test.Tap(button(t, w, "Read my lights"))
	if got := checksOf(w); got != "All lights|Bedroom|Office|Bed|Desk|On and off|10% brightness|50% brightness|80% brightness|100% brightness" || !strings.Contains(texts(w), "6 commands.") {
		t.Fatalf("%s\n%s", got, texts(w))
	}
	setCheck(w, "Office", true)
	test.Tap(button(t, w, "Add the device"))
	d = u.work.Config.Device("LIFX")
	if d == nil || len(d.Commands) != 12 || d.Commands[6].Name != "Office on" || !strings.Contains(d.Commands[6].HTTP.URL, "group_id:g") {
		t.Fatalf("LIFX %+v\n%s", d, texts(w))
	}
}
