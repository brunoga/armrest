package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The app's own widgets, beside Fyne's: rows to tap, grouped as a list
// in a card; chips saying a state; a segmented control; a switch.

// muted is the look of secondary text: the placeholder's color, readable
// but quieter than the text it explains.
type muted struct{}

func (muted) current() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }

func (t muted) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == theme.ColorNameForeground {
		return t.current().Color(theme.ColorNamePlaceHolder, v)
	}
	return t.current().Color(n, v)
}
func (t muted) Font(s fyne.TextStyle) fyne.Resource     { return t.current().Font(s) }
func (t muted) Icon(n fyne.ThemeIconName) fyne.Resource { return t.current().Icon(n) }
func (t muted) Size(n fyne.ThemeSizeName) float32       { return t.current().Size(n) }

// quiet is o in muted text.
func quiet(o fyne.CanvasObject) fyne.CanvasObject { return container.NewThemeOverride(o, muted{}) }

// navRow is a row to tap: a tile or icon on the left, a title and a line
// under it, and on the right a chevron, or what trailing holds. Hovered
// and pressed it is tinted, as buttons are.
type navRow struct {
	widget.BaseWidget
	lead     fyne.Resource
	title    *widget.Label
	sub      *widget.Label
	trailing fyne.CanvasObject
	// OnTapped is called on a tap; without it the row is only shown.
	OnTapped func()

	hovered bool
	// tileSize is the lead's size, before Scale.
	tileSize float32
}

// newNavRow is a row titled title, with sub under it ("" for none),
// lead on its left (nil for none), calling tapped when tapped.
func newNavRow(lead fyne.Resource, title, sub string, tapped func()) *navRow {
	r := &navRow{lead: lead, OnTapped: tapped, tileSize: rowTile}
	r.title = widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	r.title.Wrapping = fyne.TextWrapWord
	r.sub = wrapped(sub)
	if sub == "" {
		r.sub.Hide()
	}
	r.ExtendBaseWidget(r)
	return r
}

// SetSub sets the line under the title.
func (r *navRow) SetSub(sub string) {
	r.sub.SetText(sub)
	r.sub.Hidden = sub == ""
	r.Refresh()
}

func (r *navRow) Tapped(*fyne.PointEvent) {
	if r.OnTapped != nil {
		tapFeedback()
		r.OnTapped()
	}
}

func (r *navRow) MouseIn(*desktop.MouseEvent)    { r.hover(true) }
func (r *navRow) MouseMoved(*desktop.MouseEvent) {}
func (r *navRow) MouseOut()                      { r.hover(false) }

func (r *navRow) hover(on bool) {
	if r.OnTapped == nil || r.hovered == on {
		return
	}
	r.hovered = on
	r.Refresh()
}

// Cursor is a hand where the row does something.
func (r *navRow) Cursor() desktop.Cursor {
	if r.OnTapped == nil {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (r *navRow) AccessibilityLabel() string { return r.title.Text }

func (r *navRow) AccessibilityRole() fyne.AccessibleRole {
	if r.OnTapped == nil {
		return fyne.AccessibleRoleText
	}
	return fyne.AccessibleRoleButton
}

func (r *navRow) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	var left fyne.CanvasObject
	if r.lead != nil {
		img := canvas.NewImageFromResource(r.lead)
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSquareSize(r.tileSize * Scale()))
		left = container.NewCenter(img)
	}
	trailing := r.trailing
	if trailing == nil && r.OnTapped != nil {
		trailing = quiet(widget.NewIcon(chevronIcon))
	}
	var right fyne.CanvasObject
	if trailing != nil {
		right = container.NewCenter(trailing)
	}
	text := container.New(layout.NewCustomPaddedVBoxLayout(-2.5*theme.Padding()), r.title, quiet(r.sub))
	objs := []fyne.CanvasObject{text}
	for _, o := range []fyne.CanvasObject{left, right} {
		if o != nil {
			objs = append(objs, o)
		}
	}
	body := container.New(layout.NewCustomPaddedLayout(0, 0, theme.Padding(), theme.Padding()),
		container.NewBorder(nil, nil, left, right, objs...))
	return &navRowRenderer{r: r, bg: bg, body: body}
}

