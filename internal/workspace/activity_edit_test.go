package workspace

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/brunoga/armrest/internal/demo"
	"github.com/brunoga/sofabaton/profile"
)

// Saving an activity as it is changes nothing.
func TestSaveActivityUnchanged(t *testing.T) {
	w := demoWorkspace(t)
	for _, a := range w.Config.Activities {
		e, err := w.EditActivity(a.Name)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.SaveActivity(e); err != nil {
			t.Fatalf("%s: %v", a.Name, err)
		}
	}
	if w.Changed() {
		t.Error("saving the activities as they are changed them")
	}
}

func TestSaveActivity(t *testing.T) {
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

	// Edit one: rename it, and put a favorite and a key on it.
	e, _ := w.EditActivity("Watch Google TV")
	e.Name = "Google TV"
	cmds := w.Commands([]string{e.Devices[0].Device})
	e.Favorites = append(e.Favorites, cmds[0])
	e.Keys["Yellow"] = profile.Key{Short: cmds[1]}
	if err := w.SaveActivity(e); err != nil {
		t.Fatal(err)
	}
	if a := w.Config.Activity("Google TV"); a == nil || w.ActivityState(a) != Edited {
		t.Fatalf("not edited: %+v", a)
	}

	// A new one, starting and ending the usual way.
	n := w.NewActivity()
	n.Name = "Movie Night"
	n.Devices = []profile.Member{{Device: "3D Player"}, {Device: "Projector/Google TV"}}
	n.Keys["OK"] = profile.Key{Short: "3D Player: OK"}
	if err := w.SaveActivity(n); err != nil {
		t.Fatal(err)
	}
	p, err := w.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := titles(p)
	for _, want := range []string{"Change activity Google TV", "Add activity Movie Night"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if err := w.Apply(ctx, c, p.Writes, nil); err != nil {
		t.Fatal(err)
	}
	back, err := w.ReadBack(ctx, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := back.Config.Activity("Movie Night")
	if a == nil || len(a.Devices) != 2 || a.Buttons["OK"].Short != "3D Player: OK" || len(a.PowerOn) == 0 {
		t.Fatalf("Movie Night on the hub: %+v", a)
	}
	if p, err := back.Plan(ctx); err != nil || len(p.Changes) != 0 {
		t.Errorf("after applying: %v\n%s", err, titles(p))
	}
	if err := back.RemoveActivity("Movie Night"); err != nil {
		t.Fatal(err)
	}
	if p, err := back.Plan(ctx); err != nil || titles(p) != "Delete activity Movie Night" {
		t.Errorf("delete: %v\n%s", err, titles(p))
	}
}

func TestSaveActivityProblems(t *testing.T) {
	w := demoWorkspace(t)
	base := func() ActivityEdit {
		e := w.NewActivity()
		e.Name = "X"
		e.Devices = []profile.Member{{Device: "Receiver"}}
		return e
	}
	for _, tc := range []struct {
		edit  func(*ActivityEdit)
		field string
	}{
		{func(e *ActivityEdit) { e.Name = " " }, "name"},
		{func(e *ActivityEdit) { e.Name = "play pc" }, "name"},
		{func(e *ActivityEdit) { e.Devices = nil }, "devices"},
		{func(e *ActivityEdit) { e.Devices = append(e.Devices, profile.Member{Device: "receiver"}) }, "device"},
		{func(e *ActivityEdit) { e.Devices[0].Input = "Nope" }, "device"},
		{func(e *ActivityEdit) {
			e.PowerOn = []profile.Step{{Kind: profile.StepPowerOn, Device: "3D Player"}}
		}, "power_on"},
		{func(e *ActivityEdit) {
			e.Macros = []ActivityMacroEdit{{Name: "M", Steps: []profile.Step{{Kind: profile.StepCommand, Device: "Receiver", Command: "Nope"}}}}
		}, "macro"},
	} {
		e := base()
		tc.edit(&e)
		var fe *FieldError
		if err := w.SaveActivity(e); !errors.As(err, &fe) || fe.Field != tc.field {
			t.Errorf("got %v, want a %s problem", err, tc.field)
		}
	}
	if w.Changed() {
		t.Error("a failed save changed the configuration")
	}
}
