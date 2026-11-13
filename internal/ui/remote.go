package ui

import (
	"fmt"
	"image/color"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/record"
)

// The remote, drawn as the X2's: a graphite body with the power key at
// the top, the ring of arrows around OK, Back, Home and Menu, volume and
// channels as rockers each side of Guide and Mute, the playback keys,
// the color keys and A to C. The number keys, which the X2 shows on its
// screen, are on a panel under it. Keys that run something are bright;
// the others dim. A key held down lights in the app's orange.

// The remote's colors: the same in light and dark, as the remote is.
var (
	remoteBody   = color.NRGBA{0x26, 0x30, 0x33, 0xFF}
	remoteRing   = color.NRGBA{0x31, 0x3C, 0x3F, 0xFF}
	keySet       = color.NRGBA{0x42, 0x50, 0x53, 0xFF}
	keyUnset     = color.NRGBA{0x2E, 0x38, 0x3B, 0xFF}
	keyHover     = color.NRGBA{0x4D, 0x5D, 0x60, 0xFF}
	inkSet       = color.NRGBA{0xF5, 0xF2, 0xEC, 0xFF}
	inkUnset     = color.NRGBA{0xF5, 0xF2, 0xEC, 0x48}
	inkLit       = color.NRGBA{0x1E, 0x2A, 0x2E, 0xFF}
	powerInk     = color.NRGBA{0xFF, 0x7A, 0x6B, 0xFF}
	remoteScreen = color.NRGBA{0x1B, 0x23, 0x25, 0xFF}
)

// The drawing's size, in its own units: it is scaled to the width it
// has, up to remoteWidth times Scale.
const (
	remoteW     = 300
	remoteH     = 626
	remoteWidth = 300
)

// keyShape is how a key is drawn.
type keyShape int

const (
	shapeRound   keyShape = iota // a circle
	shapeArrow                   // on the ring: lit as a circle only
	shapeUpper                   // a rocker's upper half
	shapeLower                   // a rocker's lower half
	shapePill                    // a color key, a number key
)

// keySpot is where a key is on the drawing, in its units: its center and
// size, and its shape.
type keySpot struct {
	x, y, w, h float32
	shape      keyShape
}

// remoteSpots are the keys of the X2's remote, where they are.
var remoteSpots = map[string]keySpot{
	"Power": {150, 34, 42, 42, shapeRound},

	"Up":    {150, 102, 64, 50, shapeArrow},
	"Down":  {150, 254, 64, 50, shapeArrow},
	"Left":  {74, 178, 50, 64, shapeArrow},
	"Right": {226, 178, 50, 64, shapeArrow},
	"OK":    {150, 178, 78, 78, shapeRound},

	"Back": {70, 300, 46, 46, shapeRound},
	"Home": {150, 300, 46, 46, shapeRound},
	"Menu": {230, 300, 46, 46, shapeRound},

	"Vol+":  {58, 356, 50, 44, shapeUpper},
	"Vol-":  {58, 414, 50, 44, shapeLower},
	"Guide": {150, 362, 42, 42, shapeRound},
	"Mute":  {150, 410, 42, 42, shapeRound},
	"Ch+":   {242, 356, 50, 44, shapeUpper},
	"Ch-":   {242, 414, 50, 44, shapeLower},

	"Rewind":  {70, 464, 42, 42, shapeRound},
	"Play":    {150, 464, 42, 42, shapeRound},
	"Forward": {230, 464, 42, 42, shapeRound},
	"DVR":     {70, 514, 42, 42, shapeRound},
	"Pause":   {150, 514, 42, 42, shapeRound},
	"Exit":    {230, 514, 42, 42, shapeRound},

	"Red":    {72, 558, 46, 18, shapePill},
	"Green":  {124, 558, 46, 18, shapePill},
	"Yellow": {176, 558, 46, 18, shapePill},
	"Blue":   {228, 558, 46, 18, shapePill},

	"A": {90, 596, 36, 36, shapeRound},
	"B": {150, 596, 36, 36, shapeRound},
	"C": {210, 596, 36, 36, shapeRound},
}

// The rockers' names, between their halves.
var rockerLabels = []struct {
	text string
	x, y float32
}{{"VOL", 58, 385}, {"CH", 242, 385}}

// The ring around OK: its center and diameter.
const ringX, ringY, ringD = 150, 178, 196

// numberSpots are the number keys, on the panel under the remote, in
// the panel's units: three across.
var numberSpots = func() map[string]keySpot {
	out := map[string]keySpot{}
	for r, row := range numberRows {
		for c, k := range row {
			out[k] = keySpot{60 + float32(c)*90, 30 + float32(r)*48, 80, 38, shapePill}
		}
	}
	return out
}()

// numbersH is the panel's height, in its units.
const numbersH = 30 + 3*48 + 30

