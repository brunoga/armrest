package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// gripIcon is the drag handle's dots (Material's drag indicator, Apache
// 2.0).
var gripIcon = theme.NewThemedResource(fyne.NewStaticResource("grip.svg", []byte(
	`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="#000000" d="M11 18c0 1.1-.9 2-2 2s-2-.9-2-2 .9-2 2-2 2 .9 2 2zm-2-8c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0-6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm6 4c1.1 0 2-.9 2-2s-.9-2-2-2-2 .9-2 2 .9 2 2 2zm0 2c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2zm0 6c-1.1 0-2 .9-2 2s.9 2 2 2 2-.9 2-2-.9-2-2-2z"/></svg>`)))

// dragHint says how rows with a handle are reordered.
const dragHint = "Drag a row by its dots to move it."

// dragHandle moves its row up and down its box by dragging, with a mouse
// or a finger: the row follows, and the others make room as it passes
// them.
type dragHandle struct {
	widget.BaseWidget
	// row is the handle's row, one of box's objects.
	row fyne.CanvasObject
	box *fyne.Container
	// swap swaps rows i and j of what box shows, which are next to each
	// other; moved is called once the row is dropped.
	swap  func(i, j int)
	moved func()

	// offset is how far the row is from its place in box.
	offset float32
	// dragging is set from the drag's start to its end.
	dragging bool
}

func newDragHandle(box *fyne.Container, swap func(i, j int), moved func()) *dragHandle {
	h := &dragHandle{box: box, swap: swap, moved: moved}
	h.ExtendBaseWidget(h)
	return h
}

func (h *dragHandle) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewPadded(widget.NewIcon(gripIcon)))
}

// Cursor shows the row moves up and down.
func (h *dragHandle) Cursor() desktop.Cursor { return desktop.VResizeCursor }

func (h *dragHandle) index() int {
	for i, o := range h.box.Objects {
		if o == h.row {
			return i
		}
	}
	return -1
}

// Dragged moves the row with the pointer, past the rows whose middle it
// passes.
func (h *dragHandle) Dragged(e *fyne.DragEvent) {
	if !h.dragging {
		h.dragging = true
		tapFeedback() // picked up
	}
	h.offset += e.Dragged.DY
	i := h.index()
	if i < 0 {
		return
	}
	pad := theme.Padding()
	objs := h.box.Objects
	for {
		switch {
		case i > 0 && h.offset < -objs[i-1].Size().Height/2:
			h.offset += objs[i-1].Size().Height + pad
			h.exchange(i, i-1)
			i--
		case i < len(objs)-1 && h.offset > objs[i+1].Size().Height/2:
			h.offset -= objs[i+1].Size().Height + pad
			h.exchange(i, i+1)
			i++
		default:
			h.box.Refresh()
			h.row.Move(h.row.Position().AddXY(0, h.offset))
			return
		}
	}
}

// exchange swaps rows i and j, in box and in what it shows.
func (h *dragHandle) exchange(i, j int) {
	h.box.Objects[i], h.box.Objects[j] = h.box.Objects[j], h.box.Objects[i]
	h.swap(i, j)
}

// DragEnd drops the row in its place.
func (h *dragHandle) DragEnd() {
	h.offset, h.dragging = 0, false
	h.box.Refresh()
	if h.moved != nil {
		h.moved()
	}
}
