package ui

import (
	_ "embed"
	"fmt"
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/profile"
)

// The app's colors, from its icon: a deep teal, cream and orange.
var (
	teal      = color.NRGBA{0x1F, 0x6B, 0x78, 0xFF}
	tealLight = color.NRGBA{0x5F, 0xB8, 0xC4, 0xFF}
	barLight  = color.NRGBA{0x1F, 0x5F, 0x6B, 0xFF}
	barDark   = color.NRGBA{0x17, 0x3F, 0x47, 0xFF}
	cream     = color.NRGBA{0xF5, 0xF2, 0xEC, 0xFF}
	orange    = color.NRGBA{0xF4, 0xA0, 0x4B, 0xFF}
)

// Colors of the app's own, beside Fyne's.
const (
	// colorPage is behind the cards: the cards are Fyne's background.
	colorPage fyne.ThemeColorName = "armrestPage"
	// colorBar is the app bar's.
	colorBar fyne.ThemeColorName = "armrestBar"
)

// Theme is the app's look: Fyne's, in the app's colors, a size larger
// (larger still on phones) and rounder.
func Theme() fyne.Theme { return look{} }

type look struct{}

var palettes = map[fyne.ThemeVariant]map[fyne.ThemeColorName]color.Color{
	theme.VariantLight: {
		colorPage:                          cream,
		colorBar:                           barLight,
		theme.ColorNameBackground:          color.White,
		theme.ColorNameForeground:          color.NRGBA{0x1E, 0x2A, 0x2E, 0xFF},
		theme.ColorNamePrimary:             teal,
		theme.ColorNameForegroundOnPrimary: color.White,
		theme.ColorNameButton:              color.NRGBA{0xEC, 0xE7, 0xDF, 0xFF},
		theme.ColorNameInputBackground:     color.NRGBA{0xF3, 0xEF, 0xE8, 0xFF},
		theme.ColorNameInputBorder:         color.NRGBA{0xDD, 0xD6, 0xCB, 0xFF},
		theme.ColorNameMenuBackground:      color.White,
		theme.ColorNameOverlayBackground:   color.White,
		theme.ColorNameHover:               color.NRGBA{0x1F, 0x6B, 0x78, 0x14},
		theme.ColorNamePressed:             color.NRGBA{0x1F, 0x6B, 0x78, 0x29},
		theme.ColorNameSelection:           color.NRGBA{0x1F, 0x6B, 0x78, 0x24},
		theme.ColorNameFocus:               color.NRGBA{0xF4, 0xA0, 0x4B, 0x99},
		theme.ColorNameSeparator:           color.NRGBA{0xE4, 0xDE, 0xD4, 0xFF},
		theme.ColorNamePlaceHolder:         color.NRGBA{0x5F, 0x69, 0x6B, 0xFF},
		theme.ColorNameError:               color.NRGBA{0xC6, 0x28, 0x28, 0xFF},
		theme.ColorNameShadow:              color.NRGBA{0x1E, 0x2A, 0x2E, 0x22},
		theme.ColorNameHeaderBackground:    color.NRGBA{0xEC, 0xE7, 0xDF, 0xFF},
		theme.ColorNameWarning:             orange,
		theme.ColorNameForegroundOnWarning: color.NRGBA{0x1E, 0x2A, 0x2E, 0xFF},
		theme.ColorNameHyperlink:           teal,
	},
	theme.VariantDark: {
		colorPage:                          color.NRGBA{0x10, 0x17, 0x19, 0xFF},
		colorBar:                           barDark,
		theme.ColorNameBackground:          color.NRGBA{0x1A, 0x23, 0x26, 0xFF},
		theme.ColorNameForeground:          color.NRGBA{0xEA, 0xE6, 0xDF, 0xFF},
		theme.ColorNamePrimary:             tealLight,
		theme.ColorNameForegroundOnPrimary: color.NRGBA{0x0B, 0x1A, 0x1D, 0xFF},
		theme.ColorNameButton:              color.NRGBA{0x26, 0x32, 0x35, 0xFF},
		theme.ColorNameInputBackground:     color.NRGBA{0x22, 0x2D, 0x30, 0xFF},
		theme.ColorNameInputBorder:         color.NRGBA{0x34, 0x42, 0x45, 0xFF},
		theme.ColorNameMenuBackground:      color.NRGBA{0x1F, 0x29, 0x2C, 0xFF},
		theme.ColorNameOverlayBackground:   color.NRGBA{0x1F, 0x29, 0x2C, 0xFF},
		theme.ColorNameHover:               color.NRGBA{0x5F, 0xB8, 0xC4, 0x1A},
		theme.ColorNamePressed:             color.NRGBA{0x5F, 0xB8, 0xC4, 0x33},
		theme.ColorNameSelection:           color.NRGBA{0x5F, 0xB8, 0xC4, 0x2E},
		theme.ColorNameFocus:               color.NRGBA{0xF4, 0xA0, 0x4B, 0x99},
		theme.ColorNameSeparator:           color.NRGBA{0x2C, 0x38, 0x3B, 0xFF},
		theme.ColorNamePlaceHolder:         color.NRGBA{0x8A, 0x96, 0x98, 0xFF},
		theme.ColorNameError:               color.NRGBA{0xFF, 0x6B, 0x5E, 0xFF},
		theme.ColorNameShadow:              color.NRGBA{0, 0, 0, 0x66},
		theme.ColorNameHeaderBackground:    color.NRGBA{0x22, 0x2D, 0x30, 0xFF},
		theme.ColorNameWarning:             orange,
		theme.ColorNameHyperlink:           tealLight,
	},
}

