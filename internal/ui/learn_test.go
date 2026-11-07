package ui

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/sofabaton/hub"
)

// A command learned from another remote joins the editor, named for the
// user to change, and saving keeps its code.
func TestLearnCommand(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Receiver")
	test.Tap(editButton(t, w))

	code := []byte{0x00, 0x08, 0, 0, 0, 0, 0x94, 0x70, 0, 0, 0x23, 0x28, 0, 0, 0x11, 0x94, 0, 0, 0, 0}
	var run func()
	u.async = func(f func()) { run = f }
	u.learn = func(_ context.Context, _ *hub.Client, failed func()) ([]byte, error) {
		failed()
		return code, nil
	}
	test.Tap(button(t, w, "Learn"))
	if got := texts(w); !strings.Contains(got, "press the key once, briefly") {
		t.Fatalf("no learning dialog:\n%s", got)
	}
	assertScreens(t, w, "learn")
	run()
	if got := texts(w); strings.Contains(got, "Learning a command") {
		t.Fatalf("the dialog stayed:\n%s", got)
	}
	var learned *widget.Entry
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && e.Text == "Learned 1" {
			learned = e
		}
	})
	if learned == nil {
		t.Fatalf("no learned command:\n%s", texts(w))
	}
	learned.SetText("Zone 2 On")
	test.Tap(button(t, w, "Save"))
	d := u.work.Config.Device("Receiver")
	for _, c := range d.Commands {
		if c.Name == "Zone 2 On" {
			if c.Data != hex.EncodeToString(code) || c.Code != nil {
				t.Errorf("saved as %+v", c)
			}
			return
		}
	}
	t.Errorf("not saved:\n%s", texts(w))
}
