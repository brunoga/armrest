package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/sofabaton/irsource"
	"github.com/brunoga/sofabaton/record"
	"github.com/brunoga/sofabaton/sifli"
)

// fakeKey stands in for the catalog's key.
const fakeKey = "0123456789abcdef"

// fakeCatalog is Sofabaton's catalog, with one maker of one model.
type fakeCatalog struct{ key string }

func (c fakeCatalog) Check(context.Context) error {
	if c.key != fakeKey {
		return irsource.ErrSofabatonKey
	}
	return nil
}

func (c fakeCatalog) Brands(context.Context) ([]irsource.SofabatonBrand, error) {
	return []irsource.SofabatonBrand{{ID: "b1", Name: "Acme"}, {ID: "b2", Name: "Zeta Labs"}}, nil
}

func (c fakeCatalog) Models(_ context.Context, b irsource.SofabatonBrand) ([]irsource.Entry, error) {
	if b.ID != "b1" {
		return nil, errors.New("no models")
	}
	return []irsource.Entry{{Source: irsource.Sofabaton, Brand: "Acme", Kind: "Stream Box", Model: "Box 4", Path: "c1.sofabaton", Pattern: "(?i)^bx4.*"}}, nil
}

func (c fakeCatalog) Bluetooth(context.Context) ([]irsource.Entry, error) {
	return []irsource.Entry{{Source: irsource.Sofabaton, Brand: "Zap TV", Kind: irsource.BluetoothKind, Model: "Zap TV", Path: "d272ab7af5404b549eb260d367112165.sofabaton", Classic: true}}, nil
}

func (c fakeCatalog) Fetch(_ context.Context, e irsource.Entry) ([]byte, error) {
	if e.Kind == irsource.BluetoothKind {
		return []byte(`{"code_type": "3", "ecode_id": "d272ab7af5404b549eb260d367112165", "keys": [{"fid": 1, "keyname": "Power", "format": "3", "pulse": "070066"}, {"fid": 146, "keyname": "Play", "format": "3", "pulse": "0C00B0"}], "power": {"type": 1, "on": [{"fid": 1}], "off": [{"fid": 1}]}}`), nil
	}
	return []byte(fmt.Sprintf(`{"keys": [{"keyname": "Power", "format": "1", "irCode": "%X"}]}`, "P:NEC R:38400 D:4 S:251 F:8")), nil
}

// check is the check box labeled text.
func check(t *testing.T, w fyne.Window, text string) *widget.Check {
	t.Helper()
	var found *widget.Check
	walk(w, func(o fyne.CanvasObject) {
		if c, ok := o.(*widget.Check); ok && c.Text == text {
			found = c
		}
	})
	if found == nil {
		t.Fatalf("no check %q in:\n%s", text, texts(w))
	}
	return found
}

