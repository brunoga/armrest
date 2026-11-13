package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/brunoga/armrest/internal/backend"
)

// The devices' order changes by dragging, and is applied to the hub.
func TestOrder(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(row(t, w, "Lounge"))
	names := func() []string {
		var out []string
		for _, d := range u.work.Config.Devices {
			out = append(out, d.Name)
		}
		return out
	}
	before := names()
	pickRow(t, u, "Change the order…")
	assertScreens(t, w, "order")
	var grips []*dragHandle
	walk(w, func(o fyne.CanvasObject) {
		if h, ok := o.(*dragHandle); ok {
			grips = append(grips, h)
		}
	})
	if len(grips) != len(before) {
		t.Fatalf("%d handles for %d devices", len(grips), len(before))
	}
	h := grips[0]
	h.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DY: h.row.Size().Height + theme.Padding()}})
	h.DragEnd()
	test.Tap(button(t, w, "Done"))
	after := names()
	if after[0] != before[1] || after[1] != before[0] {
		t.Fatalf("order %v, was %v", after, before)
	}
	if !strings.Contains(texts(w), "Changed the order.") {
		t.Errorf("status:\n%s", texts(w))
	}

	test.Tap(button(t, w, "Review changes"))
	if got := texts(w); !strings.Contains(got, "Change the order of the devices") {
		t.Fatalf("review:\n%s", got)
	}
	test.Tap(button(t, w, "Apply to the hub"))
	if got := texts(w); !strings.Contains(got, "Your changes are on the hub.") || u.work.Changed() {
		t.Fatalf("after applying:\n%s", got)
	}
	if got := names(); got[0] != before[1] || got[1] != before[0] {
		t.Errorf("read back in the order %v", got)
	}
}
