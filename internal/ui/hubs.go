package ui

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
)

// hubsScreen is the first screen: the app, and the hubs found on the
// network to connect to. Connecting by address, for a hub not found, is
// beside them; opening a file and the demo hub are in the bar's menu.
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
	s.search = widget.NewButtonWithIcon("Search again", theme.SearchIcon(), s.discover)
	notListed := widget.NewButton("Hub not listed?", s.askAddress)
	notListed.Importance = widget.LowImportance
	network := widget.NewCard("Hubs on your network", "",
		container.NewVBox(s.found, s.busy, s.status, container.NewHBox(s.search), container.NewHBox(notListed)))

	cards := container.NewVBox(brand())
	if f := s.app.file; f != nil {
		// Applying a file: its hub is to be picked.
		back := widget.NewButtonWithIcon("Back to the file", theme.NavigateBackIcon(), s.app.showFile)
		back.Importance = widget.HighImportance
		cards.Add(card("Applying "+f.name, container.NewVBox(
			wrapped(fmt.Sprintf("Connect to %s to apply the file's changes to it. It is read first, and you review the changes before anything is written.", f.work.Dump.Info.Name)),
			container.NewVBox(back, widget.NewButtonWithIcon("Close the file", theme.CancelIcon(), s.app.closeFile)))))
	}
	cards.Add(network)
	s.root = scrolled(cards)

	s.app.hubsMenu = fyne.NewMenu("",
		fyne.NewMenuItem("Open a saved file…", s.app.openFile),
		fyne.NewMenuItem("Try the demo hub", func() { s.app.connect(backend.DemoHub) }),
		fyne.NewMenuItem("About Armrest", s.app.showAbout))
	more := widget.NewButtonWithIcon("", theme.MoreVerticalIcon(), nil)
	more.OnTapped = func() {
		widget.ShowPopUpMenuAtRelativePosition(s.app.hubsMenu, s.app.win.Canvas(), fyne.NewPos(0, more.Size().Height), more)
	}
	// The app's name is under its icon.
	return container.NewBorder(bar(nil, "", more), nil, nil, nil, s.root)
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
	ok := widget.NewButtonWithIcon("Connect", theme.LoginIcon(), connect)
	ok.Importance = widget.HighImportance
	cancel := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() { d.Hide() })
	d = dialog.NewCustomWithoutButtons("Connect by address", container.NewVBox(
		wrapped("A hub on another part of the network, or behind a router that keeps it from being found: its address is in the router's list of devices, or the Sofabaton app's hub settings."),
		addr, container.NewGridWithColumns(2, cancel, ok)), s.app.win)
	d.Resize(fyne.NewSize(min(ReadableWidth*Scale()*0.6, s.app.win.Canvas().Size().Width), d.MinSize().Height))
	d.Show()
	s.app.win.Canvas().Focus(addr)
}

// discover lists the hubs that answer.
func (s *hubsScreen) discover() {
	s.search.Disable()
	s.busy.Start()
	s.status.SetText("Searching…")
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
	for _, h := range hubs {
		b := widget.NewButtonWithIcon(h.String(), theme.MediaPlayIcon(), func() { s.app.connect(h) })
		b.Alignment = widget.ButtonAlignLeading
		b.Importance = widget.HighImportance
		s.found.Add(b)
	}
	switch {
	case err != nil:
		s.status.SetText("Searching failed: " + err.Error())
	case len(hubs) == 0:
		s.status.SetText("No hub answered. A hub connected to another app (Sofabaton's, or sbx) does not answer: close it. Or, if it is on another part of the network, connect to it by address.")
	default:
		s.status.SetText(fmt.Sprintf("%d found. A hub connected to another app does not show up.", len(hubs)))
	}
	s.root.Refresh()
}

// parseAddr is the hub at the address text, or says why text is not
// one.
func (s *hubsScreen) parseAddr(text string) (backend.Hub, bool) {
	text = strings.TrimSpace(text)
	addr, err := netip.ParseAddr(text)
	if err != nil || !addr.Is4() {
		dialog.ShowError(fmt.Errorf("%q is not an IPv4 address, such as 192.168.1.20", text), s.app.win)
		return backend.Hub{}, false
	}
	return backend.Hub{Name: addr.String(), Addr: addr}, true
}
