package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// pulse is a thin line sweeping across, over and over: it shows a long
// step is alive while a progress bar stands still.
type pulse struct {
	widget.BaseWidget
	line *canvas.Rectangle
	anim *fyne.Animation
	// at is how far across the line is, 0 to 1.
	at float32
}

func newPulse() *pulse {
	p := &pulse{line: canvas.NewRectangle(color.Transparent)}
	p.anim = fyne.NewAnimation(1400*time.Millisecond, p.sweep)
	p.anim.Curve = fyne.AnimationEaseInOut
	p.anim.RepeatCount = fyne.AnimationRepeatForever
	p.ExtendBaseWidget(p)
	return p
}

// Start starts the sweep.
func (p *pulse) Start() { p.anim.Start() }

// Stop stops it and hides the line.
func (p *pulse) Stop() {
	p.anim.Stop()
	p.Hide()
}

// sweep moves the line a fraction f of its way across.
func (p *pulse) sweep(f float32) {
	p.at = f
	p.place(p.Size())
}

// place places the line at p.at across size: a quarter of the width,
// coming in on the left and going out on the right.
func (p *pulse) place(size fyne.Size) {
	seg := size.Width / 4
	x := -seg + p.at*(size.Width+seg)
	left, right := max(x, 0), min(x+seg, size.Width)
	p.line.Move(fyne.NewPos(left, 0))
	p.line.Resize(fyne.NewSize(max(right-left, 0), size.Height))
}

func (p *pulse) CreateRenderer() fyne.WidgetRenderer {
	r := &pulseRenderer{p: p}
	r.Refresh()
	return r
}

type pulseRenderer struct{ p *pulse }

func (r *pulseRenderer) Layout(size fyne.Size) { r.p.place(size) }

func (r *pulseRenderer) MinSize() fyne.Size { return fyne.NewSize(0, 2) }

// Refresh takes the theme's text color, faint: white on dark, dark on
// light.
func (r *pulseRenderer) Refresh() {
	c := color.NRGBAModel.Convert(theme.Color(theme.ColorNameForeground)).(color.NRGBA)
	c.A = 0x70
	r.p.line.FillColor = c
	r.p.line.Refresh()
}

func (r *pulseRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.p.line} }

func (r *pulseRenderer) Destroy() { r.p.anim.Stop() }
