package ui

import (
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
)

// showOrder changes the order the devices, or the activities, are
// listed in on the remote and in the app, by dragging them.
func (a *App) showOrder(activities bool) {
	type item struct {
		name string
		icon fyne.Resource
	}
	var items, was []item
	title, what := i18n.T("Order of the devices"), i18n.T("The remote and the app list the devices in this order.")
	if activities {
		title, what = i18n.T("Order of the activities"), i18n.T("The remote and the app list the activities in this order.")
		for _, x := range a.work.Config.Activities {
			items = append(items, item{x.Name, tile(x.Icon, activityIcon, x.Color)})
		}
	} else {
		for _, x := range a.work.Config.Devices {
			items = append(items, item{x.Name, tile(x.Icon, deviceIcon, x.Color)})
		}
	}
	was = append(was, items...)
	list := container.NewVBox()
	for _, it := range items {
		icon := canvas.NewImageFromResource(it.icon)
		icon.FillMode = canvas.ImageFillContain
		icon.SetMinSize(fyne.NewSquareSize(rowTile * Scale()))
		grip := newDragHandle(list, func(i, j int) { items[i], items[j] = items[j], items[i] }, nil)
		row := container.NewBorder(nil, nil, container.NewHBox(grip, icon), nil, widget.NewLabel(it.name))
		grip.row = row
		list.Add(row)
	}
	back := func() { a.showBrowse(a.hub) }
	done := widget.NewButtonWithIcon(i18n.T("Done"), theme.ConfirmIcon(), func() {
		names := make([]string, len(items))
		for i, it := range items {
			names[i] = it.name
		}
		set := a.work.OrderDevices
		if activities {
			set = a.work.OrderActivities
		}
		b := a.showBrowse(a.hub)
		if slices.Equal(items, was) {
			return
		}
		if err := set(names); err != nil {
			b.setStatus(false, err.Error())
			return
		}
		b.edited(i18n.T("Changed the order."))
	})
	done.Importance = widget.HighImportance
	a.screen(title, back, scrolled(container.NewVBox(
		wrapped(what+" "+i18n.T(dragHint)),
		list,
		container.NewHBox(done))))
}
