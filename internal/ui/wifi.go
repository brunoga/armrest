package ui

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/govee"
	"github.com/brunoga/sofabaton/lifx"
	"github.com/brunoga/sofabaton/profile"
	"github.com/brunoga/sofabaton/roku"
	"github.com/brunoga/sofabaton/sonos"
)

// wifiFindTime is how long to look for Rokus and speakers.
var wifiFindTime = 3 * time.Second

// showServices offers the network services the hub has integrations of.
func (a *App) showServices() {
	box := container.NewVBox()
	for _, sv := range []struct {
		name, what string
		show       func()
	}{
		{"Philips Hue", i18n.T("Rooms and lights, through your Hue bridge."), a.showHue},
		{"Roku", i18n.T("Its remote's keys and its channels."), a.showRoku},
		{"Sonos", i18n.T("Play, pause, mute and volume of a speaker."), a.showSonos},
		{"Govee", i18n.T("Lights, through Govee's cloud."), a.showGovee},
		{"LIFX", i18n.T("Lights, through LIFX's cloud."), a.showLIFX},
	} {
		b := widget.NewButton(sv.name, sv.show)
		b.Alignment = widget.ButtonAlignLeading
		box.Add(container.NewVBox(b, wrapped(sv.what)))
	}
	a.screen(i18n.T("Services"), a.showAddDevice, scrolled(container.NewVBox(
		wrapped(i18n.T("The hub sends these the commands made here over your network.")), box)))
}

// wifiScreen is a screen that adds a device of a network service, in
// steps that each replace its content.
type wifiScreen struct {
	app  *App
	body *fyne.Container
}

func (a *App) wifiScreen(title string) *wifiScreen {
	s := &wifiScreen{app: a, body: container.NewVBox()}
	a.screen(title, a.showServices, scrolled(s.body))
	return s
}

func (s *wifiScreen) show(objs ...fyne.CanvasObject) {
	s.body.Objects = objs
	s.body.Refresh()
}

func (s *wifiScreen) busy(text string) {
	p := newPulse()
	p.Start()
	s.show(wrapped(text), p)
}

// failed says why a step failed, with a way to try again and one back.
func (s *wifiScreen) failed(why string, again func(), back fyne.CanvasObject) {
	msg := wrapped(why)
	msg.Importance = widget.DangerImportance
	s.show(msg, container.NewHBox(widget.NewButtonWithIcon(i18n.T("Try again"), theme.ViewRefreshIcon(), again), back))
}

// added puts the device in the workspace and shows it.
func (s *wifiScreen) added(d *profile.Device, problem *widget.Label) {
	a := s.app
	d.Name = a.work.FreeName(d.Name)
	if err := a.work.AddDevice(d); err != nil {
		problem.SetText(err.Error())
		problem.Show()
		return
	}
	b := a.showBrowse(a.hub)
	b.selectDevice(d.Name)
	b.setStatus(false, i18n.T("Added %s.", d.Name))
}

// found is a device found on the network: its address and what to call
// it.
type found struct {
	addr netip.Addr
	name string
}

