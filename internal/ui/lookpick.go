package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/profile"
)

// lookPicker picks a device's or activity's icon and color as the remote
// shows them: its tile, tapped, opens the X2's icons drawn in the color
// picked, and the colors as swatches. Zero is unchanged: the hub's own.
type lookPicker struct {
	widget.BaseWidget
	win      fyne.Window
	Icon     profile.Icon
	Color    profile.Color
	fallback profile.Icon
	// OnChanged is called when either is picked.
	OnChanged func()

	img  *canvas.Image
	name *widget.Label
}

// newLookPicker picks an icon and a color, starting from icon and c;
// fallback is drawn while no icon is picked.
func newLookPicker(win fyne.Window, icon profile.Icon, c profile.Color, fallback profile.Icon) *lookPicker {
	p := &lookPicker{win: win, Icon: icon, Color: c, fallback: fallback}
	p.img = canvas.NewImageFromResource(nil)
	p.img.FillMode = canvas.ImageFillContain
	p.img.SetMinSize(fyne.NewSquareSize(headingTile * Scale()))
	p.name = wrapped("")
	p.ExtendBaseWidget(p)
	p.show()
	return p
}

// show draws the tile and names it.
func (p *lookPicker) show() {
	p.img.Resource = tile(p.Icon, p.fallback, p.Color)
	p.img.Refresh()
	p.name.SetText(lookName(p.Icon, p.Color))
}

// lookName says what an icon and color are, in words.
func lookName(icon profile.Icon, c profile.Color) string {
	if d := describeLook(icon, c); d != "" {
		return d
	}
	return i18n.T("The hub's own icon and color")
}

func (p *lookPicker) Tapped(*fyne.PointEvent) { p.open() }
func (p *lookPicker) Cursor() desktop.Cursor  { return desktop.PointerCursor }

func (p *lookPicker) AccessibilityLabel() string {
	return i18n.T("Icon and color: %s", p.name.Text)
}
func (p *lookPicker) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }

func (p *lookPicker) CreateRenderer() fyne.WidgetRenderer {
	// The whole row opens the picker: a button beside the tile does not
	// fit a phone.
	return widget.NewSimpleRenderer(container.NewBorder(nil, nil, container.NewCenter(p.img),
		container.NewCenter(quiet(widget.NewIcon(chevronIcon))), quiet(p.name)))
}

// open shows the icons and colors to pick from.
func (p *lookPicker) open() {
	icon, c := p.Icon, p.Color
	var d dialog.Dialog
	preview := canvas.NewImageFromResource(nil)
	preview.FillMode = canvas.ImageFillContain
	preview.SetMinSize(fyne.NewSquareSize(headingTile * Scale()))
	about := wrapped("")
	icons := container.NewGridWrap(fyne.NewSquareSize((rowTile+12)*Scale()))
	swatches := container.NewGridWrap(fyne.NewSquareSize((rowTile+8)*Scale()))
	var refresh func()
	refresh = func() {
		preview.Resource = tile(icon, p.fallback, c)
		preview.Refresh()
		about.SetText(lookName(icon, c))
		// The icons, drawn in the color picked, the one picked marked.
		icons.Objects = nil
		for i := uint8(1); i != 0; i++ {
			ic := profile.Icon(i)
			if !named(ic.String()) {
				break // past the named ones
			}
			if ic.Path() == "" {
				continue // not drawn on the X2
			}
			icons.Add(newPickTile(tile(ic, p.fallback, shown(c)), ic.String(), ic == icon, func() {
				icon = ic
				refresh()
			}))
		}
		icons.Refresh()
		swatches.Objects = nil
		for i := uint8(1); i != 0; i++ {
			col := profile.Color(i)
			if !named(col.String()) {
				break
			}
			r, g, b := col.RGB()
			swatches.Add(newSwatch(color.NRGBA{r, g, b, 0xFF}, col.String(), col == c, func() {
				c = col
				refresh()
			}))
		}
		swatches.Refresh()
	}
	refresh()
	reset := widget.NewButton(i18n.T("The hub's own"), func() {
		icon, c = 0, 0
		refresh()
	})
	reset.Importance = widget.LowImportance
	cancel := widget.NewButtonWithIcon(i18n.T("Cancel"), theme.CancelIcon(), func() { d.Hide() })
	done := widget.NewButtonWithIcon(i18n.T("Done"), theme.ConfirmIcon(), func() {
		p.Icon, p.Color = icon, c
		p.show()
		d.Hide()
		if p.OnChanged != nil {
			p.OnChanged()
		}
	})
	done.Importance = widget.HighImportance
	head := container.NewBorder(nil, nil, container.NewCenter(preview), reset, quiet(about))
	// As wide as fits, up to a readable width: the grids' least is one
	// tile.
	cs := p.win.Canvas().Size()
	body := container.NewVBox(newFitWidth(p.win, ReadableWidth*Scale()*0.75), groupTitle(i18n.T("Color")), swatches, groupTitle(i18n.T("Icon")), icons)
	content := container.NewBorder(head, container.NewGridWithColumns(2, cancel, done), nil, nil, container.NewVScroll(body))
	d = dialog.NewCustomWithoutButtons(i18n.T("Icon and color"), content, p.win)
	d.Show()
	d.Resize(fyne.NewSize(d.MinSize().Width, min(cs.Height*0.9, 640*Scale())))
}

