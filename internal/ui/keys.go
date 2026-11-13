package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"

	"github.com/brunoga/sofabaton/profile"
)

// remoteRows are the remote's keys as on the X2, top to bottom ("" is
// a gap): power over the screen, the ring around OK, then Back, Home and
// Menu; volume and channels each side of Guide and Mute; playback; the
// color keys; A to C.
var remoteRows = [][]string{
	{"", "Power", ""},
	{"", "Up", ""},
	{"Left", "OK", "Right"},
	{"", "Down", ""},
	{"Back", "Home", "Menu"},
	{"Vol+", "Guide", "Ch+"},
	{"Vol-", "Mute", "Ch-"},
	{"Rewind", "Play", "Forward"},
	{"DVR", "Pause", "Exit"},
	{"Red", "Green", "Yellow", "Blue"},
	{"A", "B", "C"},
}

// numberRows are the number keys, which the remote shows on its screen.
var numberRows = [][]string{
	{"1", "2", "3"},
	{"4", "5", "6"},
	{"7", "8", "9"},
	{"-", "0", "Enter"},
}

// PadWidth is the widest the drawing of the remote grows.
const PadWidth = 360

// keyPad sets what the remote's keys do: a drawing of the remote, where
// tapping a key shows what pressing and holding it runs, and under it the
// keys set, with what they run.
type keyPad struct {
	win fyne.Window
	// picker makes a picker of what a key can run, kept up to date by the
	// editor.
	picker func(selected string) *widget.Select
	// changed is called when a key is set or cleared.
	changed func()
	// keys are the keys set.
	keys    []*keyRow
	buttons map[string]*padKey
	list    *fyne.Container
	box     fyne.CanvasObject
}

type keyRow struct {
	key         string
	short, long *widget.Select
}

// newKeyPad is the pad for keys, on a remote of the hub model (the X1
// remotes lack some keys).
func newKeyPad(win fyne.Window, model string, keys profile.Buttons, picker func(string) *widget.Select, changed func()) *keyPad {
	p := &keyPad{win: win, picker: picker, changed: changed, buttons: map[string]*padKey{}, list: container.NewVBox()}
	for _, k := range keyNames(keys) {
		if isRemoteKey(k) {
			p.keys = append(p.keys, &keyRow{key: k, short: picker(keys[k].Short), long: picker(keys[k].Long)})
		}
	}
	drawing := drawRemote(model, func(k string) bool { return p.row(k) != nil }, func(k, label string) fyne.CanvasObject {
		p.buttons[k] = newPadKey(k, label, func() {
			tapFeedback()
			p.open(k)
		})
		return p.buttons[k]
	})
	p.box = container.New(&centerLayout{width: PadWidth * Scale()}, container.NewVBox(drawing, p.list))
	p.refresh()
	return p
}

// drawRemote draws the remote of the hub model with the keys key makes,
// labelled label (the X1 remotes lack some keys, drawn only when set).
func drawRemote(model string, set func(string) bool, key func(k, label string) fyne.CanvasObject) fyne.CanvasObject {
	return newRemoteDrawing(model, set, key)
}

// row is the key's row, or nil when it is not set.
func (p *keyPad) row(key string) *keyRow {
	for _, r := range p.keys {
		if r.key == key {
			return r
		}
	}
	return nil
}

