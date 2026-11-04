package ui

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/irsource"
)

// The settings' preferences.
const (
	// autoConnectPref is set to connect to the last hub on starting;
	// lastHubPref holds that hub.
	autoConnectPref = "autoConnect"
	lastHubPref     = "lastHub"
	// sourcePref, followed by a source's name, is whether adding a device
	// searches the source.
	sourcePref = "source."
	// sofabatonKeyPref is the key of Sofabaton's catalog, which the user
	// gives: the app does not carry it.
	sofabatonKeyPref = "sofabatonKey"
)

// sources are the online sources of IR codes the settings list: the
// public ones, searched unless turned off, and Sofabaton's catalog,
// searched only once turned on with its key.
var sources = append(append([]irsource.Source(nil), irsource.Sources...), irsource.Sofabaton)

func prefs() fyne.Preferences { return fyne.CurrentApp().Preferences() }

// sourceOn reports whether adding a device searches the source.
func sourceOn(s irsource.Source) bool {
	if s == irsource.Sofabaton {
		return prefs().Bool(sourcePref+string(s)) && sofabatonKey() != ""
	}
	return prefs().BoolWithFallback(sourcePref+string(s), true)
}

func sofabatonKey() string { return prefs().String(sofabatonKeyPref) }

// sofabatonCatalog is Sofabaton's catalog (irsource.SofabatonDB); tests
// fake it.
type sofabatonCatalog interface {
	Check(context.Context) error
	Brands(context.Context) ([]irsource.SofabatonBrand, error)
	Models(context.Context, irsource.SofabatonBrand) ([]irsource.Entry, error)
	Fetch(context.Context, irsource.Entry) ([]byte, error)
}

func realSofabaton(key string) sofabatonCatalog { return irsource.SofabatonDB{Key: key} }

// lastHub is the hub connected to last, if one was.
func lastHub() (backend.Hub, bool) {
	var h struct {
		Name, Model string
		Firmware    int
		Addr        string
	}
	if err := json.Unmarshal([]byte(prefs().String(lastHubPref)), &h); err != nil {
		return backend.Hub{}, false
	}
	addr, err := netip.ParseAddr(h.Addr)
	if err != nil {
		return backend.Hub{}, false
	}
	return backend.Hub{Name: h.Name, Model: h.Model, Firmware: h.Firmware, Addr: addr}, true
}

// rememberHub keeps h as the hub connected to last; the demo hub is not
// kept.
func rememberHub(h backend.Hub) {
	if h.Demo || !h.Addr.IsValid() {
		return
	}
	b, _ := json.Marshal(map[string]any{"Name": h.Name, "Model": h.Model, "Firmware": h.Firmware, "Addr": h.Addr.String()})
	prefs().SetString(lastHubPref, string(b))
}

// settingsContent is the settings: the text size, connecting to the last
// hub on starting, and the online sources of IR codes. resized shows the
// page again once the text size changed.
func (a *App) settingsContent(resized func()) []fyne.CanvasObject {
	sizes := make([]string, len(textSizes))
	for i, s := range textSizes {
		sizes[i] = i18n.T(s.name)
	}
	size := widget.NewSelect(sizes, nil)
	size.SetSelectedIndex(textSize())
	size.OnChanged = func(string) {
		if size.SelectedIndex() == textSize() {
			return
		}
		prefs().SetInt(textSizePref, size.SelectedIndex())
		fyne.CurrentApp().Settings().SetTheme(Theme())
		resized()
	}

	auto := widget.NewCheck(i18n.T("Connect on start"), func(on bool) { prefs().SetBool(autoConnectPref, on) })
	auto.SetChecked(prefs().Bool(autoConnectPref))
	last := i18n.T("No hub connected to yet.")
	if h, ok := lastHub(); ok {
		last = i18n.T("The last hub: %s.", h.String())
	}

	list := container.NewVBox(wrapped(i18n.T("Where adding a device searches for a model's IR codes, besides the app's own templates.")))
	for _, s := range sources {
		list.Add(a.sourceSetting(s))
	}

	return []fyne.CanvasObject{
		card(i18n.T("Text size"), container.NewVBox(wrapped(i18n.T("How large the app's text is.")), size)),
		card(i18n.T("Connecting"), container.NewVBox(auto, wrapped(i18n.T("Connects to the last hub when the app starts, without looking for hubs first.")+" "+last))),
		card(i18n.T("IR code sources"), list),
	}
}

