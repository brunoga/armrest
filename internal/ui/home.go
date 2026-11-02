package ui

import (
	"context"
	"errors"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/record"
)

// homeScreen shows the connected hub's devices and activities.
type homeScreen struct {
	app        *App
	devices    *fyne.Container
	activities *fyne.Container
	status     *widget.Label
}

func (a *App) showHome(h backend.Hub) {
	s := &homeScreen{app: a}
	a.win.SetContent(s.content(h))
	s.load()
}

func (s *homeScreen) content(h backend.Hub) fyne.CanvasObject {
	info := s.app.client.Info()
	name := widget.NewLabelWithStyle(info.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	name.SizeName = theme.SizeNameHeadingText
	detail := fmt.Sprintf("%v, firmware %d", info.Variant, info.Firmware)
	if h.Demo {
		detail = "Demo hub: a simulated " + detail
	} else {
		detail += ", at " + h.Addr.String()
	}
	header := container.NewBorder(nil, nil, nil,
		container.NewVBox(widget.NewButtonWithIcon("Disconnect", theme.LogoutIcon(), s.app.disconnect)),
		container.NewVBox(name, widget.NewLabel(detail)))

	s.devices = container.NewVBox()
	s.activities = container.NewVBox()
	s.status = widget.NewLabel("Reading the hub…")
	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Activities", theme.MediaPlayIcon(), container.NewVScroll(s.activities)),
		container.NewTabItemWithIcon("Devices", theme.ComputerIcon(), container.NewVScroll(s.devices)),
	)
	return container.NewPadded(container.NewBorder(header, s.status, nil, nil, tabs))
}

// load reads the hub's lists.
func (s *homeScreen) load() {
	c := s.app.client
	s.app.async(func() {
		ctx := context.Background()
		devs, err := c.Devices(ctx)
		var acts []*record.Entity
		if err == nil {
			acts, err = c.Activities(ctx)
			if errors.Is(err, hub.ErrNotFound) {
				acts, err = nil, nil
			}
		}
		fyne.Do(func() {
			if err != nil {
				s.status.SetText("Reading the hub failed: " + err.Error())
				return
			}
			s.status.SetText(fmt.Sprintf("%d activities, %d devices.", len(acts), len(devs)))
			fill(s.activities, acts, "No activities yet.")
			fill(s.devices, devs, "No devices yet.")
		})
	})
}

func fill(list *fyne.Container, ents []*record.Entity, empty string) {
	list.RemoveAll()
	if len(ents) == 0 {
		list.Add(widget.NewLabel(empty))
		return
	}
	for _, e := range ents {
		text := e.Name
		if e.Brand != "" {
			text += "  ·  " + e.Brand
		}
		list.Add(widget.NewLabel(text))
	}
}