type navRowRenderer struct {
	r    *navRow
	bg   *canvas.Rectangle
	body *fyne.Container
}

func (n *navRowRenderer) Layout(s fyne.Size) {
	n.bg.Resize(s)
	n.body.Resize(s)
}
func (n *navRowRenderer) MinSize() fyne.Size { return n.body.MinSize() }
func (n *navRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{n.bg, n.body}
}
func (n *navRowRenderer) Destroy() {}
func (n *navRowRenderer) Refresh() {
	n.bg.FillColor = color.Transparent
	if n.r.hovered {
		n.bg.FillColor = theme.Color(theme.ColorNameHover)
	}
	n.bg.CornerRadius = theme.Size(theme.SizeNameInputRadius)
	n.bg.Refresh()
	n.body.Refresh()
}

// chevronIcon points right, at where a row goes (Material's
// chevron_right, Apache 2.0).
var chevronIcon = theme.NewThemedResource(fyne.NewStaticResource("chevron.svg", []byte(
	`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#000" d="M9.3 6.7a1 1 0 0 1 1.4 0l4.6 4.6a1 1 0 0 1 0 1.4l-4.6 4.6a1 1 0 0 1-1.4-1.4l3.9-3.9-3.9-3.9a1 1 0 0 1 0-1.4z"/></svg>`)))

// group is a card of rows, a rule between each, under a small title
// ("" for none): a list, as phones' settings show them.
func group(title string, rows ...fyne.CanvasObject) fyne.CanvasObject {
	box := container.NewVBox()
	for i, r := range rows {
		if i > 0 {
			box.Add(widget.NewSeparator())
		}
		box.Add(r)
	}
	body := widget.NewCard("", "", box)
	if title == "" {
		return body
	}
	return container.NewVBox(groupTitle(title), body)
}

// groupTitle is a group's title: small, bold and muted, over its card.
func groupTitle(title string) fyne.CanvasObject {
	t := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	t.SizeName = theme.SizeNameCaptionText
	t.Wrapping = fyne.TextWrapWord
	return quiet(t)
}

// The kinds of chip, by what they say.
const (
	chipNew     = iota // added: the app's teal
	chipEdited         // changed: orange
	chipRemoved        // removed: red
	chipInfo           // anything else: grey
)

// chip is a short word in a rounded tint: a row's state, a step's kind.
type chip struct {
	widget.BaseWidget
	text string
	kind int
}

func newChip(text string, kind int) *chip {
	c := &chip{text: text, kind: kind}
	c.ExtendBaseWidget(c)
	return c
}

// SetText changes what the chip says; "" hides it.
func (c *chip) SetText(text string, kind int) {
	c.text, c.kind = text, kind
	c.Hidden = text == ""
	c.Refresh()
}

func (c *chip) AccessibilityLabel() string            { return c.text }
func (c *chip) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleText }

// colors are the chip's tint and text.
func (c *chip) colors() (bg, fg color.Color) {
	v := fyne.CurrentApp().Settings().ThemeVariant()
	tint := func(c color.Color, a uint8) color.Color {
		n := color.NRGBAModel.Convert(c).(color.NRGBA)
		n.A = a
		return n
	}
	var base color.Color
	switch c.kind {
	case chipNew:
		base = theme.Color(theme.ColorNamePrimary)
	case chipEdited:
		base = orange
	case chipRemoved:
		base = theme.Color(theme.ColorNameError)
	default:
		base = theme.Color(theme.ColorNamePlaceHolder)
	}
	fg = base
	if c.kind == chipEdited && v != theme.VariantDark {
		fg = color.NRGBA{0xA8, 0x5A, 0x0C, 0xFF} // orange, dark enough to read on cream
	}
	return tint(base, 0x2E), fg
}

