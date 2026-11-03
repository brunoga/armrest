package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// SplitWidth is the narrowest window that shows a list beside its detail;
// narrower ones show one at a time, the detail with a Back button.
const SplitWidth = 760

// ListWidth is the list's width beside a detail.
const ListWidth = 300

// listDetail shows a list and the detail of what is selected in it: side
// by side on wide windows, one at a time on narrow ones.
type listDetail struct {
	widget.BaseWidget
	list   fyne.CanvasObject
	detail *fyne.Container // the detail's bar and content
	bar    *fyne.Container // Back and the detail's title, on narrow windows
	title  *widget.Label
	body   *fyne.Container
	rule   *canvas.Rectangle

	// placeholder fills the detail when nothing is picked, on wide
	// windows.
	placeholder fyne.CanvasObject

	showing bool // a detail is shown
	narrow  bool
	// onBack is called when Back leaves the detail, on narrow windows.
	onBack func()
}

func newListDetail(list fyne.CanvasObject) *listDetail {
	s := &listDetail{list: list}
	s.title = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	s.title.Truncation = fyne.TextTruncateEllipsis
	s.bar = container.NewBorder(nil, nil,
		widget.NewButtonWithIcon("Back", theme.NavigateBackIcon(), s.back), nil, s.title)
	s.body = container.NewStack()
	s.detail = container.NewBorder(s.bar, nil, nil, nil, s.body)
	s.rule = canvas.NewRectangle(theme.Color(theme.ColorNameSeparator))
	s.ExtendBaseWidget(s)
	return s
}

// show shows o as the detail, titled title on narrow windows.
func (s *listDetail) show(title string, o fyne.CanvasObject) {
	s.title.SetText(title)
	s.body.Objects = []fyne.CanvasObject{o}
	s.body.Refresh()
	s.showing = true
	s.Refresh()
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
}

// back leaves the detail, on narrow windows.
func (s *listDetail) back() {
	s.showing = false
	s.Refresh()
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
	if s.narrow {
		s.rule.Hide()
		s.bar.Show()
		if s.showing {
			s.list.Hide()
			s.detail.Show()
		} else {
			s.list.Show()
			s.detail.Hide()
		}
		s.list.Resize(size)
		s.list.Move(fyne.NewPos(0, 0))
		s.detail.Resize(size)
		s.detail.Move(fyne.NewPos(0, 0))
		return
	}
	s.bar.Hide()
	s.list.Show()
	s.detail.Show()
	s.rule.Show()
	pad := theme.Padding()
	lw := ListWidth * Scale()
	s.list.Resize(fyne.NewSize(lw, size.Height))
	s.list.Move(fyne.NewPos(0, 0))
	s.rule.Resize(fyne.NewSize(1, size.Height))
	s.rule.Move(fyne.NewPos(lw+pad, 0))
	x := lw + 2*pad + 1
	s.detail.Resize(fyne.NewSize(size.Width-x, size.Height))
	s.detail.Move(fyne.NewPos(x, 0))
}

func (r *listDetailRenderer) MinSize() fyne.Size {
	// Narrow is the smallest it gets: one at a time.
	return r.s.list.MinSize().Max(r.s.detail.MinSize())
}

func (r *listDetailRenderer) Refresh() {
	r.s.rule.FillColor = theme.Color(theme.ColorNameSeparator)
	r.Layout(r.s.Size())
	canvas.Refresh(r.s)
}

func (r *listDetailRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.s.list, r.s.rule, r.s.detail}
}

func (r *listDetailRenderer) Destroy() {}
