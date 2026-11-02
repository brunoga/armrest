// Command armrest is a GUI for setting up Sofabaton hubs.
package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/ui"
)

//go:embed Icon.png
var icon []byte

func main() {
	a := app.NewWithID("io.github.brunoga.armrest")
	a.SetIcon(fyne.NewStaticResource("Icon.png", icon))
	w := a.NewWindow("Armrest")
	w.Resize(fyne.NewSize(720, 640))
	u := ui.New(w, backend.Network())
	w.SetOnClosed(u.Close)
	u.Start()
	w.ShowAndRun()
}