func (c *chip) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	t := canvas.NewText(c.text, color.Black)
	t.TextStyle.Bold = true
	r := &chipRenderer{c: c, bg: bg, t: t}
	r.Refresh()
	return r
}

type chipRenderer struct {
	c  *chip
	bg *canvas.Rectangle
	t  *canvas.Text
}

func (r *chipRenderer) pad() (float32, float32) {
	return theme.Padding() * 1.5, theme.Padding() / 2
}

func (r *chipRenderer) Layout(s fyne.Size) {
	ts := r.t.MinSize()
	h := ts.Height + 2*(theme.Padding()/2)
	r.bg.Resize(fyne.NewSize(s.Width, h))
	r.bg.Move(fyne.NewPos(0, (s.Height-h)/2))
	r.t.Move(fyne.NewPos((s.Width-ts.Width)/2, (s.Height-ts.Height)/2))
	r.t.Resize(ts)
}

func (r *chipRenderer) MinSize() fyne.Size {
	x, y := r.pad()
	ts := r.t.MinSize()
	return fyne.NewSize(ts.Width+2*x, ts.Height+2*y)
}

func (r *chipRenderer) Refresh() {
	bg, fg := r.c.colors()
	r.t.Text = r.c.text
	r.t.TextSize = theme.Size(theme.SizeNameCaptionText)
	r.t.Color = fg
	r.bg.FillColor = bg
	r.bg.CornerRadius = r.MinSize().Height / 2
	r.t.Refresh()
	r.bg.Refresh()
	r.Layout(r.c.Size())
}

func (r *chipRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.bg, r.t} }
func (r *chipRenderer) Destroy()                     {}

// segmented picks one of a few options: buttons side by side in a
// rounded track, the one picked filled.
type segmented struct {
	widget.BaseWidget
	buttons  []*widget.Button
	selected int
	// OnSelected is called when an option is picked by a tap.
	OnSelected func(int)
	track      *fill
	grid       *fyne.Container
}

func newSegmented(names []string, selected int, picked func(int)) *segmented {
	return newSegmentedLook(names, nil, selected, picked)
}

// newSegmentedLook is a segmented control whose option i is drawn in
// looks[i], where it is set.
func newSegmentedLook(names []string, looks []fyne.Theme, selected int, picked func(int)) *segmented {
	s := &segmented{selected: selected, OnSelected: picked, track: newRoundFill(theme.ColorNameButton)}
	s.grid = container.New(&shareLayout{})
	for i, n := range names {
		b := widget.NewButton(n, func() {
			if i == s.selected {
				return
			}
			tapFeedback()
			s.Select(i)
			if s.OnSelected != nil {
				s.OnSelected(i)
			}
		})
		s.buttons = append(s.buttons, b)
		if i < len(looks) && looks[i] != nil {
			s.grid.Add(container.NewThemeOverride(b, looks[i]))
		} else {
			s.grid.Add(b)
		}
	}
	s.mark()
	s.ExtendBaseWidget(s)
	return s
}

// Select picks option i, without calling OnSelected.
func (s *segmented) Select(i int) {
	s.selected = i
	s.mark()
}

// Selected is the option picked.
func (s *segmented) Selected() int { return s.selected }

func (s *segmented) mark() {
	for j, b := range s.buttons {
		imp := widget.LowImportance
		if j == s.selected {
			imp = widget.HighImportance
		}
		if b.Importance != imp {
			b.Importance = imp
			b.Refresh()
		}
	}
}

func (s *segmented) CreateRenderer() fyne.WidgetRenderer {
	inner := container.New(layout.NewCustomPaddedLayout(2, 2, 2, 2), s.grid)
	return widget.NewSimpleRenderer(container.NewStack(s.track, container.NewThemeOverride(inner, tightKeys{})))
}

// shareLayout lays objects out in a row, each as wide as it needs plus
// an equal share of the room left: no wider than the sum of them, as a
// grid of the widest would be.
type shareLayout struct{}

