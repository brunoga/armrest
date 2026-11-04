package workspace

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/profile"
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
	if err := profile.ApplyConfig(ctx, sim, w.Config, w.applyOptions(r.line)); err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}
	return r.plan(), nil
}

// Apply writes the edits to the hub c is connected to, calling progress
// (if not nil) after each write with how many are done of total, the
// count Plan gave. ctx cancels it between writes; what was written
// stays written, so read the hub again after a failure.
func (w *Workspace) Apply(ctx context.Context, c *hub.Client, total int, progress func(done, total int, step string)) error {
	done := 0
	report := func(line string) {
		if strings.HasPrefix(line, "  ") || reDeleted.MatchString(line) {
			done++
		}
		if progress != nil {
			progress(min(done, total), total, strings.TrimSpace(line))
		}
	}
	return profile.ApplyConfig(ctx, c, w.Config, w.applyOptions(report))
}

// applyOptions make the hub hold exactly the workspace: matched by id, as
// the configuration was read from it, with what was removed deleted. What
// was not edited is not read again.
func (w *Workspace) applyOptions(report func(string)) profile.ApplyOptions {
	return profile.ApplyOptions{ByID: true, Prune: true, Delete: true, Report: report, Base: w.base}
}

var (
	reChanges = regexp.MustCompile(`^(device|activity) "(.*)" \(\d+\): (\d+) changes$`)
	reCreated = regexp.MustCompile(`^(device|activity) "(.*)": created as \d+$`)
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
		r.changes = append(r.changes, Change{Title: fmt.Sprintf("Add %s %s", m[1], m[2]), Details: r.pending})
		r.pending = nil
	case reChanges.MatchString(l):
		m := reChanges.FindStringSubmatch(l)
		r.changes = append(r.changes, Change{Title: fmt.Sprintf("Change %s %s", m[1], m[2])})
		r.owed, _ = strconv.Atoi(m[3])
	case reDeleted.MatchString(l):
		m := reDeleted.FindStringSubmatch(l)
		// One write, which takes the hub up to 15 s for a device.
		r.changes = append(r.changes, Change{Title: fmt.Sprintf("Delete %s %s", m[1], m[2]), Details: []string{"delete"}})
		r.writes++
	case reSkipped.MatchString(l):
		m := reSkipped.FindStringSubmatch(l)
		r.changes = append(r.changes, Change{Title: fmt.Sprintf("Cannot add %s %s: %s", m[1], m[2], m[3]), Warning: true})
	default:
		r.changes = append(r.changes, Change{Title: l})
	}
}

func (r *reporter) plan() *Plan {
	return &Plan{Changes: r.changes, Writes: r.writes}
}
