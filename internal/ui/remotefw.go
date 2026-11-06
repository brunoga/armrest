package ui

import (
	"context"
	"errors"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/androidble"
	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/irsource"
	"github.com/brunoga/sofabaton/record"
	"github.com/brunoga/sofabaton/sifli"
)

// remoteUpdate is a remote of the hub and the newer firmware the server
// has for it, if any; and the server's copy of the firmware it has, to
// install again.
type remoteUpdate struct {
	remote  *record.RemoteInfo
	update  *irsource.Firmware
	current *irsource.Firmware
}

// remoteUpdates are the hub's remotes, each with its newer firmware;
// tests replace it.
var remoteUpdates = func(ctx context.Context, c *hub.Client, cat sofabatonCatalog) ([]remoteUpdate, error) {
	// A hub whose remotes do not answer is not held up for long.
	rctx, cancel := context.WithTimeout(ctx, remotesTimeout)
	rs, err := c.Remotes(rctx)
	cancel()
	if err != nil {
		return nil, err
	}
	all, err := cat.RemoteFirmwares(ctx)
	if err != nil {
		return nil, err
	}
	var out []remoteUpdate
	for _, r := range rs {
		u := remoteUpdate{remote: r}
		for _, f := range all {
			switch {
			case f.Version == int(r.Firmware):
				u.current = f
			case f.Version > int(r.Firmware) && (u.update == nil || f.Version > u.update.Version):
				u.update = f
			}
		}
		out = append(out, u)
	}
	return out, nil
}

// remotesTimeout bounds reading the hub's remotes.
const remotesTimeout = 5 * time.Second

// remoteDial connects to a remote over Bluetooth, where remoteBluetooth
// says this build can; tests replace both.
var (
	remoteDial      = androidble.Dial
	remoteBluetooth = androidble.Supported
)

// remoteRow says whether the remote has a newer firmware, and installs
// it where this device can: over Android's Bluetooth, as the app does.
func (s *browseScreen) remoteRow(r remoteUpdate) fyne.CanvasObject {
	if r.update == nil {
		text := wrapped(i18n.T("%s: firmware %d is the newest.", r.remote.Name, r.remote.Firmware))
		if !remoteBluetooth || r.current == nil {
			return text
		}
		// Sent again: to try the update, or to get a remote out of its
		// updater if an update stopped.
		again := widget.NewButtonWithIcon(i18n.T("Install again…"), theme.DownloadIcon(), func() {
			s.installRemote(remoteUpdate{remote: r.remote, update: r.current})
		})
		return container.NewVBox(text, container.NewHBox(again))
	}
	text := wrapped(i18n.T("%s: firmware %d is available: %s", r.remote.Name, r.update.Version, r.update.Notes))
	if !remoteBluetooth {
		note := wrapped(i18n.T("The remote is updated over Bluetooth from a phone: with Armrest on Android, or the Sofabaton app."))
		note.TextStyle.Italic = true
		return container.NewVBox(text, note)
	}
	install := widget.NewButtonWithIcon(i18n.T("Install…"), theme.DownloadIcon(), func() { s.installRemote(r) })
	install.Importance = widget.HighImportance
	return container.NewVBox(text, container.NewHBox(install))
}

// installRemote downloads the remote's firmware and sends it over
// Bluetooth, the phone near the remote.
func (s *browseScreen) installRemote(r remoteUpdate) {
	a := s.app
	dialog.ShowConfirm(i18n.T("Install firmware %d on %s?", r.update.Version, r.remote.Name),
		i18n.T("This device sends it to the remote over Bluetooth: about 3 MB, in 3 to 5 minutes. Keep the remote next to it and awake (press a key now), and keep Armrest open until it is done. The remote restarts on its own after."),
		func(ok bool) {
			if ok {
				s.sendRemote(r)
			}
		}, a.win)
}

func (s *browseScreen) sendRemote(r remoteUpdate) {
	a := s.app
	bar := widget.NewProgressBar()
	what := widget.NewLabel(i18n.T("Downloading the firmware…"))
	wait := dialog.NewCustomWithoutButtons(i18n.T("Updating %s", r.remote.Name), container.NewVBox(what, bar), a.win)
	wait.Show()
	key := sofabatonKey()
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
		defer cancel()
		p, err := a.sofabaton(key).RemotePackage(ctx, r.update)
		if err == nil {
			err = sifli.Update(ctx, remoteDial(r.remote.Addr), p, func(pr sifli.Progress) {
				fyne.Do(func() {
					what.SetText(remoteStep(pr.Step))
					if pr.Total > 0 {
						bar.SetValue(float64(pr.Done) / float64(pr.Total))
					}
				})
			})
		}
		fyne.Do(func() {
			wait.Hide()
			switch {
			case errors.Is(err, androidble.ErrPermission):
				dialog.ShowInformation(i18n.T("Bluetooth"), i18n.T("Armrest needs the Nearby devices permission to reach the remote: allow it when Android asks, or in the phone's settings for Armrest, then install again."), a.win)
			case err != nil:
				dialog.ShowError(err, a.win)
			default:
				dialog.ShowInformation(i18n.T("Firmware sent"),
					i18n.T("%s has firmware %d and is installing it: it restarts on its own in about a minute.", r.remote.Name, r.update.Version), a.win)
			}
		})
	})
}

// remoteTimeout bounds a remote's update: ours took a little over three
// minutes, from a phone.
const remoteTimeout = 15 * time.Minute

// remoteStep is an update's step, in the user's language.
func remoteStep(step string) string {
	switch step {
	case "asking the device to restart into its updater":
		return i18n.T("Restarting the remote into its updater…")
	case "waiting for the device to restart":
		return i18n.T("Waiting for the remote to restart…")
	case "sending the control packet":
		return i18n.T("Starting the update…")
	case "the device is installing the update":
		return i18n.T("The remote is installing the update…")
	}
	return i18n.T("Sending the firmware…")
}