func (shareLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	var need, widest float32
	for _, o := range objs {
		need += o.MinSize().Width
		widest = max(widest, o.MinSize().Width)
	}
	n := float32(max(len(objs), 1))
	extra := max(size.Width-need, 0) / n
	// All alike where they fit so.
	equal := widest*n <= size.Width
	x := float32(0)
	for _, o := range objs {
		w := o.MinSize().Width + extra
		if equal {
			w = size.Width / n
		}
		o.Resize(fyne.NewSize(w, size.Height))
		o.Move(fyne.NewPos(x, 0))
		x += w
	}
}

func (shareLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		m := o.MinSize()
		s.Width += m.Width
		s.Height = max(s.Height, m.Height)
	}
	return s
}

// toggle is a switch: on, its knob is right on the app's teal; off,
// left on grey. It slides when tapped.
type toggle struct {
	widget.BaseWidget
	On bool
	// OnChanged is called when a tap turns it on or off.
	OnChanged func(bool)
	// label names it, for screen readers.
	label string
	// at is where the knob is, 0 (off) to 1 (on), while it slides.
	at   float32
	anim *fyne.Animation
}

func newToggle(label string, on bool, changed func(bool)) *toggle {
	t := &toggle{label: label, On: on, OnChanged: changed}
	if on {
		t.at = 1
	}
	t.ExtendBaseWidget(t)
	return t
}

// SetOn turns it on or off, without calling OnChanged.
func (t *toggle) SetOn(on bool) {
	if t.On == on {
		return
	}
	t.On = on
	t.slide()
}

func (t *toggle) Tapped(*fyne.PointEvent) {
	tapFeedback()
	t.On = !t.On
	t.slide()
	if t.OnChanged != nil {
		t.OnChanged(t.On)
	}
}

func (t *toggle) slide() {
	from, to := t.at, float32(0)
	if t.On {
		to = 1
	}
	if t.anim != nil {
		t.anim.Stop()
	}
	t.anim = fyne.NewAnimation(150*time.Millisecond, func(f float32) {
		t.at = from + (to-from)*f
		t.Refresh()
	})
	t.anim.Curve = fyne.AnimationEaseOut
	t.anim.Start()
	// Shown at once where animations do not run (tests).
	t.at = to
	t.Refresh()
}

func (t *toggle) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (t *toggle) AccessibilityLabel() string {
	return t.label
}

func (t *toggle) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }

func (t *toggle) CreateRenderer() fyne.WidgetRenderer {
	track := canvas.NewRectangle(color.Transparent)
	knob := canvas.NewCircle(color.White)
	knob.Shadow = canvas.Shadow{Color: color.NRGBA{0, 0, 0, 0x44}, BlurRadius: 2, Offset: fyne.NewPos(0, 1)}
	r := &toggleRenderer{t: t, track: track, knob: knob}
	r.Refresh()
	return r
}

type toggleRenderer struct {
	t     *toggle
	track *canvas.Rectangle
	knob  *canvas.Circle
}

// size is the switch's, before Scale: about a phone's.
func (r *toggleRenderer) size() fyne.Size { return fyne.NewSize(40*Scale(), 24*Scale()) }

func (r *toggleRenderer) Layout(s fyne.Size) {
	sz := r.size()
	x0, y0 := s.Width-sz.Width, (s.Height-sz.Height)/2
	r.track.Resize(sz)
	r.track.Move(fyne.NewPos(x0, y0))
	inset := 3 * Scale()
	d := sz.Height - 2*inset
	x := x0 + inset + r.t.at*(sz.Width-sz.Height)
	r.knob.Resize(fyne.NewSquareSize(d))
	r.knob.Move(fyne.NewPos(x, y0+inset))
}

func (r *toggleRenderer) MinSize() fyne.Size { return r.size() }