// keyIcons are the keys drawn as icons (Material's, Apache 2.0), by key.
// The others show their names.
var keyIcons = map[string]string{
	"Power":   "M13 3h-2v10h2V3zm4.83 2.17l-1.42 1.42C17.99 7.86 19 9.81 19 12c0 3.87-3.13 7-7 7s-7-3.13-7-7c0-2.19 1.01-4.14 2.58-5.42L6.17 5.17C4.23 6.82 3 9.26 3 12c0 4.97 4.03 9 9 9s9-4.03 9-9c0-2.74-1.23-5.18-3.17-6.83z",
	"Up":      "M12 8l-6 6 1.41 1.41L12 10.83l4.59 4.58L18 14z",
	"Down":    "M16.59 8.59L12 13.17 7.41 8.59 6 10l6 6 6-6z",
	"Left":    "M15.41 7.41L14 6l-6 6 6 6 1.41-1.41L10.83 12z",
	"Right":   "M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z",
	"Back":    "M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2z",
	"Home":    "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z",
	"Menu":    "M3 18h18v-2H3v2zm0-5h18v-2H3v2zm0-7v2h18V6H3z",
	"Guide":   "M4 8h4V4H4v4zm6 12h4v-4h-4v4zm-6 0h4v-4H4v4zm0-6h4v-4H4v4zm6 0h4v-4h-4v4zm6-10v4h4V4h-4zm-6 4h4V4h-4v4zm6 6h4v-4h-4v4zm0 6h4v-4h-4v4z",
	"Mute":    "M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z",
	"Vol+":    "M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z",
	"Ch+":     "M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z",
	"Vol-":    "M19 13H5v-2h14v2z",
	"Ch-":     "M19 13H5v-2h14v2z",
	"Rewind":  "M11 18V6l-8.5 6 8.5 6zm.5-6l8.5 6V6l-8.5 6z",
	"Play":    "M8 5v14l11-7z",
	"Forward": "M4 18l8.5-6L4 6v12zm9-12v12l8.5-6L13 6z",
	"DVR":     "M12 7a5 5 0 1 0 0 10a5 5 0 1 0 0-10z",
	"Pause":   "M6 19h4V5H6v14zm8-14v14h4V5h-4z",
	"Exit":    "M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z",
}

// icons are the keys' icons drawn, by key and color.
var (
	iconsMu sync.Mutex
	icons   = map[string]fyne.Resource{}
)

// keyIcon is key's icon in c, or nil for a key without one.
func keyIcon(key string, c color.NRGBA) fyne.Resource {
	path, ok := keyIcons[key]
	if !ok {
		return nil
	}
	name := fmt.Sprintf("key-%s-%02X%02X%02X%02X.svg", strings.NewReplacer("+", "plus", "-", "minus").Replace(key), c.R, c.G, c.B, c.A)
	iconsMu.Lock()
	defer iconsMu.Unlock()
	if r, ok := icons[name]; ok {
		return r
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#%02X%02X%02X" fill-opacity="%.3f" d="%s"/></svg>`,
		c.R, c.G, c.B, float32(c.A)/255, path)
	r := fyne.NewStaticResource(name, []byte(svg))
	icons[name] = r
	return r
}

// padKey is a key of the drawn remote: a button, drawn as the remote's
// key named key. HighImportance marks a key that runs something;
// WarningImportance one held down.
type padKey struct {
	widget.Button
	key     string
	shape   keyShape
	hovered bool
}

func newPadKey(key, label string, tapped func()) *padKey {
	k := &padKey{}
	k.init(key, label, tapped)
	k.ExtendBaseWidget(k)
	return k
}

// init sets the key up, for padKey and the widgets made of one.
func (k *padKey) init(key, label string, tapped func()) {
	k.key, k.Text, k.OnTapped = key, label, tapped
	if s, ok := remoteSpots[key]; ok {
		k.shape = s.shape
	} else {
		k.shape = shapePill
	}
}

func (k *padKey) MouseIn(e *desktop.MouseEvent) {
	k.hovered = true
	k.Button.MouseIn(e)
}

func (k *padKey) MouseOut() {
	k.hovered = false
	k.Button.MouseOut()
}

func (k *padKey) AccessibilityLabel() string { return k.key }

// look is the key's fill and ink.
func (k *padKey) look() (fill, ink color.NRGBA) {
	set := k.Importance == widget.HighImportance || k.Importance == widget.WarningImportance
	fill, ink = keyUnset, inkUnset
	if set {
		fill, ink = keySet, inkSet
		if k.key == "Power" {
			ink = powerInk
		}
	}
	if c, ok := keyColors[k.key]; ok {
		fill = c
		if !set {
			fill.A = 0x50
		}
	}
	if k.hovered && !k.Disabled() {
		fill = keyHover
		if c, ok := keyColors[k.key]; ok {
			fill = c
		}
	}
	if k.Importance == widget.WarningImportance {
		fill, ink = orange, inkLit
	}
	return fill, ink
}

