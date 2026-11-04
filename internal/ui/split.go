package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// SplitWidth is the narrowest window that shows a list beside its detail;
// narrower ones show one at a time, the app's bar showing the detail's
// title and Back (onChange).
const SplitWidth = 760

// ListWidth is the list's width beside a detail.
const ListWidth = 300

// listDetail shows a list and the detail of what is selected in it: side
// by side on wide windows, one at a time on narrow ones.
type listDetail struct {
	widget.BaseWidget
	list fyne.CanvasObject
	// title is the detail's, which the app's bar shows on narrow windows.
	title string
	body  *fyne.Container
	rule  *canvas.Rectangle

	// placeholder fills the detail when nothing is picked, on wide
	// windows.
	placeholder fyne.CanvasObject

	showing bool // a detail is shown
	narrow  bool
	// onBack is called when Back leaves the detail, on narrow windows;
	// onChange when the detail comes or goes alone (detailAlone).
	onBack, onChange func()
	alone            bool
}

// detailAlone reports whether the detail is shown alone: on a narrow
// window, in the list's place.
func (s *listDetail) detailAlone() bool { return s.narrow && s.showing }

// changed calls onChange if the detail came or went alone.
func (s *listDetail) changed() {
	if a := s.detailAlone(); a != s.alone {
		s.alone = a
		if s.onChange != nil {
			s.onChange()
		}
	}
}

func newListDetail(list fyne.CanvasObject) *listDetail {
	s := &listDetail{list: list}
	s.body = container.NewStack()
	s.rule = canvas.NewRectangle(theme.Color(theme.ColorNameSeparator))
	s.ExtendBaseWidget(s)
	return s
}

// show shows o as the detail, titled title on narrow windows.
func (s *listDetail) show(title string, o fyne.CanvasObject) {
	s.title = title
	s.body.Objects = []fyne.CanvasObject{o}
	s.body.Refresh()
	s.showing = true
	s.alone = false // the title may be new: told again
	s.Refresh()
	s.changed()
}

// clear shows the placeholder: nothing is picked.
func (s *listDetail) clear() {
	s.body.Objects = nil
	if s.placeholder != nil {
		s.body.Objects = []fyne.CanvasObject{s.placeholder}
	}
	s.body.Refresh()
	s.showing = false
	s.Refresh()
	s.changed()
}

// back leaves the detail, on narrow windows.
func (s *listDetail) back() {
	s.showing = false
	s.Refresh()
	s.changed()
	if s.onBack != nil {
		s.onBack()
	}
}

func (s *listDetail) CreateRenderer() fyne.WidgetRenderer {
	return &listDetailRenderer{s: s}
}

type listDetailRenderer struct{ s *listDetail }

func (r *listDetailRenderer) Layout(size fyne.Size) {
	s := r.s
	s.narrow = size.Width < SplitWidth
	defer s.changed()
	if s.narrow {
		s.rule.Hide()
		if s.showing {
			s.list.Hide()
			s.body.Show()
		} else {
			s.list.Show()
			s.body.Hide()
		}
		s.list.Resize(size)
		s.list.Move(fyne.NewPos(0, 0))
		s.body.Resize(size)
		s.body.Move(fyne.NewPos(0, 0))
		return
	}
	s.list.Show()
	s.body.Show()
	s.rule.Show()
	pad := theme.Padding()
	lw := ListWidth * Scale()
	s.list.Resize(fyne.NewSize(lw, size.Height))
	s.list.Move(fyne.NewPos(0, 0))
	s.rule.Resize(fyne.NewSize(1, size.Height))
	s.rule.Move(fyne.NewPos(lw+pad, 0))
	x := lw + 2*pad + 1
	s.body.Resize(fyne.NewSize(size.Width-x, size.Height))
	s.body.Move(fyne.NewPos(x, 0))
}

func (r *listDetailRenderer) MinSize() fyne.Size {
	// Narrow is the smallest it gets: one at a time.
	return r.s.list.MinSize().Max(r.s.body.MinSize())
}

func (r *listDetailRenderer) Refresh() {
	r.s.rule.FillColor = theme.Color(theme.ColorNameSeparator)
	r.Layout(r.s.Size())
	canvas.Refresh(r.s)
}

func (r *listDetailRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.s.list, r.s.rule, r.s.body}
}

func (r *listDetailRenderer) Destroy() {}