func (r *toggleRenderer) Refresh() {
	off := theme.Color(theme.ColorNameInputBorder)
	on := theme.Color(theme.ColorNamePrimary)
	r.track.FillColor = mix(off, on, r.t.at)
	r.track.CornerRadius = r.size().Height / 2
	r.track.Refresh()
	r.knob.Refresh()
	r.Layout(r.t.Size())
}

func (r *toggleRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.track, r.knob} }
func (r *toggleRenderer) Destroy()                     {}

// mix is a and b mixed, f of the way to b.
func mix(a, b color.Color, f float32) color.Color {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	m := func(x, y uint32) uint8 { return uint8((float32(x>>8)*(1-f) + float32(y>>8)*f)) }
	return color.NRGBA{m(ar, br), m(ag, bg), m(ab, bb), m(aa, ba)}
}

// settingRow is a setting: its name and what it does, and its control
// on the right.
func settingRow(title, about string, control fyne.CanvasObject) fyne.CanvasObject {
	r := newNavRow(nil, title, about, nil)
	r.trailing = control
	return r
}

// launchTile is a device's or activity's tile, its name under it, to tap:
// a home screen's.
type launchTile struct {
	widget.BaseWidget
	res     fyne.Resource
	name    *widget.Label
	onTap   func()
	hovered bool
}

func newLaunchTile(res fyne.Resource, name string, tapped func()) *launchTile {
	t := &launchTile{res: res, onTap: tapped}
	t.name = widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{})
	t.name.Wrapping = fyne.TextWrapWord
	t.ExtendBaseWidget(t)
	return t
}

func (t *launchTile) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		tapFeedback()
		t.onTap()
	}
}

func (t *launchTile) MouseIn(*desktop.MouseEvent)    { t.hovered = true; t.Refresh() }
func (t *launchTile) MouseMoved(*desktop.MouseEvent) {}
func (t *launchTile) MouseOut()                      { t.hovered = false; t.Refresh() }
func (t *launchTile) Cursor() desktop.Cursor         { return desktop.PointerCursor }

func (t *launchTile) AccessibilityLabel() string            { return t.name.Text }
func (t *launchTile) AccessibilityRole() fyne.AccessibleRole { return fyne.AccessibleRoleButton }

func (t *launchTile) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	img := canvas.NewImageFromResource(t.res)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSquareSize(headingTile * Scale()))
	body := container.NewBorder(container.NewCenter(img), nil, nil, nil, container.New(layout.NewCustomPaddedLayout(-theme.Padding(), 0, 0, 0), t.name))
	r := &launchTileRenderer{t: t, bg: bg, body: container.NewPadded(body)}
	r.Refresh()
	return r
}

type launchTileRenderer struct {
	t    *launchTile
	bg   *canvas.Rectangle
	body *fyne.Container
}

func (r *launchTileRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	r.body.Resize(s)
}
func (r *launchTileRenderer) MinSize() fyne.Size { return r.body.MinSize() }
func (r *launchTileRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.body}
}
func (r *launchTileRenderer) Destroy() {}
func (r *launchTileRenderer) Refresh() {
	r.bg.FillColor = color.Transparent
	if r.t.hovered {
		r.bg.FillColor = theme.Color(theme.ColorNameHover)
	}
	r.bg.CornerRadius = theme.Size(theme.SizeNameCardRadius)
	r.bg.Refresh()
	r.body.Refresh()
}

// tileGrid lays tiles out in rows, as many across as fit: room for
// names of two lines.
func tileGrid(tiles []fyne.CanvasObject) *fyne.Container {
	return container.NewGridWrap(fyne.NewSize(120*Scale(), (headingTile+56)*Scale()), tiles...)
}

// textAt is the app's look with text factor times its size.
type textAt struct{ factor float32 }

func (textAt) current() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }

func (t textAt) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameText {
		return t.current().Size(n) * t.factor
	}
	return t.current().Size(n)
}
func (t textAt) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return t.current().Color(n, v)
}
func (t textAt) Font(s fyne.TextStyle) fyne.Resource     { return t.current().Font(s) }
func (t textAt) Icon(n fyne.ThemeIconName) fyne.Resource { return t.current().Icon(n) }