func (k *padKey) CreateRenderer() fyne.WidgetRenderer {
	r := &padKeyRenderer{k: k, bg: canvas.NewRectangle(color.Transparent), icon: canvas.NewImageFromResource(nil), text: canvas.NewText("", inkSet)}
	r.icon.FillMode = canvas.ImageFillContain
	r.text.TextStyle.Bold = true
	r.text.Alignment = fyne.TextAlignCenter
	r.Refresh()
	return r
}

type padKeyRenderer struct {
	k    *padKey
	bg   *canvas.Rectangle
	icon *canvas.Image
	text *canvas.Text
}

func (r *padKeyRenderer) Layout(s fyne.Size) {
	d := min(s.Width, s.Height)
	bg, at := s, fyne.Position{}
	if r.k.shape == shapeRound || r.k.shape == shapeArrow {
		bg, at = fyne.NewSquareSize(d), fyne.NewPos((s.Width-d)/2, (s.Height-d)/2)
	}
	r.bg.Resize(bg)
	r.bg.Move(at)
	r.bg.CornerRadius, r.bg.TopLeftCornerRadius, r.bg.TopRightCornerRadius = 0, 0, 0
	r.bg.BottomLeftCornerRadius, r.bg.BottomRightCornerRadius = 0, 0
	small := d * 0.18
	switch r.k.shape {
	case shapeRound, shapeArrow, shapePill:
		r.bg.CornerRadius = min(bg.Width, bg.Height) / 2
	case shapeUpper:
		r.bg.TopLeftCornerRadius, r.bg.TopRightCornerRadius = s.Width/2, s.Width/2
		r.bg.BottomLeftCornerRadius, r.bg.BottomRightCornerRadius = small, small
	case shapeLower:
		r.bg.TopLeftCornerRadius, r.bg.TopRightCornerRadius = small, small
		r.bg.BottomLeftCornerRadius, r.bg.BottomRightCornerRadius = s.Width/2, s.Width/2
	}
	is := d * 0.5
	if r.k.shape == shapeArrow {
		is = d * 0.62
	}
	r.icon.Resize(fyne.NewSquareSize(is))
	r.icon.Move(fyne.NewPos((s.Width-is)/2, (s.Height-is)/2))
	r.text.TextSize = d * 0.34
	if r.k.shape == shapePill {
		r.text.TextSize = d * 0.42
	}
	if len([]rune(r.text.Text)) > 2 {
		r.text.TextSize *= 0.7
	}
	ts := r.text.MinSize()
	r.text.Resize(ts)
	r.text.Move(fyne.NewPos((s.Width-ts.Width)/2, (s.Height-ts.Height)/2))
}

// MinSize is a finger's, for keys drawn on their own; the drawing sizes
// its keys itself.
func (r *padKeyRenderer) MinSize() fyne.Size { return fyne.NewSquareSize(24) }

func (r *padKeyRenderer) Refresh() {
	fill, ink := r.k.look()
	r.bg.FillColor = fill
	if r.k.shape == shapeArrow && (fill == keySet || fill == keyUnset) {
		// On the ring the arrows are the ring's, lit only when touched.
		r.bg.FillColor = color.Transparent
	}
	if _, ok := keyColors[r.k.key]; ok {
		ink = color.NRGBA{}
	}
	r.icon.Resource = keyIcon(r.k.key, ink)
	r.icon.Hidden = r.icon.Resource == nil
	r.text.Text = ""
	if r.icon.Hidden && ink.A > 0 {
		r.text.Text = r.k.Text
	}
	r.text.Color = ink
	r.bg.Refresh()
	r.icon.Refresh()
	r.text.Refresh()
	r.Layout(r.k.Size())
}

func (r *padKeyRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.icon, r.text}
}

func (r *padKeyRenderer) Destroy() {}

// remoteDrawing is the remote drawn: its body and ring, and its keys
// where they are; the number keys on their panel under it.
type remoteDrawing struct {
	widget.BaseWidget
	keys    map[string]fyne.CanvasObject
	numbers map[string]fyne.CanvasObject
	// note says what the number keys are, over their panel.
	note *widget.Label
}