func (look) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if v != theme.VariantDark {
		v = theme.VariantLight
	}
	if c, ok := palettes[v][n]; ok {
		return c
	}
	return theme.DefaultTheme().Color(n, v)
}

func (look) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }

func (look) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }

// textSizes are the sizes the About tab sets text to, by how much larger
// than normal they are.
var textSizes = []struct {
	name   string
	factor float32
}{{i18n.Mark("Smaller"), 0.85}, {i18n.Mark("Normal"), 1}, {i18n.Mark("Larger"), 1.15}, {i18n.Mark("Largest"), 1.3}}

// textSizePref is the preference holding the index of the text size.
const textSizePref = "textSize"

// textSize is the index in textSizes of the size set.
func textSize() int {
	i := fyne.CurrentApp().Preferences().IntWithFallback(textSizePref, 1)
	if i < 0 || i >= len(textSizes) {
		return 1
	}
	return i
}

// Scale is how much larger than Fyne's the app is: text of 14 becomes
// about 18, and on phones, whose screens are near, 17; times the text
// size set.
func Scale() float32 {
	base := float32(1.3)
	if fyne.CurrentDevice().IsMobile() {
		base = 1.2
	}
	return base * textSizes[textSize()].factor
}

func (look) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameButtonRadius, theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 10
	case theme.SizeNameCardRadius, theme.SizeNameDialogRadius:
		return 14
	case theme.SizeNameHeadingText:
		// Text grows, headings less: cards' titles were loud.
		return 18 * Scale()
	case theme.SizeNameSubHeadingText:
		return 15 * Scale()
	case theme.SizeNameSeparatorThickness, theme.SizeNameInputBorder:
		return theme.DefaultTheme().Size(n)
	}
	return theme.DefaultTheme().Size(n) * Scale()
}

// fill is a rectangle of a theme color, following the theme; rounded as
// buttons are, if round.
type fill struct {
	widget.BaseWidget
	name  fyne.ThemeColorName
	round bool
}

func newFill(name fyne.ThemeColorName) *fill {
	f := &fill{name: name}
	f.ExtendBaseWidget(f)
	return f
}

// newRoundFill is a fill with the corners of a button.
func newRoundFill(name fyne.ThemeColorName) *fill {
	f := &fill{name: name, round: true}
	f.ExtendBaseWidget(f)
	return f
}

