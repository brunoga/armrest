package workspace

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/dump"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/hubsim"
	"github.com/brunoga/sofabaton/profile"
	"github.com/brunoga/sofabaton/wire"
)

// Plan is what applying the workspace's edits writes to the hub.
type Plan struct {
	Changes []Change
	// Writes is how many writes it takes, for a progress bar.
	Writes int
}

// Change is what happens to one device or activity.
type Change struct {
	// Title says it in words: "Add device TV".
	Title string
	// Details are the writes, as the library names them.
	Details []string
	// Warning marks a change that will not happen as asked (a device the
	// hub cannot create, say).
	Warning bool
}

// Plan works out what applying the edits writes, by applying them to a
// simulated copy of the hub as it was read: the same writes, in the same
// order, that Apply then makes on the hub.
func (w *Workspace) Plan(ctx context.Context) (*Plan, error) {
	sim, err := w.Dump.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}
	defer func() { _ = sim.Close() }()
	var r reporter
	if err := profile.ApplyConfig(ctx, sim, w.Config, w.applyOptions(r.line, simPair)); err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}
	return r.plan(), nil
}

// Apply writes the edits to the hub c is connected to, calling progress
// (if not nil) after each write with how many are done of total, the
// count Plan gave. ctx cancels it between writes; what was written
// stays written, so read the hub again after a failure.
//
// It returns the workspace of the hub after the writes, without reading
// it, when the hub answered them as a simulated copy of it answers the
// same writes: no one else can change the hub while c is connected, and
// the hub's answers carry what it decides itself (the ids it gives).
// Otherwise it returns nil: read the hub back (ReadBack).
func (w *Workspace) Apply(ctx context.Context, c *hub.Client, total int, progress func(done, total int, step string)) (*Workspace, error) {
	done := 0
	report := func(line string) {
		if strings.HasPrefix(line, "  ") || reDeleted.MatchString(line) || reOrdered.MatchString(line) {
			done++
		}
		if progress != nil {
			progress(min(done, total), total, strings.TrimSpace(line))
		}
	}
	var real exchanges
	stop := c.Session().Watch(real.add)
	pair := func(ctx context.Context, c *hub.Client, id byte, name string) error {
		if progress != nil {
			progress(min(done, total), total, i18n.T("Pair %s now: in its Bluetooth settings (or its remote's pairing), pick \"Sofabaton\", a name with an id after it.", name))
		}
		ctx, cancel := context.WithTimeout(ctx, PairTimeout)
		defer cancel()
		err := c.Pair(ctx, id)
		if errors.Is(err, context.DeadlineExceeded) {
			return errors.New(i18n.T("%s did not pair in time: the hub keeps a Bluetooth device only once it has paired. Apply again to try once more.", name))
		}
		return err
	}
	err := profile.ApplyConfig(ctx, c, w.Config, w.applyOptions(report, pair))
	stop()
	if err != nil {
		return nil, err
	}
	return w.predict(ctx, real.frames()), nil
}

// predict is the workspace of the hub after the edits, made by applying
// them to a simulated copy of the hub as it was read, if the copy
// exchanged real's frames; nil if it did not.
func (w *Workspace) predict(ctx context.Context, real []exchanged) *Workspace {
	m, err := w.Dump.Model()
	if err != nil {
		return nil
	}
	sim, err := hub.Open(ctx, hubsim.Pipe(hubsim.FromModel(m)), hub.Options{})
	if err != nil {
		return nil
	}
	defer func() { _ = sim.Close() }()
	var got exchanges
	stop := sim.Session().Watch(got.add)
	err = profile.ApplyConfig(ctx, sim, w.Config, w.applyOptions(nil, simPair))
	stop()
	if err != nil || !slices.Equal(real, got.frames()) {
		if differ != nil {
			differ(real, got.frames())
		}
		return nil
	}
	d, err := dump.ReadProgress(ctx, sim, Tool, nil, nil)
	if err != nil {
		return nil
	}
	next, err := FromDump(ctx, d)
	if err != nil {
		return nil
	}
	return next
}

// differ, if set (by tests), is told of an apply the simulated copy did
// not exchange as the hub did.
var differ func(real, sim []exchanged)

// exchanged is a frame of an apply that a simulated hub must match: every
// frame sent, and the hub's answers to writes, by their family and first
// byte (the status, or the id the hub gave). Replies to reads are left
// out: they carry what the hub changes on its own (power states).
type exchanged struct {
	dir     wire.Direction
	family  wire.Family
	payload string
}

// exchanges records an apply's frames; the session adds them from its
// goroutines.
type exchanges struct {
	mu   sync.Mutex
	list []exchanged
}