// shown is c, or grey while no color is picked: icons need one to be
// drawn in.
func shown(c profile.Color) profile.Color {
	if c.IsZero() {
		for i := uint8(1); i != 0; i++ {
			if profile.Color(i).String() == "grey" {
				return profile.Color(i)
			}
		}
	}
	return c
}

// named reports whether a library name is a name, not a number.
func named(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return true
		}
	}
	return false
}

// pickTile is an icon to pick, ringed once picked.
type pickTile struct {
	widget.BaseWidget
	res     fyne.Resource
	name    string
	picked  bool
	onTap   func()
	hovered bool
}

func newPickTile(res fyne.Resource, name string, picked bool, tapped func()) *pickTile {
	t := &pickTile{res: res, name: name, picked: picked, onTap: tapped}
	t.ExtendBaseWidget(t)
	return t
}

func (t *pickTile) Tapped(*fyne.PointEvent) {
	tapFeedback()
	t.onTap()
}
func (t *pickTile) MouseIn(*desktop.MouseEvent)            { t.hovered = true; t.Refresh() }
func (t *pickTile) MouseMoved(*desktop.MouseEvent)         {}
func (t *pickTile) MouseOut()                              { t.hovered = false; t.Refresh() }
func (t *pickTile) Cursor() desktop.Cursor                 { return desktop.PointerCursor }
func (t *pickTile) AccessibilityLabel() string             { return t.name }
func (t *pickTile) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }

func (t *pickTile) CreateRenderer() fyne.WidgetRenderer {
	ring := canvas.NewRectangle(color.Transparent)
	img := canvas.NewImageFromResource(t.res)
	img.FillMode = canvas.ImageFillContain
	r := &ringedRenderer{ring: ring, inner: img, picked: func() bool { return t.picked }, hovered: func() bool { return t.hovered }, round: false}
	r.Refresh()
	return r
}

// swatch is a color to pick, ringed once picked.
type swatch struct {
	widget.BaseWidget
	c       color.NRGBA
	name    string
	picked  bool
	onTap   func()
	hovered bool
}

func newSwatch(c color.NRGBA, name string, picked bool, tapped func()) *swatch {
	s := &swatch{c: c, name: name, picked: picked, onTap: tapped}
	s.ExtendBaseWidget(s)
	return s
}

