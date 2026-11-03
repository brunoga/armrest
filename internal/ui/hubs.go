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

// hubsScreen lists the hubs found on the network, and offers connecting by
// address and to the demo hub.
type hubsScreen struct {
	app    *App
	found  *fyne.Container
	status *widget.Label
	search *widget.Button
	busy   *pulse
	addr   *widget.Entry
	// root is laid out again when the found hubs change.
	root *container.Scroll
}

func (a *App) showHubs() {
	s := &hubsScreen{app: a}
	a.show(s.content(), nil)
	s.discover()
}

func (s *hubsScreen) content() fyne.CanvasObject {
	sub := wrapped("Set up Sofabaton X1, X1S and X2 hubs, without the cloud.")

	s.found = container.NewVBox()
	s.status = wrapped("")
	s.busy = newPulse()
	s.busy.Hide()
	s.search = widget.NewButtonWithIcon("Search again", theme.SearchIcon(), s.discover)
	network := widget.NewCard("Hubs on your network", "",
		container.NewVBox(s.found, s.busy, s.status, container.NewHBox(s.search)))

	s.addr = widget.NewEntry()
	s.addr.SetPlaceHolder("192.168.1.20")
	s.addr.OnSubmitted = func(string) { s.connectAddr() }
	byAddr := widget.NewCard("Connect by address", "", container.NewVBox(
		wrapped("For a hub that does not show up above."),
		container.NewBorder(nil, nil, nil,
			widget.NewButtonWithIcon("Connect", theme.LoginIcon(), s.connectAddr), s.addr)))

	demo := widget.NewCard("No hub at hand?", "", container.NewVBox(
		wrapped("The demo hub is a simulated X2: changes stay in the app."),
		container.NewHBox(widget.NewButtonWithIcon("Open the demo hub", theme.ComputerIcon(),
			func() { s.app.connect(backend.DemoHub) }))))

	files := widget.NewCard("Saved files", "", container.NewVBox(
		wrapped("Save a hub to a file from its Hub tab: as a backup, or with changes to apply later. Open the file here to look at it and change it without the hub, then apply or restore it. Hub dumps made with sbx open too."),
		container.NewHBox(widget.NewButtonWithIcon("Open a file…", theme.FolderOpenIcon(), s.app.openFile))))

	cards := container.NewVBox(sub)
	if f := s.app.file; f != nil {
		// Applying a file: its hub is to be picked.
		back := widget.NewButtonWithIcon("Back to the file", theme.NavigateBackIcon(), s.app.showFile)
		back.Importance = widget.HighImportance
		cards.Add(card("Applying "+f.name, container.NewVBox(
			wrapped(fmt.Sprintf("Connect to %s to apply the file's changes to it. It is read first, and you review the changes before anything is written.", f.work.Dump.Info.Name)),
			container.NewVBox(back, widget.NewButtonWithIcon("Close the file", theme.CancelIcon(), s.app.closeFile)))))
	}
	cards.Add(network)
	cards.Add(byAddr)
	cards.Add(files)
	cards.Add(demo)
	s.root = container.NewVScroll(column(container.NewPadded(cards)))
	about := widget.NewButtonWithIcon("", theme.InfoIcon(), s.app.showAbout)
	return container.NewBorder(bar(nil, "Armrest", about), nil, nil, nil, s.root)
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
		s.found.Add(b)
	}
	switch {
	case err != nil:
		s.status.SetText("Searching failed: " + err.Error())
	case len(hubs) == 0:
		s.status.SetText("No hub answered. A hub that is connected to another app (Sofabaton's, or sbx) does not answer: close it, or connect by address.")
	default:
		s.status.SetText(fmt.Sprintf("%d found. A hub connected to another app does not show up.", len(hubs)))
	}
	s.root.Refresh()
}

func (s *hubsScreen) connectAddr() {
	text := strings.TrimSpace(s.addr.Text)
	addr, err := netip.ParseAddr(text)
	if err != nil || !addr.Is4() {
		dialog.ShowError(fmt.Errorf("%q is not an IPv4 address, such as 192.168.1.20", text), s.app.win)
		return
	}
	s.app.connect(backend.Hub{Name: addr.String(), Addr: addr})
}
