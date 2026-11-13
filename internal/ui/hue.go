package ui

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/hue"
)

// How long to look for bridges, to wait for the link button (good for
// 30 seconds once pressed), and how often to ask the bridge.
var (
	hueFindTime = 3 * time.Second
	hueLinkWait = 60 * time.Second
	hueLinkPoll = 2 * time.Second
)

// hueScreen adds a Philips Hue bridge, as the app does: the bridge, its
// link button, then the rooms and lights to make commands for.
type hueScreen struct {
	app  *App
	body *fyne.Container
	// stop ends the wait for the link button, on leaving.
	stop context.CancelFunc

	client  *hue.Client
	targets []*hueTarget
	onOff   *widget.Check
	levels  []*widget.Check
	count   *widget.Label
	add     *widget.Button
}

type hueTarget struct {
	t     hue.Target
	check *widget.Check
}

func (a *App) showHue() {
	s := &hueScreen{app: a, body: container.NewVBox()}
	a.screen(i18n.T("Philips Hue"), func() { s.leave(); a.showServices() }, scrolled(s.body))
	s.find()
}

func (s *hueScreen) leave() {
	if s.stop != nil {
		s.stop()
	}
}

// show replaces the screen's content.
func (s *hueScreen) show(objs ...fyne.CanvasObject) {
	s.body.Objects = objs
	s.body.Refresh()
}

// busy shows text with a sign of work going on.
func (s *hueScreen) busy(text string) {
	p := newPulse()
	p.Start()
	s.show(wrapped(text), p)
}

// find looks for bridges, then offers them and an address to type.
func (s *hueScreen) find() {
	s.busy(i18n.T("Looking for Hue bridges on your network…"))
	a := s.app
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), hueFindTime)
		found, err := a.hueBridges(ctx)
		cancel()
		fyne.Do(func() { s.pick(found, err) })
	})
}

func (s *hueScreen) pick(found []hue.Bridge, err error) {
	intro := wrapped(i18n.T("The hub sends your Hue bridge the commands made here, to switch and dim its rooms and lights."))
	var objs []fyne.CanvasObject
	objs = append(objs, intro)
	switch {
	case len(found) > 0:
		objs = append(objs, wrapped(i18n.N("Found a bridge:", "Found %d bridges:", len(found), len(found))))
		for _, b := range found {
			addr := b.Addr
			objs = append(objs, widget.NewButtonWithIcon(addr.String(), theme.ComputerIcon(), func() { s.link(addr) }))
		}
	case err != nil:
		objs = append(objs, wrapped(i18n.T("Looking for bridges failed: %v", err)))
	default:
		objs = append(objs, wrapped(i18n.T("No bridge answered. Type its address: the Hue app shows it in Settings, Bridges.")))
	}
	ip := widget.NewEntry()
	ip.SetPlaceHolder("192.168.1.183")
	problem := wrapped("")
	problem.Importance = widget.DangerImportance
	problem.Hide()
	use := widget.NewButton(i18n.T("Use this address"), func() {
		addr, err := netip.ParseAddr(strings.TrimSpace(ip.Text))
		if err != nil || !addr.Is4() {
			problem.SetText(i18n.T("Type the bridge's IPv4 address, such as 192.168.1.183."))
			problem.Show()
			return
		}
		s.link(addr)
	})
	again := widget.NewButtonWithIcon(i18n.T("Look again"), theme.ViewRefreshIcon(), s.find)
	objs = append(objs, widget.NewCard(i18n.T("Another bridge"), "", container.NewVBox(
		container.NewBorder(nil, nil, nil, use, ip), problem)), container.NewHBox(again))
	s.show(objs...)
}

// link gets a user of the bridge at addr: the one a device of the hub
// holds for it, if the bridge still knows it, or a new one once its
// link button is pressed.
func (s *hueScreen) link(addr netip.Addr) {
	a := s.app
	if user := a.work.HueUser(addr.String()); user != "" {
		s.busy(i18n.T("Reading the bridge's rooms and lights…"))
		c := &hue.Client{Addr: addr, User: user, HTTP: a.hueHTTP}
		a.async(func() {
			ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
			targets, err := s.read(ctx, c)
			cancel()
			fyne.Do(func() {
				if err != nil {
					s.press(addr) // the bridge forgot the user
					return
				}
				s.choose(c, targets)
			})
		})
		return
	}
	s.press(addr)
}

// press asks for the link button and waits for it.
func (s *hueScreen) press(addr netip.Addr) {
	a := s.app
	ctx, cancel := context.WithTimeout(context.Background(), hueLinkWait)
	s.stop = cancel
	p := newPulse()
	p.Start()
	cancelBtn := widget.NewButtonWithIcon(i18n.T("Cancel"), theme.CancelIcon(), func() { cancel(); s.find() })
	s.show(
		wrapped(i18n.T("Press the round link button on top of the Hue bridge at %s.", addr)),
		p, container.NewHBox(cancelBtn))
	a.async(func() {
		defer cancel()
		var c *hue.Client
		var err error
		for {
			if c, err = hue.Link(ctx, addr, appInstance(), a.hueHTTP); !errors.Is(err, hue.ErrLinkButton) {
				break
			}
			select {
			case <-ctx.Done():
				err = ctx.Err()
			case <-time.After(hueLinkPoll):
				continue
			}
			break
		}
		var targets []hue.Target
		if err == nil {
			targets, err = s.read(ctx, c)
		}
		fyne.Do(func() {
			switch {
			case errors.Is(err, context.Canceled):
				// Cancelled: the screen moved on.
			case errors.Is(err, context.DeadlineExceeded):
				s.failed(addr, i18n.T("The bridge's link button was not pressed in time."))
			case err != nil:
				s.failed(addr, i18n.T("The bridge did not answer as a Hue bridge: %v", err))
			default:
				s.choose(c, targets)
			}
		})
	})
}

