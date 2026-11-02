package workspace

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/brunoga/armrest/internal/demo"
	"github.com/brunoga/sofabaton/profile"
	"github.com/brunoga/sofabaton/templates"
)

func templateDevice(t *testing.T, name, as string) *profile.Device {
	t.Helper()
	tmpl, err := templates.Read(name)
	if err != nil {
		t.Fatal(err)
	}
	y, err := profile.FromTemplate(tmpl, name, as)
	if err != nil {
		t.Fatal(err)
	}
	d, err := profile.Load(bytes.NewReader(y))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func titles(p *Plan) string {
	var out []string
	for _, c := range p.Changes {
		out = append(out, c.Title)
	}
	return strings.Join(out, "\n")
}

// Edits are planned on a simulated copy of the hub, applied, and read back:
// then there is nothing left to apply.
func TestPlanAndApply(t *testing.T) {
	ctx := context.Background()
	c, err := demo.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	w, err := Read(ctx, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w.Changed() {
		t.Fatal("changed before any edit")
	}
	p, err := w.Plan(ctx)
	if err != nil || len(p.Changes) != 0 || p.Writes != 0 {
		t.Fatalf("plan before any edit: %v %+v", err, p)
	}

	box := templateDevice(t, "ir/ugoos/am6b-plus", "Media Box")
	if err := w.AddDevice(box); err != nil {
		t.Fatal(err)
	}
	if err := w.AddDevice(templateDevice(t, "ir/ugoos/am6b-plus", "media box")); err == nil {
		t.Error("added a second device of the same name")
	}
	if err := w.Config.RenameDevice("3D Player", "Blu-ray"); err != nil {
		t.Fatal(err)
	}
	if !w.Changed() || w.DeviceState(box) != New || w.DeviceState(w.Config.Device("Blu-ray")) != Edited ||
		w.DeviceState(w.Config.Device("Receiver")) != Unchanged {
		t.Error("states")
	}
	if a := w.Config.Activity("Watch 3D Movies"); w.ActivityState(a) != Edited {
		t.Error("an activity using the renamed device is not edited")
	}

	p, err = w.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := titles(p)
	// The activities using the 3D player name it by id on the hub, so its
	// rename writes nothing to them.
	for _, want := range []string{"Change device Blu-ray", "Add device Media Box"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, ch := range p.Changes {
		if len(ch.Details) == 0 {
			t.Errorf("%s: no writes", ch.Title)
		}
	}

	var last [2]int
	if err := w.Apply(ctx, c, p.Writes, func(done, total int, _ string) { last = [2]int{done, total} }); err != nil {
		t.Fatal(err)
	}
	if last[0] != p.Writes || last[1] != p.Writes {
		t.Errorf("progress ended at %v, want %d of %d", last, p.Writes, p.Writes)
	}

	again, err := Refresh(ctx, c, w.Dump, nil)
	if err != nil {
		t.Fatal(err)
	}
	if again.Config.Device("Media Box") == nil || again.Config.Device("Blu-ray") == nil {
		t.Fatal("the edits are not on the hub")
	}
	if p, err := again.Plan(ctx); err != nil || len(p.Changes) != 0 {
		t.Errorf("after applying: %v\n%s", err, titles(p))
	}

	// A delete is one write: a long one, which the progress bar counts.
	if err := again.Config.RemoveDevice("Media Box"); err != nil {
		t.Fatal(err)
	}
	if p, err := again.Plan(ctx); err != nil || titles(p) != "Delete device Media Box" || p.Writes != 1 {
		t.Errorf("delete: %v %+v", err, p)
	}
}

func TestTemplates(t *testing.T) {
	ts := Templates()
	if len(ts) < 3 {
		t.Fatalf("%d templates", len(ts))
	}
	var ugoos *Template
	for i, x := range ts {
		if x.Name == "ir/ugoos/am6b-plus" {
			ugoos = &ts[i]
		}
	}
	if ugoos == nil || ugoos.Brand != "Ugoos" || ugoos.Model != "Ugoos AM6B Plus" || ugoos.Commands == 0 || ugoos.Summary == "" {
		t.Fatalf("ugoos: %+v", ugoos)
	}
	if !ugoos.Matches("ugoos am6b") || ugoos.Matches("denon") {
		t.Error("matches")
	}
	d, err := ugoos.Device("")
	if err != nil || d.Name != "Ugoos AM6B Plus" || len(d.Commands) != ugoos.Commands {
		t.Errorf("device: %v %+v", err, d)
	}
}
