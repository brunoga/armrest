package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/workspace"
	"github.com/brunoga/sofabaton/profile"
)

// A Bluetooth device is made of keys picked by name, the usual ones of a
// TV box's remote to start with.
func TestBluetoothDevice(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "Bluetooth"))
	var keys []*widget.Select
	var entries []*widget.Entry
	walk(w, func(o fyne.CanvasObject) {
		switch e := o.(type) {
		case *widget.Select:
			if e.PlaceHolder == "Key" {
				keys = append(keys, e)
			}
		case *widget.Entry:
			entries = append(entries, e)
		}
	})
	usual := len(workspace.UsualBluetoothKeys)
	// Name, brand, then a name for each command.
	if len(keys) != usual || len(entries) != 2+usual {
		t.Fatalf("%d keys, %d entries:\n%s", len(keys), len(entries), texts(w))
	}
	test.Tap(button(t, w, "Add the device"))
	if got := texts(w); !strings.Contains(got, "The device needs a name.") {
		t.Fatalf("added without a name:\n%s", got)
	}
	test.Type(entries[0], "TV Box")
	entries[2+6].SetText("Return") // Home's
	assertScreens(t, w, "bluetooth")
	test.Tap(button(t, w, "Add the device"))
	if got := texts(w); !strings.Contains(got, "Added TV Box. Keys set from command names:") {
		t.Fatalf("not added:\n%s", got)
	}
	d := u.work.Config.Device("TV Box")
	if d == nil || d.Bluetooth == nil || d.Bluetooth.Profile != workspace.OtherBluetoothProfile || len(d.Commands) != usual || d.Category != 15 {
		t.Fatalf("device: %+v", d)
	}
	if c := d.Commands[6]; c.Name != "Return" || c.Data != "0c0223" {
		t.Errorf("Home's command: %+v", c)
	}
	if d.Buttons["OK"].Short != "OK" || d.Buttons["Vol+"].Short != "Volume Up" {
		t.Errorf("keys: %+v", d.Buttons)
	}
}

// A Bluetooth device's editor picks the key each command presses, and
// adds commands as keys too.
func TestBluetoothDeviceEdit(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	d, err := workspace.BluetoothSpec{Name: "TV Box", Commands: []workspace.BluetoothCommand{{Key: "OK"}, {Key: "Back"}}}.Build()
	if err != nil {
		t.Fatal(err)
	}
	d.Commands = append(d.Commands, profile.Command{Name: "Odd", Data: "0c0043"}) // not a key of ours
	if err := u.work.AddDevice(d); err != nil {
		t.Fatal(err)
	}
	u.showEditDevice("TV Box")
	keys := func() []*widget.Select {
		var out []*widget.Select
		walk(w, func(o fyne.CanvasObject) {
			if s, ok := o.(*widget.Select); ok && s.PlaceHolder == "Key" {
				out = append(out, s)
			}
		})
		return out
	}
	got := keys()
	if len(got) != 3 || got[0].Selected != "OK" || got[1].Selected != "Back" || got[2].Selected != "0c0043" {
		t.Fatalf("%d keys:\n%s", len(got), texts(w))
	}
	walk(w, func(o fyne.CanvasObject) {
		if c, ok := o.(*widget.Card); ok && c.Title == "Inputs" && c.Visible() {
			t.Error("inputs offered")
		}
	})
	got[1].SetSelected("Home")
	test.Tap(button(t, w, "Add")) // with Learn, as the hub is connected
	test.Tap(button(t, w, "Save"))
	if !strings.Contains(texts(w), "Pick the key command 4 presses.") {
		t.Fatalf("a command without a key saved:\n%s", texts(w))
	}
	got = keys()
	got[3].SetSelected("Mute")
	test.Tap(button(t, w, "Save"))
	d = u.work.Config.Device("TV Box")
	want := []profile.Command{{Name: "OK", Data: "0c0041"}, {Name: "Back", Data: "0c0223"}, {Name: "Odd", Data: "0c0043"}, {Name: "Mute", Data: "0c00e2"}}
	if len(d.Commands) != len(want) {
		t.Fatalf("saved %+v\n%s", d.Commands, texts(w))
	}
	for i, c := range want {
		if d.Commands[i] != c {
			t.Errorf("command %d: %+v, want %+v", i, d.Commands[i], c)
		}
	}
}
