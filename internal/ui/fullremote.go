package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/profile"
)

// showFullRemote shows the remote with keys alone, as large as the screen
// holds, on the remote's own graphite: the phone as the remote. The number
// keys take its place while "123" is on. Back, or the cross, goes back to
// where it was opened; the screen stays on meanwhile.
func (a *App) showFullRemote(model string, keys profile.Buttons, ctl *controls) {
	prev, prevBack := a.shown, a.goBack
	set := func(k string) bool { _, ok := keys[k]; return ok }
	key := func(k, label string) fyne.CanvasObject { return newRemoteKey(k, label, keys[k], ctl) }
	main := newRemoteDrawing(model, set, key)
	main.fit, main.numbers = true, nil
	numbers := newRemoteDrawing(model, set, key)
	numbers.fit, numbers.keys = true, nil
	numbers.Hide()

	mobile := fyne.CurrentDevice().IsMobile()
	exit := func() {
		fyne.CurrentApp().Driver().SetDisableScreenBlanking(a.working)
		if mobile {
			a.win.SetFullScreen(false)
		}
		a.show(prev, prevBack)
	}
	var toggle *widget.Button
	toggle = widget.NewButton("123", func() {
		if numbers.Visible() {
			numbers.Hide()
			main.Show()
			toggle.Importance = widget.LowImportance
		} else {
			main.Hide()
			numbers.Show()
			toggle.Importance = widget.HighImportance
		}
		toggle.Refresh()
	})
	toggle.Importance = widget.LowImportance
	if len(numbers.numbers) == 0 {
		toggle.Hide()
	}
	closeBtn := newIconButton(i18n.T("Close the full-screen remote"), theme.CancelIcon(), exit)
	closeBtn.Importance = widget.LowImportance
	corners := container.NewBorder(container.NewHBox(toggle, layout.NewSpacer(), closeBtn), nil, nil, nil)

	bg := canvas.NewRectangle(remoteBody)
	p := theme.Padding()
	body := container.New(layout.NewCustomPaddedLayout(2*p, 2*p, p, p), container.NewStack(main, numbers))
	a.show(container.NewStack(bg, body, container.NewThemeOverride(container.NewPadded(corners), onBar{})), func() bool {
		if numbers.Visible() {
			toggle.OnTapped()
			return true
		}
		exit()
		return true
	})
	fyne.CurrentApp().Driver().SetDisableScreenBlanking(true)
	if mobile {
		a.win.SetFullScreen(true)
	}
}