func (s *hueScreen) failed(addr netip.Addr, why string) {
	msg := wrapped(why)
	msg.Importance = widget.DangerImportance
	s.show(msg, container.NewHBox(
		widget.NewButtonWithIcon(i18n.T("Try again"), theme.ViewRefreshIcon(), func() { s.press(addr) }),
		widget.NewButton(i18n.T("Another bridge"), s.find)))
}

// read is the bridge's rooms and zones, then its lights.
func (s *hueScreen) read(ctx context.Context, c *hue.Client) ([]hue.Target, error) {
	rooms, err := c.Rooms(ctx)
	if err != nil {
		return nil, err
	}
	lights, err := c.Lights(ctx)
	return append(rooms, lights...), err
}

// choose offers the rooms, lights and kinds of command, counting the
// commands against what a bridge's device takes.
func (s *hueScreen) choose(c *hue.Client, targets []hue.Target) {
	s.client, s.targets = c, nil
	rooms, lights := container.NewVBox(), container.NewVBox()
	for _, t := range targets {
		ht := &hueTarget{t: t, check: widget.NewCheck(t.Name, func(bool) { s.recount() })}
		ht.check.Checked = t.Group // rooms to start with: lights are many
		s.targets = append(s.targets, ht)
		if t.Group {
			rooms.Add(ht.check)
		} else {
			lights.Add(ht.check)
		}
	}
	s.onOff = widget.NewCheck(i18n.T("On and off"), func(bool) { s.recount() })
	s.onOff.Checked = true
	kinds := container.NewVBox(s.onOff)
	s.levels = nil
	for _, pct := range hue.Brightness {
		l := widget.NewCheck(i18n.T("%d%% brightness", pct), func(bool) { s.recount() })
		l.Checked = true
		s.levels = append(s.levels, l)
		kinds.Add(l)
	}
	s.count = wrapped("")
	s.add = widget.NewButtonWithIcon(i18n.T("Add the device"), theme.ConfirmIcon(), s.create)
	s.add.Importance = widget.HighImportance
	note := wrapped(i18n.T("Nothing is written to the hub until you review and apply your changes. The device holds a key to your bridge, which the hub needs to send it commands."))
	note.TextStyle.Italic = true
	s.show(
		widget.NewCard(i18n.T("Rooms and zones"), "", container.NewVBox(rooms, allNone(rooms))),
		widget.NewCard(i18n.T("Lights"), "", container.NewVBox(lights, allNone(lights))),
		widget.NewCard(i18n.T("Commands for each"), "", kinds),
		s.count, s.add, note)
	s.recount()
}

func (s *hueScreen) options() hue.Options {
	o := hue.Options{OnOff: s.onOff.Checked}
	for i, l := range s.levels {
		if l.Checked {
			o.Brightness = append(o.Brightness, hue.Brightness[i])
		}
	}
	return o
}

func (s *hueScreen) picked() []hue.Target {
	var out []hue.Target
	for _, t := range s.targets {
		if t.check.Checked {
			out = append(out, t.t)
		}
	}
	return out
}

// recount says how many commands the choice makes, and whether a bridge's
// device takes them.
func (s *hueScreen) recount() {
	if s.count == nil {
		return
	}
	n := len(hue.Commands(s.picked(), s.options()))
	switch {
	case n == 0:
		s.count.SetText(i18n.T("Pick rooms or lights, and the commands to make for them."))
		s.add.Disable()
	case n > hue.MaxCommands:
		s.count.SetText(i18n.T("%d commands: a bridge's device takes at most %d. Pick fewer, then add the bridge again for the rest.", n, hue.MaxCommands))
		s.add.Disable()
	default:
		s.count.SetText(i18n.N("%d command.", "%d commands.", n, n))
		s.add.Enable()
	}
}

// create adds the bridge's device to the workspace.
func (s *hueScreen) create() {
	a := s.app
	d := a.work.HueDevice(s.client, hue.Commands(s.picked(), s.options()))
	if err := a.work.AddDevice(d); err != nil {
		s.count.SetText(err.Error())
		return
	}
	b := a.showBrowse(a.hub)
	b.selectDevice(d.Name)
	b.setStatus(false, i18n.T("Added %s.", d.Name))
}

// appInstance names this copy of the app to the bridge, which lists the
// users it gave by it: the app's name and, after #, the device's, of 19
// characters at most.
func appInstance() string {
	device := "phone"
	if !fyne.CurrentDevice().IsMobile() {
		if h, err := os.Hostname(); err == nil && h != "" {
			device = h
		}
	}
	if len(device) > 19 {
		device = device[:19]
	}
	return "armrest#" + device
}
