package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// wrapped is a label that wraps at word boundaries, so it never makes its
// screen wider than a phone.
func wrapped(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// ReadableWidth is the widest a column of text and forms grows: wider
// lines are hard to read, and wide entries hard to scan.
const ReadableWidth = 720

// column centers o at most ReadableWidth wide, using the full width of
// narrower windows.
func column(o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(&centerLayout{width: ReadableWidth * Scale()}, o)
}

// gutter is the margin beside what scrolls: as wide as the scroll bar,
// which grows when the pointer is over it on desktop, so it covers no
// button.
func gutter() float32 { return theme.Size(theme.SizeNameScrollBar) }

// scrolled is o in a column, scrolling, with a gutter each side.
func scrolled(o fyne.CanvasObject) *container.Scroll {
	p, g := theme.Padding(), gutter()
	return container.NewVScroll(column(container.New(layout.NewCustomPaddedLayout(p, p, g, g), o)))
}

// listRow is a list's row, with o clear of the scroll bar.
func listRow(o fyne.CanvasObject) *fyne.Container {
	return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Padding(), gutter()), o)
}
