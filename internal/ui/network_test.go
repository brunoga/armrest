package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
)

// A network device takes plain http addresses to IPv4 addresses, and its
// editor edits them.
func TestNetworkDevice(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "A network device"))
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
	// Name, brand, then the command's address.
	if len(entries) != 3 || len(names) != 1 {
		t.Fatalf("%d entries, %d name entries:\n%s", len(entries), len(names), texts(w))
	}
	test.Type(entries[0], "Lights")
	names[0].SetText("OK")
	test.Type(entries[2], "http://lights.local/on")
	test.Tap(button(t, w, "Add the device"))
	if got := texts(w); !strings.Contains(got, "OK: the address is a plain http:// address") {
		t.Fatalf("a name taken:\n%s", got)
	}
	entries[2].SetText("http://192.168.1.50:8080/on")
	assertScreens(t, w, "network")
	test.Tap(button(t, w, "Add the device"))
	if got := texts(w); !strings.Contains(got, "Added Lights. Keys set from command names: OK.") {
		t.Fatalf("not added:\n%s", got)
	}
	d := u.work.Config.Device("Lights")
	if d == nil || len(d.Commands) != 1 || d.Commands[0].HTTP == nil || d.Commands[0].HTTP.URL != "http://192.168.1.50:8080/on" || d.Category != 16 {
		t.Fatalf("device: %+v", d)
	}

	u.showEditDevice("Lights")
	var url *widget.Entry
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && e.Text == "http://192.168.1.50:8080/on" {
			url = e
		}
	})
	if url == nil {
		t.Fatalf("no address to edit:\n%s", texts(w))
	}
	url.SetText("http://192.168.1.50:8080/toggle")
	test.Tap(button(t, w, "Add a command"))
	test.Tap(button(t, w, "Save"))
	if got := texts(w); !strings.Contains(got, "Command 2 needs a name.") {
		t.Fatalf("an empty command saved:\n%s", got)
	}
	var cmds []*widget.Entry
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && e.PlaceHolder == urlHint {
			cmds = append(cmds, e)
		}
	})
	cmds[1].SetText("http://192.168.1.50:8080/off")
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && e.PlaceHolder == "Name" && e.Text == "" {
			e.SetText("Off")
		}
	})
	test.Tap(button(t, w, "Save"))
	d = u.work.Config.Device("Lights")
	if d == nil || len(d.Commands) != 2 || d.Commands[0].HTTP.URL != "http://192.168.1.50:8080/toggle" || d.Commands[1].HTTP.URL != "http://192.168.1.50:8080/off" {
		t.Fatalf("saved: %+v\n%s", d, texts(w))
	}
}