func (f *fill) CreateRenderer() fyne.WidgetRenderer {
	r := &fillRenderer{f, canvas.NewRectangle(theme.Color(f.name))}
	r.Refresh()
	return r
}

type fillRenderer struct {
	f *fill
	r *canvas.Rectangle
}

func (r *fillRenderer) Layout(s fyne.Size)           { r.r.Resize(s) }
func (r *fillRenderer) MinSize() fyne.Size           { return fyne.Size{} }
func (r *fillRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.r} }
func (r *fillRenderer) Destroy()                     {}
func (r *fillRenderer) Refresh() {
	r.r.FillColor = theme.Color(r.f.name)
	if r.f.round {
		r.r.CornerRadius = theme.Size(theme.SizeNameButtonRadius) + 2
	}
	r.r.Refresh()
}

// onBar is the look of what sits on the app bar: cream on teal, else
// the app's theme.
type onBar struct{}

func (onBar) current() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }

func (t onBar) Font(s fyne.TextStyle) fyne.Resource     { return t.current().Font(s) }
func (t onBar) Icon(n fyne.ThemeIconName) fyne.Resource { return t.current().Icon(n) }
func (t onBar) Size(n fyne.ThemeSizeName) float32       { return t.current().Size(n) }

func (t onBar) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameForeground:
		return cream
	case theme.ColorNameDisabled:
		return color.NRGBA{0xF5, 0xF2, 0xEC, 0x55}
	case theme.ColorNameButton:
		return color.NRGBA{0xFF, 0xFF, 0xFF, 0x1C}
	case theme.ColorNameDisabledButton:
		return color.NRGBA{0xFF, 0xFF, 0xFF, 0x0C}
	case theme.ColorNameHover:
		return color.NRGBA{0xFF, 0xFF, 0xFF, 0x22}
	case theme.ColorNamePressed:
		return color.NRGBA{0xFF, 0xFF, 0xFF, 0x33}
	}
	return t.current().Color(n, v)
}

// bar is the app bar: left (a Back button, say), the title, and right
// (actions), cream on teal.
//
// Every screen has one, the same way: Back on the left where there is
// somewhere to go back to, the screen's title and, under it in smaller
// text, sub (the hub or file the app shows, "" for none), and the
// screen's actions on the right. The title is returned, for screens whose
// title changes.
func bar(left fyne.CanvasObject, title, sub string, right fyne.CanvasObject) (fyne.CanvasObject, *widget.Label) {
	t := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	t.SizeName = theme.SizeNameSubHeadingText
	t.Truncation = fyne.TextTruncateEllipsis
	var middle fyne.CanvasObject = t
	if sub != "" {
		s := widget.NewLabel(sub)
		s.SizeName = theme.SizeNameCaptionText
		s.Truncation = fyne.TextTruncateEllipsis
		// The two lines close together, as one heading.
		middle = container.New(layout.NewCustomPaddedVBoxLayout(-2*theme.Padding()), t, s)
	}
	var sides []fyne.CanvasObject // Border takes no nil sides
	for _, o := range []fyne.CanvasObject{left, right} {
		if o != nil {
			sides = append(sides, o)
		}
	}
	content := container.NewBorder(nil, nil, left, right, append([]fyne.CanvasObject{middle}, sides...)...)
	return container.NewStack(newFill(colorBar), container.NewThemeOverride(container.NewPadded(content), onBar{})), t
}

// tiles are the drawn tiles, by icon and color.
var (
	tilesMu sync.Mutex
	tiles   = map[[2]uint8]fyne.Resource{}
)