func (s *swatch) Tapped(*fyne.PointEvent) {
	tapFeedback()
	s.onTap()
}
func (s *swatch) MouseIn(*desktop.MouseEvent)            { s.hovered = true; s.Refresh() }
func (s *swatch) MouseMoved(*desktop.MouseEvent)         {}
func (s *swatch) MouseOut()                              { s.hovered = false; s.Refresh() }
func (s *swatch) Cursor() desktop.Cursor                 { return desktop.PointerCursor }
func (s *swatch) AccessibilityLabel() string             { return s.name }
func (s *swatch) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }

func (s *swatch) CreateRenderer() fyne.WidgetRenderer {
	ring := canvas.NewCircle(color.Transparent)
	dot := canvas.NewCircle(s.c)
	r := &ringedRenderer{ringCircle: ring, inner: dot, picked: func() bool { return s.picked }, hovered: func() bool { return s.hovered }, round: true}
	r.Refresh()
	return r
}

// ringedRenderer draws inner inset in a ring: the primary color's once
// picked, the hover's while the pointer is over it.
type ringedRenderer struct {
	ring            *canvas.Rectangle
	ringCircle      *canvas.Circle
	inner           fyne.CanvasObject
	picked, hovered func() bool
	round           bool
	size            fyne.Size
}

func (r *ringedRenderer) Layout(s fyne.Size) {
	r.size = s
	d := min(s.Width, s.Height)
	at := fyne.NewPos((s.Width-d)/2, (s.Height-d)/2)
	if r.round {
		r.ringCircle.Resize(fyne.NewSquareSize(d))
		r.ringCircle.Move(at)
	} else {
		r.ring.Resize(fyne.NewSquareSize(d))
		r.ring.Move(at)
		r.ring.CornerRadius = d * 0.28
	}
	in := d * 0.16
	r.inner.Resize(fyne.NewSquareSize(d - 2*in))
	r.inner.Move(at.AddXY(in, in))
}

func (r *ringedRenderer) MinSize() fyne.Size { return fyne.NewSquareSize(rowTile * Scale()) }

func (r *ringedRenderer) Refresh() {
	var stroke color.Color = color.Transparent
	var fill color.Color = color.Transparent
	if r.hovered() {
		fill = theme.Color(theme.ColorNameHover)
	}
	if r.picked() {
		stroke = theme.Color(theme.ColorNamePrimary)
	}
	w := 2.5 * Scale()
	if r.round {
		r.ringCircle.StrokeColor, r.ringCircle.StrokeWidth, r.ringCircle.FillColor = stroke, w, fill
		r.ringCircle.Refresh()
	} else {
		r.ring.StrokeColor, r.ring.StrokeWidth, r.ring.FillColor = stroke, w, fill
		r.ring.Refresh()
	}
	r.inner.Refresh()
	r.Layout(r.size)
}

func (r *ringedRenderer) Objects() []fyne.CanvasObject {
	if r.round {
		return []fyne.CanvasObject{r.ringCircle, r.inner}
	}
	return []fyne.CanvasObject{r.ring, r.inner}
}

func (r *ringedRenderer) Destroy() {}

// fitWidth is nothing, as wide as the window has room for in a dialog,
// up to most: it widens a dialog whose content's least width is small.
type fitWidth struct {
	widget.BaseWidget
	win  fyne.Window
	most float32
}

func newFitWidth(win fyne.Window, most float32) *fitWidth {
	f := &fitWidth{win: win, most: most}
	f.ExtendBaseWidget(f)
	return f
}

func (f *fitWidth) CreateRenderer() fyne.WidgetRenderer {
	return &fitWidthRenderer{f}
}

type fitWidthRenderer struct{ f *fitWidth }

func (r *fitWidthRenderer) MinSize() fyne.Size {
	return fyne.NewSize(max(min(r.f.most, r.f.win.Canvas().Size().Width-8*theme.Padding()), 0), 0)
}
func (r *fitWidthRenderer) Layout(fyne.Size)             {}
func (r *fitWidthRenderer) Refresh()                     {}
func (r *fitWidthRenderer) Objects() []fyne.CanvasObject { return nil }
func (r *fitWidthRenderer) Destroy()                     {}
