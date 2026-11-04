package ui

import (
	"net/url"
	"runtime/debug"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/irsource"
)

// The app's author and its pages.
const (
	author      = "Bruno Albuquerque"
	website     = "https://github.com/brunoga/armrest"
	issues      = "https://github.com/brunoga/armrest/issues"
	authorPage  = "https://github.com/brunoga"
	library     = "github.com/brunoga/sofabaton"
	iconsPage   = "https://github.com/m3tac0de/home-assistant-sofabaton-x1s"
	irdbPage    = "https://github.com/probonopd/irdb"
	flipperPage = "https://github.com/Lucaslhm/Flipper-IRDB"
)

// appIcon is the app's icon; tests set it, as their app has none.
var appIcon = func() fyne.Resource { return fyne.CurrentApp().Icon() }

// iconSize is the app icon's size on the About page, before Scale.
const iconSize = 96

// aboutContent is the About page: what the app is, who made it, how to
// reach them, what it is built on, and the text size. resized shows the
// page again once the text size changed.
func (a *App) aboutContent(resized func()) fyne.CanvasObject {
	top := brand()

	made := container.NewVBox(
		wrapped(author),
		link(i18n.T("Questions, problems and ideas: open an issue"), issues),
		link(i18n.T("The app's page, with its source"), website),
		link(i18n.T("%s on GitHub", author), authorPage))

	sizes := make([]string, len(textSizes))
	for i, s := range textSizes {
		sizes[i] = i18n.T(s.name)
	}
	size := widget.NewSelect(sizes, nil)
	size.SetSelectedIndex(textSize())
	size.OnChanged = func(string) {
		if size.SelectedIndex() == textSize() {
			return
		}
		fyne.CurrentApp().Preferences().SetInt(textSizePref, size.SelectedIndex())
		fyne.CurrentApp().Settings().SetTheme(Theme())
		resized()
	}

	built := container.NewVBox(
		wrapped(i18n.T("The Sofabaton library %s (%s), and Fyne %s.", depVersion(library), library, depVersion("fyne.io/fyne/v2"))),
		wrapped(i18n.T("The X2's icon drawings come from the home-assistant-sofabaton-x1s project, under the MIT license; the logos among them are their owners'.")),
		link("home-assistant-sofabaton-x1s", iconsPage))

	codes := container.NewVBox(
		wrapped(i18n.T("Adding a device can search two online collections of IR codes, and fetches a device's file from them when you pick it. Thank you to both, and to their contributors.")),
		wrapped(irsource.Credit(irsource.IRDB)),
		link("irdb", irdbPage),
		wrapped(irsource.Credit(irsource.Flipper)),
		link("Flipper-IRDB", flipperPage))

	return scrolled(container.NewVBox(
		top,
		card(i18n.T("Text size"), container.NewVBox(wrapped(i18n.T("How large the app's text is.")), size)),
		card(i18n.T("Author"), made),
		card(i18n.T("Built with"), built),
		card(i18n.T("IR codes"), codes),
		card(i18n.T("License"), wrapped(i18n.T("Armrest is free software, under the Apache License 2.0. It is not affiliated with or endorsed by Sofabaton."))),
	))
}

// brand is the app's icon, name, version and what it does.
func brand() fyne.CanvasObject {
	icon := canvas.NewImageFromResource(appIcon())
	if appIcon() == nil {
		icon.Hide()
	}
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSquareSize(iconSize * Scale()))
	name := widget.NewLabelWithStyle("Armrest", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	name.SizeName = theme.SizeNameHeadingText
	ver := widget.NewLabelWithStyle(version(), fyne.TextAlignCenter, fyne.TextStyle{})
	what := wrapped(i18n.T("Set up Sofabaton X1, X1S and X2 hubs on your own network, without the cloud."))
	what.Alignment = fyne.TextAlignCenter
	return container.NewVBox(container.NewCenter(icon), name, ver, what)
}

// showAbout shows the About page on its own, from the hubs screen.
func (a *App) showAbout() {
	var show func()
	show = func() { a.screen(i18n.T("About"), a.showHubs, a.aboutContent(show)) }
	show()
}

// link is a hyperlink to u, which is well formed.
func link(text, u string) *widget.Hyperlink {
	p, _ := url.Parse(u)
	l := widget.NewHyperlink(text, p)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// buildInfo is what the binary was built from; tests fix it.
var buildInfo = debug.ReadBuildInfo

// version is the app's version: the package's, or for a build of the
// source, its commit.
func version() string {
	if m := fyne.CurrentApp().Metadata(); m.Version != "" {
		return i18n.T("Version %s (build %d)", m.Version, m.Build)
	}
	if bi, ok := buildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && len(s.Value) >= 7 {
				return i18n.T("Built from the source, at %s", s.Value[:7])
			}
		}
	}
	return i18n.T("Built from the source")
}

// depVersion is the version of the module path the app is built with: a
// release, or the commit of one between releases.
func depVersion(path string) string {
	if bi, ok := buildInfo(); ok {
		for _, d := range bi.Deps {
			if d.Path != path {
				continue
			}
			// A pseudo-version ends in the commit's first 12 digits.
			if parts := strings.Split(d.Version, "-"); len(parts) >= 3 && len(parts[len(parts)-1]) == 12 {
				return i18n.T("at %s", parts[len(parts)-1][:7])
			}
			return d.Version
		}
	}
	return i18n.T("(version unknown)")
}