// besideOrUnder lays main out with side on its right where the window is
// wide enough for main to keep half its width, else side under main, on
// the right: a long text squeezed beside a button wraps into a tall,
// narrow column.
type besideOrUnder struct {
	// width is the last laid out at: the height depends on it.
	width float32
}

// stacks reports whether objs are laid out one under the other at width.
func (*besideOrUnder) stacks(objs []fyne.CanvasObject, width float32) bool {
	return len(objs) == 2 && objs[1].Visible() && objs[1].MinSize().Width > width/2
}

func (l *besideOrUnder) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	if len(objs) != 2 {
		return
	}
	l.width = size.Width
	main, side := objs[0], objs[1]
	p := theme.Padding()
	if !side.Visible() {
		main.Resize(size)
		main.Move(fyne.Position{})
		return
	}
	sm := side.MinSize()
	if l.stacks(objs, size.Width) {
		mh := main.MinSize().Height
		if !main.Visible() {
			mh = 0
		}
		main.Resize(fyne.NewSize(size.Width, mh))
		main.Move(fyne.Position{})
		side.Resize(sm)
		side.Move(fyne.NewPos(size.Width-sm.Width, size.Height-sm.Height))
		return
	}
	main.Resize(fyne.NewSize(size.Width-sm.Width-p, size.Height))
	main.Move(fyne.Position{})
	side.Resize(fyne.NewSize(sm.Width, size.Height))
	side.Move(fyne.NewPos(size.Width-sm.Width, 0))
}

func (l *besideOrUnder) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) != 2 {
		return fyne.Size{}
	}
	main, side := objs[0].MinSize(), objs[1].MinSize()
	if !objs[0].Visible() {
		main = fyne.Size{}
	}
	if !objs[1].Visible() {
		return main
	}
	// The narrowest: one under the other; the height, at the width last
	// laid out at, as wrapped text's is.
	h := max(main.Height, side.Height)
	if l.width == 0 || l.stacks(objs, l.width) {
		h = main.Height + side.Height + theme.Padding()
	}
	return fyne.NewSize(max(main.Width, side.Width), h)
}

// rowOrStack lays its objects out in a row, the first on the left and
// the others on the right, where they fit; else one under the other,
// full width, in reverse order (the main action, last, on top).
type rowOrStack struct {
	// width is the last laid out at: the height depends on it.
	width float32
}

func (l *rowOrStack) fits(objs []fyne.CanvasObject, width float32) bool {
	var need float32
	for _, o := range objs {
		need += o.MinSize().Width + theme.Padding()
	}
	return need <= width
}

func (l *rowOrStack) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	l.width = size.Width
	p := theme.Padding()
	if l.fits(objs, size.Width) {
		x := size.Width
		for i := len(objs) - 1; i >= 1; i-- {
			m := objs[i].MinSize()
			x -= m.Width
			objs[i].Resize(fyne.NewSize(m.Width, size.Height))
			objs[i].Move(fyne.NewPos(x, 0))
			x -= p
		}
		if len(objs) > 0 {
			objs[0].Resize(fyne.NewSize(objs[0].MinSize().Width, size.Height))
			objs[0].Move(fyne.Position{})
		}
		return
	}
	y := float32(0)
	for i := len(objs) - 1; i >= 0; i-- {
		h := objs[i].MinSize().Height
		objs[i].Resize(fyne.NewSize(size.Width, h))
		objs[i].Move(fyne.NewPos(0, y))
		y += h + p
	}
}

func (l *rowOrStack) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	var sum float32
	for _, o := range objs {
		m := o.MinSize()
		s.Width = max(s.Width, m.Width)
		s.Height = max(s.Height, m.Height)
		sum += m.Height + theme.Padding()
	}
	if l.width == 0 || !l.fits(objs, l.width) {
		s.Height = sum - theme.Padding()
	}
	return s
}
