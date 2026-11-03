package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/sofabaton/profile"
	"github.com/brunoga/sofabaton/record"
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
	buttons map[string]*widget.Button
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
	p := &keyPad{win: win, picker: picker, changed: changed, buttons: map[string]*widget.Button{}, list: container.NewVBox()}
	for _, k := range keyNames(keys) {
		if isRemoteKey(k) {
			p.keys = append(p.keys, &keyRow{key: k, short: picker(keys[k].Short), long: picker(keys[k].Long)})
		}
	}
	x1 := strings.HasPrefix(strings.ToUpper(model), "X1")
	grids := func(rows [][]string) *fyne.Container {
		out := container.NewVBox()
		for _, row := range rows {
			grid := container.NewGridWithColumns(len(row))
			shown := false
			for _, k := range row {
				b, _ := record.ParseButton(k)
				if k == "" || x1 && b.X2Only() && p.row(k) == nil {
					grid.Add(layout.NewSpacer())
					continue
				}
				p.buttons[k] = widget.NewButton(k, func() { p.open(k) })
				if c, ok := keyColors[k]; ok {
					grid.Add(container.NewThemeOverride(p.buttons[k], colorKey{c}))
				} else {
					grid.Add(p.buttons[k])
				}
				shown = true
			}
			if shown {
				out.Add(grid)
			}
		}
		return out
	}
	rows := grids(remoteRows)
	if numbers := grids(numberRows); len(numbers.Objects) > 0 {
		rows.Add(widget.NewLabelWithStyle("Number keys, on the remote's screen", fyne.TextAlignCenter, fyne.TextStyle{Italic: true}))
		rows.Add(numbers)
	}
	p.box = container.New(&centerLayout{width: PadWidth * Scale()}, container.NewVBox(rows, p.list))
	p.refresh()
	return p
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
	clear := widget.NewButtonWithIcon("Clear", theme.DeleteIcon(), func() {
		r.short.ClearSelected()
		r.long.ClearSelected()
		d.Hide()
	})
	done := widget.NewButtonWithIcon("Done", theme.ConfirmIcon(), func() { d.Hide() })
	done.Importance = widget.HighImportance
	content := container.NewVBox(
		widget.NewForm(widget.NewFormItem("Press", r.short), widget.NewFormItem("Hold", r.long)),
		container.NewGridWithColumns(2, clear, done))
	d = dialog.NewCustomWithoutButtons(fmt.Sprintf("The %s key", key), content, p.win)
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
		edit := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), func() { p.open(k) })
		p.list.Add(container.NewBorder(nil, nil, nil, edit, wrapped(k+": "+describeKey(set[k]))))
	}
	if len(set) == 0 {
		none := wrapped("No key is set: tap one to set it.")
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

// colorKey is a color key's look: tinted with its color, filled with it
// once set.
type colorKey struct{ c color.NRGBA }

func (k colorKey) current() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }

func (k colorKey) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNameButton:
		c := k.c
		c.A = 0x40
		return c
	case theme.ColorNamePrimary:
		return k.c
	case theme.ColorNameForegroundOnPrimary:
		return color.White
	}
	return k.current().Color(n, v)
}

func (k colorKey) Font(s fyne.TextStyle) fyne.Resource     { return k.current().Font(s) }
func (k colorKey) Icon(n fyne.ThemeIconName) fyne.Resource { return k.current().Icon(n) }
func (k colorKey) Size(n fyne.ThemeSizeName) float32       { return k.current().Size(n) }
