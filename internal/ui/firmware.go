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
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/irsource"
)

// firmwareTimeout bounds asking Sofabaton's server and downloading an
// image (a hub's is about 50 KB).
const firmwareTimeout = time.Minute

// firmwareCheck is the last check of the connected hub's firmware, and
// of its remotes', with Sofabaton's server.
type firmwareCheck struct {
	// client is the connection it was made on: a new one checks again.
	client  *hub.Client
	running bool
	// firmware is the hub's, hub the newer one the server has, if any.
	firmware byte
	hub      *irsource.Firmware
	remotes  []remoteUpdate
	err      error
}

// checkFirmware asks Sofabaton's server for newer firmware for the hub
// and its remotes, in the background; auto says the app checks on its
// own, on connecting, and says so only if there is something newer.
func (a *App) checkFirmware(auto bool) {
	key := sofabatonKey()
	if key == "" || a.client == nil {
		return
	}
	c := a.client
	fc := &firmwareCheck{client: c, running: true, firmware: c.Info().Firmware}
	a.fw = fc
	if a.browse != nil {
		a.browse.showFirmware()
	}
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), firmwareTimeout)
		defer cancel()
		cat := a.sofabaton(key)
		f, err := cat.HubFirmware(ctx, c.Info())
		var found []remoteUpdate
		if err == nil {
			// The hub's answer stands without its remotes'.
			var rerr error
			if found, rerr = remoteUpdates(ctx, c, cat); rerr != nil {
				fyne.LogError("checking the remotes' firmware", rerr)
			}
		}
		fyne.Do(func() {
			fc.running, fc.hub, fc.remotes, fc.err = false, f, found, err
			if a.fw != fc || a.browse == nil {
				return // checked again, or left, meanwhile
			}
			a.browse.showFirmware()
			if auto && fc.newer() {
				a.browse.setStatus(false, i18n.T("A firmware update is available: the Hub tab has it."))
			}
		})
	})
}

// newer reports whether the check found newer firmware.
func (fc *firmwareCheck) newer() bool {
	if fc.hub != nil {
		return true
	}
	for _, r := range fc.remotes {
		if r.update != nil {
			return true
		}
	}
	return false
}

// firmwareCard shows the firmware check, and installs what it found, as
// the app does; nil for a file or the demo. Connecting checks on its own
// once a connection, with the catalog's key set.
func (s *browseScreen) firmwareCard() fyne.CanvasObject {
	a := s.app
	if a.offline() || s.hub.Demo || a.client == nil {
		return nil
	}
	s.fwStatus = wrapped("")
	s.fwActions = container.NewHBox()
	s.fwRemotes = container.NewVBox()
	s.showFirmware()
	if sofabatonKey() != "" && (a.fw == nil || a.fw.client != a.client) {
		a.checkFirmware(true)
	}
	return card(i18n.T("Firmware"), container.NewVBox(s.fwStatus, s.fwActions, s.fwRemotes))
}

// showFirmware shows the last check on the firmware card.
func (s *browseScreen) showFirmware() {
	if s.fwStatus == nil {
		return
	}
	a := s.app
	check := func() { a.checkFirmware(false) }
	show := func(text string, buttons ...fyne.CanvasObject) {
		s.fwStatus.SetText(text)
		s.fwActions.Objects = buttons
		s.fwActions.Refresh()
	}
	s.fwRemotes.RemoveAll()
	fc := a.fw
	if fc != nil && fc.client != a.client {
		fc = nil // of another connection
	}
	switch {
	case sofabatonKey() == "" && fc == nil:
		show(i18n.T("Checking for updates asks Sofabaton's server, with the key the settings take for its catalog."),
			widget.NewButtonWithIcon(i18n.T("Settings…"), theme.SettingsIcon(), a.showSettings))
	case fc == nil:
		show(i18n.T("Sofabaton's server says whether there is a newer firmware for the hub."),
			widget.NewButtonWithIcon(i18n.T("Check for an update"), theme.ViewRefreshIcon(), check))
	case fc.running:
		show(i18n.T("Checking for an update…"))
	case fc.err != nil:
		show(i18n.T("Checking failed: %v", fc.err), widget.NewButtonWithIcon(i18n.T("Try again"), theme.ViewRefreshIcon(), check))
	case fc.hub == nil:
		show(i18n.T("Firmware %d is the newest.", fc.firmware), widget.NewButtonWithIcon(i18n.T("Check again"), theme.ViewRefreshIcon(), check))
	default:
		f := fc.hub
		install := widget.NewButtonWithIcon(i18n.T("Install…"), theme.DownloadIcon(), func() { s.installFirmware(f) })
		install.Importance = widget.HighImportance
		show(i18n.T("Firmware %d is available: %s", f.Version, f.Notes), install)
	}
	if fc != nil {
		for _, r := range fc.remotes {
			s.fwRemotes.Add(s.remoteRow(r))
		}
	}
	s.fwRemotes.Hidden = len(s.fwRemotes.Objects) == 0
	s.fwRemotes.Refresh()
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