func (x *exchanges) add(d wire.Direction, f wire.Frame) {
	e := exchanged{dir: d, family: f.Family, payload: string(f.Payload)}
	if d == wire.FromHub {
		switch f.Family {
		case wire.FamStatusAck, wire.FamCreateDeviceAck, wire.FamMacroAck, wire.FamCreateActAck, wire.FamButtonAck:
			e.payload = string(f.Payload[:min(len(f.Payload), 1)])
		default:
			return
		}
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.list = append(x.list, e)
}

func (x *exchanges) frames() []exchanged {
	x.mu.Lock()
	defer x.mu.Unlock()
	return slices.Clone(x.list)
}

// applyOptions make the hub hold exactly the workspace: matched by id, as
// the configuration was read from it, with what was removed deleted. What
// was not edited is not read again. pair pairs the Bluetooth devices
// created.
func (w *Workspace) applyOptions(report func(string), pair func(context.Context, *hub.Client, byte, string) error) profile.ApplyOptions {
	return profile.ApplyOptions{ByID: true, Prune: true, Delete: true, Report: report, Base: w.base, Pair: pair}
}

// PairTimeout is how long applying waits for a new Bluetooth device to
// pair.
var PairTimeout = 2 * time.Minute

// simPair pairs a device on a simulated hub, which pairs it at once.
func simPair(ctx context.Context, c *hub.Client, id byte, _ string) error { return c.Pair(ctx, id) }

var (
	reChanges = regexp.MustCompile(`^(device|activity) "(.*)" \(\d+\): (\d+) changes$`)
	reCreated = regexp.MustCompile(`^(device|activity) "(.*)": created as \d+( and paired)?$`)
	reOrdered = regexp.MustCompile(`^(devices|activities): reordered$`)
	reDeleted = regexp.MustCompile(`^(device|activity) "(.*)" \(\d+\): deleted$`)
	reSkipped = regexp.MustCompile(`^(device|activity) "(.*)": skipped: (.*)$`)
	reMatches = regexp.MustCompile(`: matches$`)
)

// reporter turns ApplyConfig's report into a plan's changes. A change's
// line says how many writes follow it; a create's writes come before its
// line (they are made while creating).
type reporter struct {
	changes []Change
	owed    int      // writes still to come for the last change
	pending []string // writes for a create not yet reported
	writes  int
}

func (r *reporter) line(l string) {
	if strings.HasPrefix(l, "  ") {
		r.writes++
		if r.owed > 0 {
			last := &r.changes[len(r.changes)-1]
			last.Details = append(last.Details, strings.TrimSpace(l))
			r.owed--
			return
		}
		r.pending = append(r.pending, strings.TrimSpace(l))
		return
	}
	r.owed = 0
	switch {
	case reMatches.MatchString(l):
	case reCreated.MatchString(l):
		m := reCreated.FindStringSubmatch(l)
		r.changes = append(r.changes, Change{Title: changeTitle(m[1], i18n.Mark("Add device %s"), i18n.Mark("Add activity %s"), m[2]), Details: r.pending})
		r.pending = nil
	case reChanges.MatchString(l):
		m := reChanges.FindStringSubmatch(l)
		r.changes = append(r.changes, Change{Title: changeTitle(m[1], i18n.Mark("Change device %s"), i18n.Mark("Change activity %s"), m[2])})
		r.owed, _ = strconv.Atoi(m[3])
	case reDeleted.MatchString(l):
		m := reDeleted.FindStringSubmatch(l)
		// One write, which takes the hub up to 15 s for a device.
		r.changes = append(r.changes, Change{Title: changeTitle(m[1], i18n.Mark("Delete device %s"), i18n.Mark("Delete activity %s"), m[2]), Details: []string{"delete"}})
		r.writes++
	case reOrdered.MatchString(l):
		m := reOrdered.FindStringSubmatch(l)
		title := i18n.T("Change the order of the devices")
		if m[1] == "activities" {
			title = i18n.T("Change the order of the activities")
		}
		r.changes = append(r.changes, Change{Title: title, Details: []string{"order"}})
		r.writes++
	case reSkipped.MatchString(l):
		m := reSkipped.FindStringSubmatch(l)
		r.changes = append(r.changes, Change{Title: changeTitle(m[1], i18n.Mark("Cannot add device %s: %s"), i18n.Mark("Cannot add activity %s: %s"), m[2], m[3]), Warning: true})
	default:
		r.changes = append(r.changes, Change{Title: l})
	}
}

// changeTitle is a change's title, in the user's language: device's
// format for the library's kind "device", activity's for "activity".
func changeTitle(kind, device, activity string, args ...any) string {
	if kind == "activity" {
		return i18n.T(activity, args...)
	}
	return i18n.T(device, args...)
}

func (r *reporter) plan() *Plan {
	return &Plan{Changes: r.changes, Writes: r.writes}
}