// find looks for what to add, then offers what it found and an address
// to type; use is given the one picked.
func (s *wifiScreen) find(looking, intro, none, example string, look func(context.Context) ([]found, error), use func(netip.Addr)) {
	s.busy(looking)
	a := s.app
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), wifiFindTime)
		got, err := look(ctx)
		cancel()
		fyne.Do(func() {
			objs := []fyne.CanvasObject{wrapped(intro)}
			switch {
			case len(got) > 0:
				objs = append(objs, wrapped(i18n.N("Found one:", "Found %d:", len(got), len(got))))
				for _, f := range got {
					addr := f.addr
					label := addr.String()
					if f.name != "" {
						label = fmt.Sprintf("%s (%s)", f.name, addr)
					}
					objs = append(objs, widget.NewButtonWithIcon(label, theme.ComputerIcon(), func() { use(addr) }))
				}
			case err != nil:
				objs = append(objs, wrapped(i18n.T("Looking on the network failed: %v", err)))
			default:
				objs = append(objs, wrapped(none))
			}
			ip := widget.NewEntry()
			ip.SetPlaceHolder(example)
			problem := wrapped("")
			problem.Importance = widget.DangerImportance
			problem.Hide()
			useIt := widget.NewButton(i18n.T("Use this address"), func() {
				addr, err := netip.ParseAddr(strings.TrimSpace(ip.Text))
				if err != nil || !addr.Is4() {
					problem.SetText(i18n.T("Type its IPv4 address, such as %s.", example))
					problem.Show()
					return
				}
				use(addr)
			})
			again := widget.NewButtonWithIcon(i18n.T("Look again"), theme.ViewRefreshIcon(), func() { s.find(looking, intro, none, example, look, use) })
			objs = append(objs, widget.NewCard(i18n.T("By its address"), "", container.NewVBox(
				container.NewBorder(nil, nil, nil, useIt, ip), problem)), container.NewHBox(again))
			s.show(objs...)
		})
	})
}

// nameEntry is an entry for the new device's name.
func nameEntry(name string) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(name)
	return e
}

// addButton is the screen's last step's button.
func addButton(tapped func()) *widget.Button {
	b := widget.NewButtonWithIcon(i18n.T("Add the device"), theme.ConfirmIcon(), tapped)
	b.Importance = widget.HighImportance
	return b
}

// reviewNote says that nothing is written yet; key adds that the device
// holds one.
func reviewNote(key bool) *widget.Label {
	text := i18n.T("Nothing is written to the hub until you review and apply your changes.")
	if key {
		text += " " + i18n.T("The device holds your account's key, which the hub needs to send its commands.")
	}
	n := wrapped(text)
	n.TextStyle.Italic = true
	return n
}

func (a *App) showRoku() {
	s := a.wifiScreen("Roku")
	var start func()
	start = func() {
		s.find(i18n.T("Looking for Rokus on your network…"),
			i18n.T("The hub sends your Roku the commands made here: its remote's keys, and one to start each of its channels."),
			i18n.T("No Roku answered. Type its address: the Roku shows it in Settings, Network, About."),
			"192.168.1.50",
			func(ctx context.Context) ([]found, error) {
				ps, err := a.findRokus(ctx)
				var out []found
				for _, p := range ps {
					out = append(out, found{p.Addr, p.Name})
				}
				return out, err
			},
			func(addr netip.Addr) { s.roku(addr, start) })
	}
	start()
}

// roku reads the Roku's name and channels, then offers them.
func (s *wifiScreen) roku(addr netip.Addr, back func()) {
	a := s.app
	s.busy(i18n.T("Reading the Roku's channels…"))
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
		defer cancel()
		info, _ := roku.Info(ctx, addr, a.wifiHTTP)
		apps, err := roku.Apps(ctx, addr, a.wifiHTTP)
		fyne.Do(func() {
			if err != nil {
				s.failed(i18n.T("The Roku at %s did not answer: %v", addr, err), func() { s.roku(addr, back) },
					widget.NewButton(i18n.T("Another Roku"), back))
				return
			}
			name := info.Name()
			if name == "" {
				name = "Roku"
			}
			s.rokuChoose(addr, name, apps)
		})
	})
}

func (s *wifiScreen) rokuChoose(addr netip.Addr, name string, apps []roku.App) {
	n := nameEntry(name)
	checks := container.NewVBox()
	for _, app := range apps {
		c := widget.NewCheck(app.Name, nil)
		c.Checked = true
		checks.Add(c)
	}
	picked := func() []roku.App {
		var out []roku.App
		for i, o := range checks.Objects {
			if o.(*widget.Check).Checked {
				out = append(out, apps[i])
			}
		}
		return out
	}
	problem := wrapped("")
	problem.Importance = widget.DangerImportance
	problem.Hide()
	add := addButton(func() {
		if strings.TrimSpace(n.Text) == "" {
			problem.SetText(i18n.T("The device needs a name."))
			problem.Show()
			return
		}
		s.added(roku.Device(strings.TrimSpace(n.Text), addr, picked()), problem)
	})
	objs := []fyne.CanvasObject{
		widget.NewForm(widget.NewFormItem(i18n.T("Name"), n)),
		wrapped(i18n.T("It gets the Roku remote's keys, bound to the remote's, and power on and off.")),
	}
	if len(apps) > 0 {
		objs = append(objs, widget.NewCard(i18n.T("Channels"), "", container.NewVBox(checks, allNone(checks))))
	}
	s.show(append(objs, problem, add, reviewNote(false))...)
}