// The settings turn sources on and off, Sofabaton's only with a key that
// opens its catalog; searching it finds a maker's models by number.
func TestSettingsSources(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	u.sofabaton = func(key string) sofabatonCatalog { return fakeCatalog{key} }
	var asked []irsource.Source
	u.irIndex = func(ctx context.Context, s irsource.Source) ([]irsource.Entry, error) {
		asked = append(asked, s)
		if s == irsource.Sofabaton {
			return u.cachedIndex(ctx, s)
		}
		return nil, nil
	}
	u.showSettings()
	assertScreens(t, w, "settings")
	if !check(t, w, "irdb").Checked || !check(t, w, "Flipper-IRDB").Checked || check(t, w, "Sofabaton").Checked {
		t.Fatal("not irdb and Flipper-IRDB on, Sofabaton off")
	}
	test.Tap(check(t, w, "irdb"))
	if sourceOn(irsource.IRDB) {
		t.Error("irdb still on")
	}

	// Sofabaton asks for the key, and keeps only one that works.
	test.Tap(check(t, w, "Sofabaton"))
	var key *widget.Entry
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && e.Password {
			key = e
		}
	})
	if key == nil {
		t.Fatalf("no key asked:\n%s", texts(w))
	}
	test.Type(key, "fedcba9876543210")
	test.Tap(button(t, w, "Check and turn on"))
	if !strings.Contains(texts(w), "That key does not open Sofabaton's catalog.") || sourceOn(irsource.Sofabaton) {
		t.Fatalf("a wrong key taken:\n%s", texts(w))
	}
	key.SetText(fakeKey)
	test.Tap(button(t, w, "Check and turn on"))
	if !sourceOn(irsource.Sofabaton) || !check(t, w, "Sofabaton").Checked {
		t.Fatal("the right key did not turn Sofabaton on")
	}

	// Adding a device searches Flipper-IRDB and Sofabaton's makers; a
	// maker named brings its models, found by model number.
	test.Tap(button(t, w, "Back"))
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Add a device…")
	if len(asked) != 2 || asked[0] != irsource.Flipper || asked[1] != irsource.Sofabaton {
		t.Errorf("searched %v", asked)
	}
	var search *widget.Entry
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && search == nil {
			search = e
		}
	})
	search.SetText("acme bx4000")
	var list *widget.List
	walk(w, func(o fyne.CanvasObject) {
		if l, ok := o.(*widget.List); ok {
			list = l
		}
	})
	if got := texts(w); !strings.Contains(got, "Acme Box 4") || !strings.Contains(got, "Stream Box · Sofabaton") || !strings.Contains(got, "Online results come from Flipper-IRDB and Sofabaton") {
		t.Fatalf("results:\n%s", got)
	}
	list.Select(0)
	test.Tap(button(t, w, "Add"))
	d := u.work.Config.Device("Acme Box 4")
	if d == nil || d.Category.String() != "Media player" || len(d.Commands) != 1 {
		t.Fatalf("added %+v", d)
	}

	// Its Bluetooth profiles are found by name, and make Bluetooth
	// devices.
	pickRow(t, u, "Add a device…")
	search = nil
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && search == nil {
			search = e
		}
	})
	search.SetText("zap")
	walk(w, func(o fyne.CanvasObject) {
		if l, ok := o.(*widget.List); ok {
			list = l
		}
	})
	if got := texts(w); !strings.Contains(got, "Zap TV") || !strings.Contains(got, "Bluetooth · Sofabaton") {
		t.Fatalf("results:\n%s", got)
	}
	list.Select(0)
	test.Tap(button(t, w, "Add"))
	d = u.work.Config.Device("Zap TV")
	if d == nil || d.Bluetooth == nil || !d.Bluetooth.Classic || d.Bluetooth.Profile != "d272ab7af5404b549eb260d367112165" || d.Category.String() != "Bluetooth" || d.Power == nil || len(d.Commands) != 2 {
		t.Fatalf("added %+v", d)
	}
}

// The last hub is kept, and connected to on starting when asked.
func TestAutoConnect(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	if h, ok := lastHub(); !ok || h.Addr != lounge || h.Name != "Lounge" {
		t.Fatalf("last hub %+v, %v", h, ok)
	}
	u.disconnect()
	u.Start() // without asking: the hubs
	if u.client != nil {
		t.Fatal("connected unasked")
	}
	u.showSettings()
	if !strings.Contains(texts(w), "The last hub: Lounge (X2, 192.168.1.20).") {
		t.Errorf("settings:\n%s", texts(w))
	}
	test.Tap(check(t, w, "Connect on start"))
	u.Start()
	if u.client == nil || u.hub.Addr != lounge {
		t.Fatal("not connected to the last hub on starting")
	}
}

// fakeFirmware is the firmware the fake catalog offers a hub on a
// lower one; fakeImage its image.
var (
	fakeFirmware = &irsource.Firmware{Product: "x2", Version: 10, Notes: "Fixes bugs", URLs: []string{"https://x/hub.bin"}}
	fakeImage    = bytes.Repeat([]byte("image"), 1000)
)

func (c fakeCatalog) HubFirmware(_ context.Context, info *record.HubInfo) (*irsource.Firmware, error) {
	if c.key != fakeKey {
		return nil, irsource.ErrSofabatonKey
	}
	if int(info.Firmware) >= fakeFirmware.Version {
		return nil, nil
	}
	return fakeFirmware, nil
}

func (c fakeCatalog) HubImage(context.Context, *irsource.Firmware) ([]byte, error) {
	return fakeImage, nil
}

func (c fakeCatalog) RemoteFirmwares(context.Context) ([]*irsource.Firmware, error) {
	return []*irsource.Firmware{{Product: "remote2_release", Version: 10, Notes: "Faster sync"}, {Product: "remote2_release", Version: 9}}, nil
}

func (c fakeCatalog) RemotePackage(context.Context, *irsource.Firmware) (sifli.Package, error) {
	return sifli.Package{Ctrl: []byte("ctrl"), Images: [][]byte{[]byte("image")}}, nil
}
