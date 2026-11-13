package ui

import (
	"context"
	"errors"
	"net/netip"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/i18n"
)

// hubsScreen is the first screen: the app, and the hubs found on the
// network to connect to, and the other ways in under them: connecting by
// address, for a hub not found, opening a file and the demo hub. The
// bar's menu has the settings and About.
type hubsScreen struct {
	app    *App
	found  *fyne.Container
	status *widget.Label
	search *widget.Button
	busy   *pulse
	// root is laid out again when the found hubs change.
	root *container.Scroll
}

func (a *App) showHubs() {
	s := &hubsScreen{app: a}
	a.show(s.content(), nil)
	s.discover()
}

func (s *hubsScreen) content() fyne.CanvasObject {
	s.found = container.NewVBox()
	s.status = wrapped("")
	s.busy = newPulse()
	s.busy.Hide()
	s.search = widget.NewButtonWithIcon(i18n.T("Search again"), theme.ViewRefreshIcon(), s.discover)
	s.search.Importance = widget.LowImportance
	network := container.NewVBox(
		container.NewBorder(nil, nil, nil, s.search, groupTitle(i18n.T("Hubs on your network"))),
		widget.NewCard("", "", container.NewVBox(s.found, s.busy, quiet(s.status))))

	// The other ways in, in sight: a first-time user finds them here, not
	// in a menu.
	others := group(i18n.T("Other ways in"),
		newNavRow(theme.NewPrimaryThemedResource(theme.SearchIcon()), i18n.T("Hub not listed?"), i18n.T("Connect to it by its address."), s.askAddress),
		newNavRow(theme.NewPrimaryThemedResource(theme.FolderOpenIcon()), i18n.T("Open a saved file…"), i18n.T("A hub saved to a file, to change without the hub."), s.app.openFile),
		newNavRow(theme.NewPrimaryThemedResource(theme.MediaPlayIcon()), i18n.T("Try the demo hub"), i18n.T("A simulated hub, to look around."), func() { s.app.connect(backend.DemoHub) }))

	cards := container.NewVBox(brand(false))
	if f := s.app.file; f != nil {
		// Applying a file: its hub is to be picked.
		back := widget.NewButtonWithIcon(i18n.T("Back to the file"), theme.NavigateBackIcon(), s.app.showFile)
		back.Importance = widget.HighImportance
		cards.Add(card(i18n.T("Applying %s", f.name), container.NewVBox(
			wrapped(i18n.T("Connect to %s to apply the file's changes to it. It is read first, and you review the changes before anything is written.", f.work.Dump.Info.Name)),
			container.NewVBox(back, widget.NewButtonWithIcon(i18n.T("Close the file"), theme.CancelIcon(), s.app.closeFile)))))
	}
	cards.Add(network)
	cards.Add(others)
	s.root = scrolled(cards)

	s.app.hubsMenu = fyne.NewMenu("",
		fyne.NewMenuItem(i18n.T("Settings"), s.app.showSettings),
		fyne.NewMenuItem(i18n.T("About Armrest"), s.app.showAbout))
	more := newIconButton(i18n.T("More"), theme.MoreVerticalIcon(), nil)
	more.OnTapped = func() {
		widget.ShowPopUpMenuAtRelativePosition(s.app.hubsMenu, s.app.win.Canvas(), fyne.NewPos(0, more.Size().Height), more)
	}
	// The app's name is under its icon.
	top, _ := bar(nil, i18n.T("Hubs"), "", container.NewCenter(more))
	return container.NewBorder(top, nil, nil, nil, s.root)
}

// askAddress asks for the address of a hub that is not found, and
// connects to it.
func (s *hubsScreen) askAddress() {
	addr := widget.NewEntry()
	addr.SetPlaceHolder("192.168.1.20")
	var d dialog.Dialog
	connect := func() {
		if h, ok := s.parseAddr(addr.Text); ok {
			d.Hide()
			s.app.connect(h)
		}
	}
	addr.OnSubmitted = func(string) { connect() }
	ok := widget.NewButtonWithIcon(i18n.T("Connect"), theme.LoginIcon(), connect)
	ok.Importance = widget.HighImportance
	cancel := widget.NewButtonWithIcon(i18n.T("Cancel"), theme.CancelIcon(), func() { d.Hide() })
	d = dialog.NewCustomWithoutButtons(i18n.T("Connect by address"), container.NewVBox(
		wrapped(i18n.T("A hub on another part of the network, or behind a router that keeps it from being found: its address is in the router's list of devices, or the Sofabaton app's hub settings.")),
		addr, container.NewGridWithColumns(2, cancel, ok)), s.app.win)
	d.Resize(fyne.NewSize(min(ReadableWidth*Scale()*0.6, s.app.win.Canvas().Size().Width), d.MinSize().Height))
	d.Show()
	s.app.win.Canvas().Focus(addr)
}

// discover lists the hubs that answer.
func (s *hubsScreen) discover() {
	s.search.Disable()
	s.busy.Start()
	s.status.SetText(i18n.T("Searching…"))
	s.found.RemoveAll()
	s.root.Refresh()
	s.app.async(func() {
		hubs, err := s.app.back.Discover(context.Background())
		fyne.Do(func() { s.show(hubs, err) })
	})
}

func (s *hubsScreen) show(hubs []backend.Hub, err error) {
	s.busy.Stop()
	s.search.Enable()
	for i, h := range hubs {
		if i > 0 {
			s.found.Add(widget.NewSeparator())
		}
		sub := i18n.T("%s, firmware %d · %v", h.Model, h.Firmware, h.Addr)
		r := newNavRow(hubArt, h.Name, sub, func() { s.app.connect(h) })
		r.leadRatio = hubArtRatio
		s.found.Add(r)
	}
	switch {
	case err != nil:
		s.status.SetText(i18n.T("Searching failed: %s", err.Error()))
	case len(hubs) == 0:
		// Something to look at besides the words: the hub not found.
		s.found.Add(container.NewCenter(hubImage(headingTile * Scale())))
		s.status.SetText(i18n.T("No hub answered. A hub connected to another app (Sofabaton's, or sbx) does not answer: close it. Or, if it is on another part of the network, connect to it by address."))
	default:
		s.status.SetText(i18n.T("A hub connected to another app does not show up: close that app first."))
	}
	s.root.Refresh()
}

// parseAddr is the hub at the address text, or says why text is not
// one.
func (s *hubsScreen) parseAddr(text string) (backend.Hub, bool) {
	text = strings.TrimSpace(text)
	addr, err := netip.ParseAddr(text)
	if err != nil || !addr.Is4() {
		dialog.ShowError(errors.New(i18n.T("%q is not an IPv4 address, such as 192.168.1.20", text)), s.app.win)
		return backend.Hub{}, false
	}
	return backend.Hub{Name: addr.String(), Addr: addr}, true
}
