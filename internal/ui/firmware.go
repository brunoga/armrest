package ui

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/irsource"
)

// firmwareTimeout bounds asking Sofabaton's server and downloading an
// image (a hub's is about 50 KB).
const firmwareTimeout = time.Minute

// firmwareCard checks the hub's firmware against Sofabaton's server and
// installs a newer one, as the app does; nil for a file or the demo.
func (s *browseScreen) firmwareCard() fyne.CanvasObject {
	a := s.app
	if a.offline() || s.hub.Demo || a.client == nil {
		return nil
	}
	status := wrapped("")
	actions := container.NewHBox()
	var check func()
	show := func(text string, buttons ...fyne.CanvasObject) {
		status.SetText(text)
		actions.Objects = buttons
		actions.Refresh()
	}
	check = func() {
		key := sofabatonKey()
		if key == "" {
			show(i18n.T("Checking for updates asks Sofabaton's server, with the key the settings take for its catalog."))
			return
		}
		info := a.client.Info()
		show(i18n.T("Checking for an update…"))
		a.async(func() {
			ctx, cancel := context.WithTimeout(context.Background(), firmwareTimeout)
			defer cancel()
			f, err := a.sofabaton(key).HubFirmware(ctx, info)
			fyne.Do(func() {
				switch {
				case err != nil:
					show(i18n.T("Checking failed: %v", err), widget.NewButtonWithIcon(i18n.T("Try again"), theme.ViewRefreshIcon(), check))
				case f == nil:
					show(i18n.T("Firmware %d is the newest.", info.Firmware))
				default:
					install := widget.NewButtonWithIcon(i18n.T("Install…"), theme.DownloadIcon(), func() { s.installFirmware(f) })
					install.Importance = widget.HighImportance
					show(i18n.T("Firmware %d is available: %s", f.Version, f.Notes), install)
				}
			})
		})
	}
	show(i18n.T("Sofabaton's server says whether there is a newer firmware for the hub."), widget.NewButtonWithIcon(i18n.T("Check for an update"), theme.ViewRefreshIcon(), check))
	return card(i18n.T("Firmware"), container.NewVBox(status, actions))
}

// installFirmware downloads the firmware and sends it to the hub, which
// then restarts: the app goes back to the hubs, to connect again.
func (s *browseScreen) installFirmware(f *irsource.Firmware) {
	a := s.app
	name := hubName(a.hub)
	dialog.ShowConfirm(i18n.T("Install firmware %d?", f.Version),
		i18n.T("%s takes about a minute to receive it, then restarts. Keep it powered and this device near it until it is done. Changes you have not applied are kept.", name),
		func(ok bool) {
			if !ok {
				return
			}
			bar := widget.NewProgressBar()
			what := widget.NewLabel(i18n.T("Downloading the firmware…"))
			wait := dialog.NewCustomWithoutButtons(i18n.T("Updating %s", name), container.NewVBox(what, bar), a.win)
			wait.Show()
			a.working = true
			c, key := a.client, sofabatonKey()
			a.async(func() {
				ctx, cancel := context.WithTimeout(context.Background(), firmwareTimeout)
				image, err := a.sofabaton(key).HubImage(ctx, f)
				cancel()
				if err == nil {
					fyne.Do(func() { what.SetText(i18n.T("Sending it to the hub…")) })
					err = c.UpdateFirmware(context.Background(), image, func(done, total int) {
						fyne.Do(func() { bar.SetValue(float64(done) / float64(total)) })
					})
				}
				fyne.Do(func() {
					wait.Hide()
					a.working = false
					if err != nil {
						dialog.ShowError(err, a.win)
						return
					}
					a.saveEdits(a.hub, a.work)
					a.Close() // the hub restarts without closing the connection
					a.showHubs()
					dialog.ShowInformation(i18n.T("Firmware sent"),
						i18n.T("%s has the new firmware and is restarting. Connect to it again in a minute or two.", name), a.win)
				})
			})
		}, a.win)
}