// tile is a device's or activity's icon as the X2 remote shows it: drawn
// in cream on a rounded square of its color. Icons the X2 has no drawing
// for show fallback's.
func tile(icon, fallback profile.Icon, c profile.Color) fyne.Resource {
	if icon.Path() == "" {
		icon = fallback
	}
	key := [2]uint8{uint8(icon), uint8(c)}
	tilesMu.Lock()
	defer tilesMu.Unlock()
	if r, ok := tiles[key]; ok {
		return r
	}
	r, g, b := c.RGB()
	res := glyphTile(fmt.Sprintf("tile-%d-%d.svg", icon, c), color.NRGBA{r, g, b, 0xFF}, icon.Path())
	tiles[key] = res
	return res
}

// glyphTile is a 24x24 drawing, path, in cream on a rounded square of
// bg, as tile draws icons.
func glyphTile(name string, bg color.NRGBA, path string) fyne.Resource {
	// The 24x24 drawing, moved into the middle of the 36x36 square: a
	// view box starting below zero is cut at large sizes.
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="36" height="36" viewBox="0 0 36 36">`+
		`<rect x="0" y="0" width="36" height="36" rx="9" ry="9" fill="#%02X%02X%02X"/>`+
		`<g transform="translate(6 6)"><path fill="#F5F2EC" d="%s"/></g></svg>`, bg.R, bg.G, bg.B, path)
	return fyne.NewStaticResource(name, []byte(svg))
}

// Tiles' other colors, for the ways to add a device.
var (
	slate  = color.NRGBA{0x5A, 0x66, 0x6A, 0xFF}
	blue   = color.NRGBA{0x3B, 0x7D, 0xD8, 0xFF}
	green  = color.NRGBA{0x3F, 0xA8, 0x5F, 0xFF}
	amber  = color.NRGBA{0xD9, 0x8E, 0x2B, 0xFF}
	purple = color.NRGBA{0x6C, 0x3C, 0x97, 0xFF}
	cyan   = color.NRGBA{0x2A, 0x9D, 0xA6, 0xFF}
	violet = color.NRGBA{0x8A, 0x5C, 0xF6, 0xFF}
)

// Glyphs of the ways to add a device (Material's, Apache 2.0): generic
// drawings, not the services' logos.
const (
	pencilGlyph    = "M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04c.39-.39.39-1.02 0-1.41l-2.34-2.34c-.39-.39-1.02-.39-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z"
	folderGlyph    = "M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"
	bluetoothGlyph = "M17.71 7.71L12 2h-1v7.59L6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 11 14.41V22h1l5.71-5.71-4.3-4.29 4.3-4.29zM13 5.83l1.88 1.88L13 9.59V5.83zm1.88 10.46L13 18.17v-3.76l1.88 1.88z"
	globeGlyph     = "M11.99 2C6.47 2 2 6.48 2 12s4.47 10 9.99 10C17.52 22 22 17.52 22 12S17.52 2 11.99 2zm6.93 6h-2.95c-.32-1.25-.78-2.45-1.38-3.56 1.84.63 3.37 1.91 4.33 3.56zM12 4.04c.83 1.2 1.48 2.53 1.91 3.96h-3.82c.43-1.43 1.08-2.76 1.91-3.96zM4.26 14C4.1 13.36 4 12.69 4 12s.1-1.36.26-2h3.38c-.08.66-.14 1.32-.14 2 0 .68.06 1.34.14 2H4.26zm.82 2h2.95c.32 1.25.78 2.45 1.38 3.56-1.84-.63-3.37-1.9-4.33-3.56zm2.95-8H5.08c.96-1.66 2.49-2.93 4.33-3.56C8.81 5.55 8.35 6.75 8.03 8zM12 19.96c-.83-1.2-1.48-2.53-1.91-3.96h3.82c-.43 1.43-1.08 2.76-1.91 3.96zM14.34 14H9.66c-.09-.66-.16-1.32-.16-2 0-.68.07-1.35.16-2h4.68c.09.65.16 1.32.16 2 0 .68-.07 1.34-.16 2zm.25 5.56c.6-1.11 1.06-2.31 1.38-3.56h2.95c-.96 1.65-2.49 2.93-4.33 3.56zM16.36 14c.08-.66.14-1.32.14-2 0-.68-.06-1.34-.14-2h3.38c.16.64.26 1.31.26 2s-.1 1.36-.26 2h-3.38z"
	bulbGlyph      = "M9 21c0 .55.45 1 1 1h4c.55 0 1-.45 1-1v-1H9v1zm3-19C8.14 2 5 5.14 5 9c0 2.38 1.19 4.47 3 5.74V17c0 .55.45 1 1 1h6c.55 0 1-.45 1-1v-2.26c1.81-1.27 3-3.36 3-5.74 0-3.86-3.14-7-7-7z"
	tvGlyph        = "M21 3H3c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h5v2h8v-2h5c1.1 0 1.99-.9 1.99-2L23 5c0-1.1-.9-2-2-2zm0 14H3V5h18v12z"
	speakerGlyph   = "M17 2H7c-1.1 0-2 .9-2 2v16c0 1.1.9 1.99 2 1.99L17 22c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm-5 2c1.1 0 2 .9 2 2s-.9 2-2 2c-1.11 0-2-.9-2-2s.89-2 2-2zm0 16c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z"
)

// addTile is the tile that adds a device or activity: a plus, on grey
// (Material's add, Apache 2.0).
var addTile = glyphTile("add-tile.svg", color.NRGBA{0x8A, 0x96, 0x98, 0xFF}, "M11 5h2v6h6v2h-6v6h-2v-6H5v-2h6z")

// hubArt is the app's own drawing of a hub, as the X2's looks: wider
// than tall, by hubArtRatio.
//
//go:embed art/hub.svg
var hubArtSVG []byte

var hubArt = fyne.NewStaticResource("hub.svg", hubArtSVG)

const hubArtRatio = 240.0 / 150

// hubImage is the hub's drawing, height tall.
func hubImage(height float32) *canvas.Image {
	img := canvas.NewImageFromResource(hubArt)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(height*hubArtRatio, height))
	return img
}

// Fallback icons, for devices and activities with none the X2 draws.
const (
	deviceIcon   profile.Icon = 1  // Monitor
	activityIcon profile.Icon = 17 // Media player
)

// Tile sizes, before Scale: in lists, and beside headings.
const (
	rowTile     = 32
	headingTile = 52
)

// fitTabs lays tabs out across the width, showing only the tabs' icons
// when their names do not fit: Fyne puts the tabs that do not fit in a
// menu instead.
type fitTabs struct {
	tabs  *container.AppTabs
	names []string
	short bool
}

func newFitTabs(tabs *container.AppTabs) *fyne.Container {
	f := &fitTabs{tabs: tabs}
	for _, it := range tabs.Items {
		f.names = append(f.names, it.Text)
	}
	return container.New(f, tabs)
}

// needs is how wide the tabs are with their names, as Fyne lays them out:
// in cells of the widest on phones, else side by side.
func (f *fitTabs) needs() float32 {
	th := fyne.CurrentApp().Settings().Theme()
	text, icon := th.Size(theme.SizeNameText), th.Size(theme.SizeNameInlineIcon)
	pad, inner := th.Size(theme.SizeNamePadding), th.Size(theme.SizeNameInnerPadding)
	var sum, widest float32
	for _, n := range f.names {
		w := fyne.MeasureText(n, text, fyne.TextStyle{Bold: true}).Width
		widest = max(widest, w, 1.5*icon)
		sum += icon + pad + w + 4*inner + pad
	}
	if fyne.CurrentDevice().IsMobile() {
		return float32(len(f.names)) * (widest + 2*inner + pad)
	}
	return sum
}

func (f *fitTabs) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if short := f.needs() > size.Width; short != f.short {
		f.short = short
		for i, it := range f.tabs.Items {
			if short {
				it.Text = ""
			} else {
				it.Text = f.names[i]
			}
		}
		f.tabs.Refresh()
	}
	for _, o := range objs {
		o.Resize(size)
		o.Move(fyne.Position{})
	}
}

func (f *fitTabs) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		s = s.Max(o.MinSize())
	}
	return s
}