// sourceSetting turns searching a source on and off. Sofabaton's catalog
// asks for its key first, and checks it.
func (a *App) sourceSetting(s irsource.Source) fyne.CanvasObject {
	check := widget.NewCheck(string(s), nil)
	check.SetChecked(sourceOn(s))
	about := i18n.T("Public, and searched unless turned off.")
	if s == irsource.Sofabaton {
		about = i18n.T("The catalog Sofabaton's app searches, with most brands and models by number. It is not offered for other apps: it may change or stop answering, and its answers are encrypted with a key the Sofabaton app holds, which you give here (the app does not carry it).")
	}
	note := wrapped(about)
	note.TextStyle.Italic = true
	check.OnChanged = func(on bool) {
		if on == sourceOn(s) {
			return
		}
		if s != irsource.Sofabaton || !on {
			prefs().SetBool(sourcePref+string(s), on)
			a.online = online{} // searched again with the change
			return
		}
		check.SetChecked(false) // until the key is checked
		a.askSofabatonKey(func() {
			check.SetChecked(true)
		})
	}
	return container.NewVBox(check, note)
}

// askSofabatonKey asks for the catalog's key, checks it with the
// catalog, and keeps it and turns the catalog on if it works.
func (a *App) askSofabatonKey(done func()) {
	key := widget.NewPasswordEntry()
	key.SetText(sofabatonKey())
	key.SetPlaceHolder(i18n.T("16 characters"))
	problem := wrapped("")
	problem.Importance = widget.DangerImportance
	problem.Hide()
	alive := newPulse()
	alive.Hide()
	var d dialog.Dialog
	var ok *widget.Button
	try := func() {
		k := key.Text
		ok.Disable()
		alive.Start()
		problem.Hide()
		a.async(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			err := a.sofabaton(k).Check(ctx)
			fyne.Do(func() {
				alive.Stop()
				ok.Enable()
				if err != nil {
					msg := err.Error()
					if errors.Is(err, irsource.ErrSofabatonKey) {
						msg = i18n.T("That key does not open Sofabaton's catalog.")
					}
					problem.SetText(msg)
					problem.Show()
					return
				}
				prefs().SetString(sofabatonKeyPref, k)
				prefs().SetBool(sourcePref+string(irsource.Sofabaton), true)
				a.online = online{}
				d.Hide()
				done()
			})
		})
	}
	key.OnSubmitted = func(string) { try() }
	ok = widget.NewButton(i18n.T("Check and turn on"), try)
	ok.Importance = widget.HighImportance
	cancel := widget.NewButton(i18n.T("Cancel"), func() { d.Hide() })
	d = dialog.NewCustomWithoutButtons(i18n.T("Sofabaton's catalog"), container.NewVBox(
		wrapped(i18n.T("Its key is in the Sofabaton app. Armrest checks it with the catalog before using it.")),
		key, alive, problem, container.NewGridWithColumns(2, cancel, ok)), a.win)
	d.Resize(fyne.NewSize(min(ReadableWidth*Scale()*0.6, a.win.Canvas().Size().Width), d.MinSize().Height))
	d.Show()
	a.win.Canvas().Focus(key)
}

// showSettings shows the settings on their own, from the hubs screen.
func (a *App) showSettings() {
	var show func()
	show = func() { a.screen(i18n.T("Settings"), a.showHubs, scrolled(container.NewVBox(a.settingsContent(show)...))) }
	show()
}
