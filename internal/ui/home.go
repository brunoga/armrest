package ui

import (
	"context"
	"errors"
	"fmt"
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/workspace"
	"github.com/brunoga/sofabaton/profile"
	"github.com/brunoga/sofabaton/session"
)

// showHome reads the connected hub, showing how far it got, then shows
// its activities and devices.
func (a *App) showHome(h backend.Hub) {
	info := a.client.Info()
	title := widget.NewLabelWithStyle("Reading "+info.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameHeadingText
	bar := widget.NewProgressBar()
	step := wrapped("Asking the hub what it has…")
	ctx, cancel := context.WithCancel(context.Background())
	cancelBtn := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), func() {
		cancel()
		a.disconnect()
	})
	retry := widget.NewButtonWithIcon("Try again", theme.ViewRefreshIcon(), func() {
		a.Close()
		a.connect(h)
	})
	retry.Importance = widget.HighImportance
	retry.Hide()
	intro := wrapped("This takes a few seconds for each device and activity. Keep the app open until it is done.")
	a.win.SetContent(column(container.NewPadded(container.NewVBox(
		title, intro, bar, step, container.NewHBox(retry, cancelBtn)))))

	c := a.client
	a.async(func() {
		defer cancel()
		w, err := workspace.Read(ctx, c, func(done, total int, name string) {
			fyne.Do(func() {
				bar.SetValue(float64(done) / float64(max(total, 1)))
				switch name {
				case "":
				case workspace.Again:
					step.SetText("The hub gave an answer that does not add up: reading it again…")
				default:
					step.SetText(fmt.Sprintf("Reading %s (%d of %d)", name, done+1, total))
				}
			})
		})
		fyne.Do(func() {
			switch {
			case errors.Is(err, context.Canceled):
				// Cancel went back to the hubs already.
			case err != nil:
				title.SetText("Could not read " + info.Name)
				intro.Hide()
				bar.Hide()
				step.SetText(readFailure(err))
				cancelBtn.SetText("Back")
				cancelBtn.SetIcon(theme.NavigateBackIcon())
				retry.Show()
			default:
				a.work = w
				a.showBrowse(h)
			}
		})
	})
}

// readFailure is why reading the hub failed, in words.
func readFailure(err error) string {
	if errors.Is(err, session.ErrClosed) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) {
		return "The connection to the hub was lost. This happens when the device sleeps or leaves the Wi-Fi network while reading. Try again, and keep the app open until it is done."
	}
	return "Reading the hub failed: " + err.Error()
}

// browseScreen lists the activities and devices, and shows the one
// picked.
type browseScreen struct {
	app   *App
	rows  []browseRow
	list  *widget.List
	split *listDetail
}

// browseRow is a row of the list: a heading, an activity or a device.
type browseRow struct {
	heading  string
	activity *profile.Activity
	device   *profile.Device
}

func (r browseRow) text() string {
	switch {
	case r.activity != nil:
		return r.activity.Name
	case r.device != nil:
		return r.device.Name
	}
	return r.heading
}

func (a *App) showBrowse(h backend.Hub) {
	s := &browseScreen{app: a}
	a.win.SetContent(s.content(h))
}

func (s *browseScreen) content(h backend.Hub) fyne.CanvasObject {
	cfg := s.app.work.Config
	s.rows = append(s.rows, browseRow{heading: fmt.Sprintf("Activities (%d)", len(cfg.Activities))})
	for _, a := range cfg.Activities {
		s.rows = append(s.rows, browseRow{activity: a})
	}
	s.rows = append(s.rows, browseRow{heading: fmt.Sprintf("Devices (%d)", len(cfg.Devices))})
	for _, d := range cfg.Devices {
		s.rows = append(s.rows, browseRow{device: d})
	}

	s.list = widget.NewList(
		func() int { return len(s.rows) },
		func() fyne.CanvasObject {
			icon := widget.NewIcon(theme.MediaPlayIcon())
			l := widget.NewLabel("")
			l.Truncation = fyne.TextTruncateEllipsis
			return container.NewBorder(nil, nil, icon, nil, l)
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			r := s.rows[i]
			row := o.(*fyne.Container)
			l, icon := row.Objects[0].(*widget.Label), row.Objects[1].(*widget.Icon)
			l.SetText(r.text())
			l.TextStyle.Bold = r.heading != ""
			switch {
			case r.activity != nil:
				icon.SetResource(theme.MediaPlayIcon())
				icon.Show()
			case r.device != nil:
				icon.SetResource(theme.ComputerIcon())
				icon.Show()
			default:
				icon.Hide()
			}
			l.Refresh()
		})
	s.list.OnSelected = func(i widget.ListItemID) {
		r := s.rows[i]
		switch {
		case r.activity != nil:
			s.split.show(r.activity.Name, activityView(r.activity))
		case r.device != nil:
			s.split.show(r.device.Name, deviceView(r.device))
		default:
			s.list.Unselect(i)
		}
	}

	s.split = newListDetail(s.list)
	s.split.onBack = s.list.UnselectAll
	hint := wrapped("Pick an activity or a device to see what it does.")
	hint.Importance = widget.LowImportance
	s.split.body.Objects = []fyne.CanvasObject{container.NewPadded(container.NewVBox(hint))}

	return container.NewBorder(s.header(h), nil, nil, nil, s.split)
}

func (s *browseScreen) header(h backend.Hub) fyne.CanvasObject {
	info := s.app.client.Info()
	name := widget.NewLabelWithStyle(info.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	detail := fmt.Sprintf("%v, firmware %d", info.Variant, info.Firmware)
	if h.Demo {
		detail = "Demo hub: a simulated " + detail
	} else {
		detail += ", at " + h.Addr.String()
	}
	sub := widget.NewLabel(detail)
	sub.Truncation = fyne.TextTruncateEllipsis
	return container.NewVBox(
		container.NewBorder(nil, nil, nil,
			container.NewVBox(widget.NewButtonWithIcon("Disconnect", theme.LogoutIcon(), s.app.disconnect)),
			container.NewVBox(name, sub)),
		widget.NewSeparator())
}