// allNone is buttons that check or clear every check of box.
func allNone(box *fyne.Container) fyne.CanvasObject {
	all := func(on bool) {
		for _, o := range box.Objects {
			o.(*widget.Check).SetChecked(on)
		}
	}
	return container.NewHBox(
		widget.NewButton(i18n.T("All"), func() { all(true) }),
		widget.NewButton(i18n.T("None"), func() { all(false) }))
}

func (a *App) showSonos() {
	s := a.wifiScreen("Sonos")
	var start func()
	start = func() {
		s.find(i18n.T("Looking for Sonos speakers on your network…"),
			i18n.T("The hub sends your Sonos speaker the commands made here: play and pause, next and previous, mute and volumes."),
			i18n.T("No speaker answered. Type its address: the Sonos app shows it in Settings, System, About My System."),
			"192.168.1.60",
			func(ctx context.Context) ([]found, error) {
				ss, err := a.findSonos(ctx)
				var out []found
				for _, sp := range ss {
					out = append(out, found{sp.Addr, sp.Room})
				}
				return out, err
			},
			func(addr netip.Addr) { s.sonos(addr, start) })
	}
	start()
}

// sonos reads the speaker's room and volume, then offers its device.
func (s *wifiScreen) sonos(addr netip.Addr, back func()) {
	a := s.app
	s.busy(i18n.T("Reading the speaker…"))
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
		defer cancel()
		desc, err := sonos.Describe(ctx, addr, a.wifiHTTP)
		volume, verr := sonos.Volume(ctx, addr, a.wifiHTTP)
		if verr != nil {
			volume = 30 // the app's when it cannot ask
		}
		fyne.Do(func() {
			if err != nil {
				s.failed(i18n.T("The speaker at %s did not answer: %v", addr, err), func() { s.sonos(addr, back) },
					widget.NewButton(i18n.T("Another speaker"), back))
				return
			}
			n := nameEntry(sonos.Speaker{Addr: addr, Room: desc.Room}.Name())
			problem := wrapped("")
			problem.Importance = widget.DangerImportance
			problem.Hide()
			add := addButton(func() {
				if strings.TrimSpace(n.Text) == "" {
					problem.SetText(i18n.T("The device needs a name."))
					problem.Show()
					return
				}
				s.added(sonos.Device(strings.TrimSpace(n.Text), addr, volume), problem)
			})
			s.show(widget.NewForm(widget.NewFormItem(i18n.T("Name"), n)),
				wrapped(i18n.T("It gets play, pause, next and previous; mute and unmute; volumes of 10 to 100, the room's and its group's; and volume up and down, bound to the remote's keys.")),
				problem, add, reviewNote(false))
		})
	})
}

// lightPicker offers lights (or groups of them) and the commands to make
// for each, counting the commands against what a device takes.
type lightPicker struct {
	checks []*widget.Check
	onOff  *widget.Check
	levels []*widget.Check
	pcts   []int
	count  *widget.Label
	add    *widget.Button
	// commands is how many commands the choice makes.
	commands func() int
}

func newLightPicker(pcts []int, add func()) *lightPicker {
	p := &lightPicker{pcts: pcts, count: wrapped(""), add: addButton(add)}
	p.onOff = widget.NewCheck(i18n.T("On and off"), func(bool) { p.recount() })
	p.onOff.Checked = true
	for _, pct := range pcts {
		l := widget.NewCheck(i18n.T("%d%% brightness", pct), func(bool) { p.recount() })
		l.Checked = true
		p.levels = append(p.levels, l)
	}
	return p
}

