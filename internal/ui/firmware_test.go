package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/brunoga/armrest/internal/backend"
)

// The hub tab checks the hub's firmware with Sofabaton's server and
// installs a newer one; the app then goes back to the hubs while the hub
// restarts.
func TestFirmware(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	u.sofabaton = func(key string) sofabatonCatalog { return fakeCatalog{key} }
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	hubAction(t, u, "Check for an update")
	if got := texts(w); !strings.Contains(got, "with the key the settings take") {
		t.Fatalf("checked without a key:\n%s", got)
	}
	prefs().SetString(sofabatonKeyPref, fakeKey)
	t.Cleanup(func() { prefs().SetString(sofabatonKeyPref, "") })
	u.browse.tabs.SelectIndex(hubTab)
	u.showBrowse(u.hub).tabs.SelectIndex(hubTab)
	test.Tap(button(t, w, "Check for an update"))
	if got := texts(w); !strings.Contains(got, "Firmware 10 is available: Fixes bugs") {
		t.Fatalf("no update offered:\n%s", got)
	}
	test.Tap(button(t, w, "Install…"))
	test.Tap(button(t, w, "Yes"))
	if u.client != nil || !strings.Contains(texts(w), "has the new firmware and is restarting") {
		t.Fatalf("not installed:\n%s", texts(w))
	}
}