func (d *remoteDrawing) CreateRenderer() fyne.WidgetRenderer {
	body := canvas.NewRectangle(remoteBody)
	// No shadow or stroke: the software painter (tests, screenshots) blurs
	// and strokes slowly, and the body stands out from the page anyway.
	ring := canvas.NewCircle(remoteRing)
	screen := canvas.NewRectangle(remoteScreen)
	r := &remoteRenderer{d: d, body: body, ring: ring, screen: screen}
	for _, l := range rockerLabels {
		t := canvas.NewText(l.text, inkUnset)
		t.TextStyle.Bold = true
		r.labels = append(r.labels, t)
	}
	r.objects = []fyne.CanvasObject{body, ring}
	for _, t := range r.labels {
		r.objects = append(r.objects, t)
	}
	for _, k := range d.keys {
		r.objects = append(r.objects, k)
	}
	if len(d.numbers) > 0 {
		r.objects = append(r.objects, d.note, screen)
		for _, k := range d.numbers {
			r.objects = append(r.objects, k)
		}
	} else {
		screen.Hide()
	}
	return r
}

type remoteRenderer struct {
	d                  *remoteDrawing
	body, screen       *canvas.Rectangle
	ring               *canvas.Circle
	labels             []*canvas.Text
	objects            []fyne.CanvasObject
	lastWidth, lastTop float32
}

// scale is how many pixels a unit of the drawing is, at width.
func (r *remoteRenderer) scale(width float32) float32 {
	return min(width, remoteWidth*Scale()) / remoteW
}

func (r *remoteRenderer) Layout(s fyne.Size) {
	r.lastWidth = s.Width
	k := r.scale(s.Width)
	w := remoteW * k
	x0 := (s.Width - w) / 2
	place := func(o fyne.CanvasObject, at keySpot, top float32) {
		o.Resize(fyne.NewSize(at.w*k, at.h*k))
		o.Move(fyne.NewPos(x0+(at.x-at.w/2)*k, top+(at.y-at.h/2)*k))
	}
	r.body.Resize(fyne.NewSize(w, remoteH*k))
	r.body.Move(fyne.NewPos(x0, 0))
	r.body.CornerRadius = 46 * k
	r.ring.Resize(fyne.NewSquareSize(ringD * k))
	r.ring.Move(fyne.NewPos(x0+(ringX-ringD/2)*k, (ringY-ringD/2)*k))
	for i, l := range rockerLabels {
		t := r.labels[i]
		t.TextSize = 11 * k
		ts := t.MinSize()
		t.Resize(ts)
		t.Move(fyne.NewPos(x0+l.x*k-ts.Width/2, l.y*k-ts.Height/2))
	}
	for name, o := range r.d.keys {
		place(o, remoteSpots[name], 0)
	}
	if len(r.d.numbers) == 0 {
		return
	}
	top := remoteH*k + 2*k*12
	nh := r.d.note.MinSize().Height
	r.d.note.Resize(fyne.NewSize(s.Width, nh))
	r.d.note.Move(fyne.NewPos(0, top))
	top += nh
	r.lastTop = top
	r.screen.Resize(fyne.NewSize(w, numbersH*k))
	r.screen.Move(fyne.NewPos(x0, top))
	r.screen.CornerRadius = 24 * k
	for name, o := range r.d.numbers {
		place(o, numberSpots[name], top)
	}
}

func (r *remoteRenderer) MinSize() fyne.Size {
	// As tall as the width last laid out at makes it: the drawing
	// scales with its width.
	k := r.scale(r.lastWidth)
	if r.lastWidth == 0 {
		k = r.scale(remoteWidth * Scale())
	}
	h := remoteH * k
	if len(r.d.numbers) > 0 {
		h += 2*k*12 + r.d.note.MinSize().Height + numbersH*k
	}
	return fyne.NewSize(remoteW*0.8, h)
}

func (r *remoteRenderer) Refresh() {
	r.d.note.Refresh()
	for _, o := range r.objects {
		o.Refresh()
	}
}

func (r *remoteRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *remoteRenderer) Destroy()                     {}

// newRemoteDrawing draws the remote of the hub model with the keys key
// makes, labelled label (the X1 remotes lack some keys, drawn only when
// set).
func newRemoteDrawing(model string, set func(string) bool, key func(k, label string) fyne.CanvasObject) *remoteDrawing {
	x1 := strings.HasPrefix(strings.ToUpper(model), "X1")
	d := &remoteDrawing{keys: map[string]fyne.CanvasObject{}, numbers: map[string]fyne.CanvasObject{}}
	shown := func(k string) bool {
		b, _ := record.ParseButton(k)
		return !x1 || !b.X2Only() || set(k)
	}
	for _, row := range remoteRows {
		for _, k := range row {
			if k != "" && shown(k) {
				d.keys[k] = key(k, k)
			}
		}
	}
	for _, row := range numberRows {
		for _, k := range row {
			if shown(k) {
				d.numbers[k] = key(k, k)
			}
		}
	}
	d.note = wrapped(i18n.T("Number keys, on the remote's screen"))
	d.note.Alignment, d.note.TextStyle.Italic = fyne.TextAlignCenter, true
	d.ExtendBaseWidget(d)
	return d
}
