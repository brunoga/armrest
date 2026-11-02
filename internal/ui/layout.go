package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
	return container.New(columnLayout{}, o)
}

type columnLayout struct{}

func (columnLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	w := min(size.Width, ReadableWidth)
	for _, o := range objs {
		o.Resize(fyne.NewSize(w, size.Height))
		o.Move(fyne.NewPos((size.Width-w)/2, 0))
	}
}

func (columnLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		s = s.Max(o.MinSize())
	}
	return s
}