// open shows what pressing and holding key runs, to change it.
func (p *keyPad) open(key string) {
	r := p.row(key)
	if r == nil {
		r = &keyRow{key: key, short: p.picker(""), long: p.picker("")}
		p.keys = append(p.keys, r)
		p.changed()
	}
	var d dialog.Dialog
	clear := widget.NewButtonWithIcon(i18n.T("Clear"), theme.DeleteIcon(), func() {
		r.short.ClearSelected()
		r.long.ClearSelected()
		d.Hide()
	})
	done := widget.NewButtonWithIcon(i18n.T("Done"), theme.ConfirmIcon(), func() { d.Hide() })
	done.Importance = widget.HighImportance
	content := container.NewVBox(
		widget.NewForm(widget.NewFormItem(i18n.T("Press"), r.short), widget.NewFormItem(i18n.T("Hold"), r.long)),
		container.NewGridWithColumns(2, clear, done))
	d = dialog.NewCustomWithoutButtons(i18n.T("The %s key", key), content, p.win)
	d.SetOnClosed(func() {
		if pick(r.short) == "" {
			p.remove(r)
		}
		p.refresh()
		p.changed()
	})
	d.Resize(fyne.NewSize(min(PadWidth*Scale(), p.win.Canvas().Size().Width), d.MinSize().Height))
	d.Show()
}

func (p *keyPad) remove(r *keyRow) {
	for i, o := range p.keys {
		if o == r {
			p.keys = append(p.keys[:i], p.keys[i+1:]...)
			return
		}
	}
}

// refresh marks the keys set and lists them, after what they run changed.
func (p *keyPad) refresh() {
	set := profile.Buttons{}
	for _, r := range p.keys {
		if s := pick(r.short); s != "" {
			set[r.key] = profile.Key{Short: s, Long: pick(r.long)}
		}
	}
	for k, b := range p.buttons {
		imp := widget.MediumImportance
		if _, ok := set[k]; ok {
			imp = widget.HighImportance
		}
		if b.Importance != imp {
			b.Importance = imp
			b.Refresh()
		}
	}
	p.list.Objects = nil
	for _, k := range drawingOrder(set) {
		edit := newIconButton(i18n.T("Edit"), theme.DocumentCreateIcon(), func() { p.open(k) })
		p.list.Add(container.NewBorder(nil, nil, nil, edit, wrapped(k+": "+describeKey(set[k]))))
	}
	if len(set) == 0 {
		none := wrapped(i18n.T("No key is set: tap one to set it."))
		none.TextStyle.Italic = true
		p.list.Add(none)
	}
	p.list.Refresh()
}

// drawingOrder is the keys of bs as the drawing has them, row by row.
func drawingOrder(bs profile.Buttons) []string {
	var out []string
	for _, row := range append(remoteRows, numberRows...) {
		for _, k := range row {
			if _, ok := bs[k]; ok {
				out = append(out, k)
			}
		}
	}
	return out
}

// set is the keys set, as the editors save them.
func (p *keyPad) set(into profile.Buttons) {
	for _, r := range p.keys {
		if s := pick(r.short); s != "" {
			into[r.key] = profile.Key{Short: s, Long: pick(r.long)}
		}
	}
}

// centerLayout centers its objects at most width wide.
type centerLayout struct{ width float32 }

func (l *centerLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	w := min(size.Width, l.width)
	for _, o := range objs {
		o.Resize(fyne.NewSize(w, size.Height))
		o.Move(fyne.NewPos((size.Width-w)/2, 0))
	}
}

func (l *centerLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		s = s.Max(o.MinSize())
	}
	return s
}

// keyColors are the color keys' colors.
var keyColors = map[string]color.NRGBA{
	"Red":    {0xE0, 0x4B, 0x4B, 0xFF},
	"Green":  {0x3F, 0xA8, 0x5F, 0xFF},
	"Yellow": {0xE8, 0xB9, 0x2E, 0xFF},
	"Blue":   {0x3B, 0x7D, 0xD8, 0xFF},
}

// tightKeys is the keys' look: less room around their names, so three
// keys across fit a phone.
type tightKeys struct{}

func (tightKeys) current() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }

func (t tightKeys) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameInnerPadding {
		return t.current().Size(n) / 2
	}
	return t.current().Size(n)
}

func (t tightKeys) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return t.current().Color(n, v)
}
func (t tightKeys) Font(s fyne.TextStyle) fyne.Resource     { return t.current().Font(s) }
func (t tightKeys) Icon(n fyne.ThemeIconName) fyne.Resource { return t.current().Icon(n) }