func (p *lightPicker) check(name string, on bool) *widget.Check {
	c := widget.NewCheck(name, func(bool) { p.recount() })
	c.Checked = on
	p.checks = append(p.checks, c)
	return c
}

func (p *lightPicker) kinds() fyne.CanvasObject {
	box := container.NewVBox(p.onOff)
	for _, l := range p.levels {
		box.Add(l)
	}
	return widget.NewCard(i18n.T("Commands for each"), "", box)
}

func (p *lightPicker) brightness() []int {
	var out []int
	for i, l := range p.levels {
		if l.Checked {
			out = append(out, p.pcts[i])
		}
	}
	return out
}

func (p *lightPicker) picked() []bool {
	out := make([]bool, len(p.checks))
	for i, c := range p.checks {
		out[i] = c.Checked
	}
	return out
}

func (p *lightPicker) recount() {
	if p.commands == nil {
		return
	}
	switch n := p.commands(); {
	case n == 0:
		p.count.SetText(i18n.T("Pick lights, and the commands to make for them."))
		p.add.Disable()
	case n > profile.MaxCommands:
		p.count.SetText(i18n.T("%d commands: a device takes at most %d. Pick fewer, then add another device for the rest.", n, profile.MaxCommands))
		p.add.Disable()
	default:
		p.count.SetText(i18n.N("%d command.", "%d commands.", n, n))
		p.add.Enable()
	}
}

// keyStep asks for an account's key, prefilled with the one a device of
// the hub holds, then reads with it.
func (s *wifiScreen) keyStep(intro, label, where, key string, read func(key string)) {
	k := widget.NewPasswordEntry()
	k.SetText(key)
	problem := wrapped("")
	problem.Importance = widget.DangerImportance
	problem.Hide()
	next := widget.NewButtonWithIcon(i18n.T("Read my lights"), theme.NavigateNextIcon(), func() {
		if strings.TrimSpace(k.Text) == "" {
			problem.SetText(i18n.T("Type the key first."))
			problem.Show()
			return
		}
		read(strings.TrimSpace(k.Text))
	})
	next.Importance = widget.HighImportance
	s.show(wrapped(intro), widget.NewForm(widget.NewFormItem(label, k)), wrapped(where), problem, container.NewHBox(next))
}

func (a *App) showGovee() {
	s := a.wifiScreen("Govee")
	var ask func()
	ask = func() {
		s.keyStep(i18n.T("The hub sends Govee's cloud the commands made here, to switch and dim your Govee lights."),
			i18n.T("API key"),
			i18n.T("Govee Home gives the key: Profile, Settings, Apply for API Key; it comes by email."),
			a.work.HeaderValue(govee.API, "Govee-API-Key"),
			func(key string) { s.govee(key, ask) })
	}
	ask()
}

func (s *wifiScreen) govee(key string, back func()) {
	a := s.app
	s.busy(i18n.T("Reading your lights…"))
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
		defer cancel()
		lights, err := govee.Lights(ctx, key, a.wifiHTTP)
		fyne.Do(func() {
			switch {
			case errors.Is(err, govee.ErrKey):
				s.failed(i18n.T("Govee did not take the key."), back, widget.NewButton(i18n.T("Another key"), back))
				return
			case err != nil:
				s.failed(i18n.T("Govee did not answer: %v", err), func() { s.govee(key, back) }, widget.NewButton(i18n.T("Another key"), back))
				return
			case len(lights) == 0:
				s.failed(i18n.T("The account has no lights to control."), func() { s.govee(key, back) }, widget.NewButton(i18n.T("Another key"), back))
				return
			}
			n := nameEntry("Govee")
			problem := wrapped("")
			problem.Importance = widget.DangerImportance
			problem.Hide()
			var p *lightPicker
			device := func() *profile.Device {
				var picked []govee.Light
				for i, on := range p.picked() {
					if on {
						picked = append(picked, lights[i])
					}
				}
				return govee.Device(strings.TrimSpace(n.Text), key, picked, govee.Options{OnOff: p.onOff.Checked, Brightness: p.brightness()})
			}
			p = newLightPicker(govee.Brightness, func() {
				if strings.TrimSpace(n.Text) == "" {
					problem.SetText(i18n.T("The device needs a name."))
					problem.Show()
					return
				}
				s.added(device(), problem)
			})
			box := container.NewVBox()
			for _, l := range lights {
				box.Add(p.check(l.Name, true))
			}
			p.commands = func() int { return len(device().Commands) }
			s.show(widget.NewForm(widget.NewFormItem(i18n.T("Name"), n)),
				widget.NewCard(i18n.T("Lights"), "", container.NewVBox(box, allNone(box))),
				p.kinds(), p.count, problem, p.add, reviewNote(true))
			p.recount()
		})
	})
}

