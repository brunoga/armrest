package ui

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"net"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/record"
	"github.com/brunoga/sofabaton/sifli"
)

// The hub tab checks the hub's firmware with Sofabaton's server and
// installs a newer one; the app then goes back to the hubs while the hub
// restarts.
func TestFirmware(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	u.sofabaton = func(key string) sofabatonCatalog { return fakeCatalog{key} }
	prev := remoteUpdates
	t.Cleanup(func() { remoteUpdates = prev })
	remoteUpdates = func(context.Context, *hub.Client, sofabatonCatalog) ([]remoteUpdate, error) { return nil, nil }
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	u.browse.tabs.SelectIndex(hubTab)
	if got := texts(w); !strings.Contains(got, "with the key the settings take") || !strings.Contains(got, "[Settings…]") {
		t.Fatalf("checked without a key:\n%s", got)
	}
	// With the key, the hub tab checks on its own, once a connection, and
	// says when there is something newer.
	prefs().SetString(sofabatonKeyPref, fakeKey)
	t.Cleanup(func() { prefs().SetString(sofabatonKeyPref, "") })
	u.showBrowse(u.hub).tabs.SelectIndex(hubTab)
	if got := texts(w); !strings.Contains(got, "Firmware 10 is available: Fixes bugs") || !strings.Contains(got, "A firmware update is available") {
		t.Fatalf("no update offered:\n%s", got)
	}
	test.Tap(button(t, w, "Install…"))
	test.Tap(button(t, w, "Yes"))
	if u.client != nil || !strings.Contains(texts(w), "has the new firmware and is restarting") {
		t.Fatalf("not installed:\n%s", texts(w))
	}
}

// fakeRemote answers an update as our remote did.
type fakeRemote struct {
	buf    []byte
	out    [][]byte
	slices int
	want   int
	got    []byte
}

func (r *fakeRemote) MTU() int     { return 247 }
func (r *fakeRemote) Close() error { return nil }

func (r *fakeRemote) Notification(ctx context.Context) ([]byte, error) {
	if len(r.out) == 0 {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	n := r.out[0]
	r.out = r.out[1:]
	return n, nil
}

func (r *fakeRemote) reply(id int, hexPayload string) {
	p, _ := hex.DecodeString(hexPayload)
	msg := binary.LittleEndian.AppendUint16(nil, uint16(id))
	msg = binary.LittleEndian.AppendUint16(msg, uint16(len(p)))
	msg = append(msg, p...)
	r.out = append(r.out, append([]byte{1, 0, byte(len(msg)), byte(len(msg) >> 8)}, msg...))
}

func (r *fakeRemote) Write(f []byte) error {
	if f[1] == 0 || f[1] == 1 {
		r.buf = append([]byte(nil), f[4:]...)
	} else {
		r.buf = append(r.buf, f[2:]...)
	}
	if f[1] == 1 || f[1] == 2 {
		return nil
	}
	id, p := binary.LittleEndian.Uint16(r.buf), r.buf[4:]
	switch id {
	case 40:
		r.reply(41, "000004000100")
	case 43:
		r.reply(44, "0000010001140000000000000000001024020000")
	case 45:
		r.reply(46, "0000000000000000")
	case 47:
		if p[0] == 3 {
			r.slices, r.want = 0, int(binary.LittleEndian.Uint32(p[6:]))
		}
		r.reply(48, "0000")
	case 49:
		r.slices++
		r.got = append(r.got, p[9:]...)
		if r.slices%20 == 0 || r.slices == r.want {
			r.reply(50, "0000000000000000")
		}
	case 51:
		r.reply(52, "0000")
	case 53:
		r.reply(54, "0000")
	}
	return nil
}

// The firmware card checks the hub's remotes too, and installs a newer
// firmware over Bluetooth where the app can; elsewhere it says how.
func TestRemoteFirmware(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 10, Addr: lounge})
	u.sofabaton = func(key string) sofabatonCatalog { return fakeCatalog{key} }
	prefs().SetString(sofabatonKeyPref, fakeKey)
	t.Cleanup(func() { prefs().SetString(sofabatonKeyPref, "") })
	addr, _ := net.ParseMAC("fa:ba:25:00:0f:4e")
	prevUpdates, prevDial, prevBT, prevPause := remoteUpdates, remoteDial, remoteBluetooth, sifli.RestartPause
	t.Cleanup(func() { remoteUpdates, remoteDial, remoteBluetooth, sifli.RestartPause = prevUpdates, prevDial, prevBT, prevPause })
	firmware := byte(9)
	remoteUpdates = func(ctx context.Context, _ *hub.Client, cat sofabatonCatalog) ([]remoteUpdate, error) {
		all, _ := cat.RemoteFirmwares(ctx)
		u := remoteUpdate{remote: &record.RemoteInfo{Name: "Remote 1", Firmware: firmware, Addr: addr}}
		for _, f := range all {
			if f.Version == int(firmware) {
				u.current = f
			} else if f.Version > int(firmware) {
				u.update = f
			}
		}
		return []remoteUpdate{u}, nil
	}
	fake := &fakeRemote{}
	remoteDial = func(a net.HardwareAddr) sifli.Dial {
		if a.String() != addr.String() {
			t.Errorf("dialled %v", a)
		}
		return func(context.Context) (sifli.Conn, error) { return fake, nil }
	}
	sifli.RestartPause = 0

	// Connecting checks the remotes too.
	remoteBluetooth = false
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	u.browse.tabs.SelectIndex(hubTab)
	if got := texts(w); !strings.Contains(got, "Remote 1: firmware 10 is available: Faster sync") || !strings.Contains(got, "from a phone") {
		t.Fatalf("no remote update offered:\n%s", got)
	}

	remoteBluetooth = true
	u.showBrowse(u.hub).tabs.SelectIndex(hubTab)
	test.Tap(button(t, w, "Install on Remote 1…"))
	test.Tap(button(t, w, "Yes"))
	if got := texts(w); !strings.Contains(got, "Remote 1 has firmware 10 and is installing it") || string(fake.got) != "image" {
		t.Fatalf("not installed (%q):\n%s", fake.got, got)
	}

	// On the newest, it can be sent again.
	firmware, fake.got = 10, nil
	u.checkFirmware(false)
	u.showBrowse(u.hub).tabs.SelectIndex(hubTab)
	if got := texts(w); !strings.Contains(got, "Remote 1: firmware 10 is the newest.") {
		t.Fatalf("not the newest:\n%s", got)
	}
	test.Tap(button(t, w, "Install again…"))
	test.Tap(button(t, w, "Yes"))
	if string(fake.got) != "image" {
		t.Fatalf("not sent again:\n%s", texts(w))
	}
}
