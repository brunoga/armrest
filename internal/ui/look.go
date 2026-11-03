package ui

import (
	"fmt"
	"image/color"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

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
		theme.ColorNamePlaceHolder:         color.NRGBA{0x7A, 0x84, 0x86, 0xFF},
		theme.ColorNameShadow:              color.NRGBA{0x1E, 0x2A, 0x2E, 0x22},
		theme.ColorNameHeaderBackground:    color.NRGBA{0xEC, 0xE7, 0xDF, 0xFF},
		theme.ColorNameWarning:             orange,
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
}{{"Smaller", 0.85}, {"Normal", 1}, {"Larger", 1.15}, {"Largest", 1.3}}

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

// fill is a rectangle of a theme color, following the theme.
type fill struct {
	widget.BaseWidget
	name fyne.ThemeColorName
}

func newFill(name fyne.ThemeColorName) *fill {
	f := &fill{name: name}
	f.ExtendBaseWidget(f)
	return f
}

func (f *fill) CreateRenderer() fyne.WidgetRenderer {
	return &fillRenderer{f, canvas.NewRectangle(theme.Color(f.name))}
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
func bar(left fyne.CanvasObject, title string, right fyne.CanvasObject) fyne.CanvasObject {
	t := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	t.SizeName = theme.SizeNameSubHeadingText
	t.Truncation = fyne.TextTruncateEllipsis
	var sides []fyne.CanvasObject // Border takes no nil sides
	for _, o := range []fyne.CanvasObject{left, right} {
		if o != nil {
			sides = append(sides, o)
		}
	}
	content := container.NewBorder(nil, nil, left, right, append([]fyne.CanvasObject{t}, sides...)...)
	return container.NewStack(newFill(colorBar), container.NewThemeOverride(container.NewPadded(content), onBar{}))
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
	// Fyne draws no transforms: the icon is made smaller by a view box
	// wider than its 24x24.
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="36" height="36" viewBox="-6 -6 36 36">`+
		`<rect x="-6" y="-6" width="36" height="36" rx="9" ry="9" fill="#%02X%02X%02X"/>`+
		`<path fill="#F5F2EC" d="%s"/></svg>`, r, g, b, icon.Path())
	res := fyne.NewStaticResource(fmt.Sprintf("tile-%d-%d.svg", icon, c), []byte(svg))
	tiles[key] = res
	return res
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