func (a *App) showLIFX() {
	s := a.wifiScreen("LIFX")
	var ask func()
	ask = func() {
		s.keyStep(i18n.T("The hub sends LIFX's cloud the commands made here, to switch and dim your LIFX lights."),
			i18n.T("Token"),
			i18n.T("LIFX gives the token: cloud.lifx.com, Settings, Personal access tokens, Generate New Token."),
			strings.TrimPrefix(a.work.HeaderValue(lifx.API, "Authorization"), "Bearer "),
			func(token string) { s.lifx(token, ask) })
	}
	ask()
}

func (s *wifiScreen) lifx(token string, back func()) {
	a := s.app
	s.busy(i18n.T("Reading your lights…"))
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
		defer cancel()
		groups, lights, err := lifx.Targets(ctx, token, a.wifiHTTP)
		fyne.Do(func() {
			switch {
			case errors.Is(err, lifx.ErrToken):
				s.failed(i18n.T("LIFX did not take the token."), back, widget.NewButton(i18n.T("Another token"), back))
				return
			case err != nil:
				s.failed(i18n.T("LIFX did not answer: %v", err), func() { s.lifx(token, back) }, widget.NewButton(i18n.T("Another token"), back))
				return
			}
			n := nameEntry("LIFX")
			problem := wrapped("")
			problem.Importance = widget.DangerImportance
			problem.Hide()
			all := lifx.All
			all.Name = i18n.T("All lights")
			targets := append(append([]lifx.Target{all}, groups...), lights...)
			var p *lightPicker
			device := func() *profile.Device {
				var picked []lifx.Target
				for i, on := range p.picked() {
					if on {
						picked = append(picked, targets[i])
					}
				}
				return lifx.Device(strings.TrimSpace(n.Text), token, picked, lifx.Options{OnOff: p.onOff.Checked, Brightness: p.brightness()})
			}
			p = newLightPicker(lifx.Brightness, func() {
				if strings.TrimSpace(n.Text) == "" {
					problem.SetText(i18n.T("The device needs a name."))
					problem.Show()
					return
				}
				s.added(device(), problem)
			})
			allBox, groupBox, lightBox := container.NewVBox(p.check(all.Name, true)), container.NewVBox(), container.NewVBox()
			for _, g := range groups {
				groupBox.Add(p.check(g.Name, false))
			}
			for _, l := range lights {
				lightBox.Add(p.check(l.Name, false))
			}
			p.commands = func() int { return len(device().Commands) }
			objs := []fyne.CanvasObject{widget.NewForm(widget.NewFormItem(i18n.T("Name"), n)), allBox}
			if len(groups) > 0 {
				objs = append(objs, widget.NewCard(i18n.T("Groups"), "", container.NewVBox(groupBox, allNone(groupBox))))
			}
			if len(lights) > 0 {
				objs = append(objs, widget.NewCard(i18n.T("Lights"), "", container.NewVBox(lightBox, allNone(lightBox))))
			}
			s.show(append(objs, p.kinds(), p.count, problem, p.add, reviewNote(true))...)
			p.recount()
		})
	})
}
