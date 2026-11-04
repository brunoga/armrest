package workspace

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/brunoga/armrest/internal/demo"
	"github.com/brunoga/sofabaton/profile"
)

func demoWorkspace(t *testing.T) *Workspace {
	t.Helper()
	d, err := demo.Dump()
	if err != nil {
		t.Fatal(err)
	}
	w, err := FromDump(context.Background(), d)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestSaveDevice(t *testing.T) {
	w := demoWorkspace(t)
	e, err := w.EditDevice("receiver")
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Commands) == 0 || e.Commands[0].Fixed && w.CanAddCommands("Receiver") {
		t.Logf("receiver commands: %+v", e.Commands[:3])
	}
	users := w.ActivitiesUsing("Receiver")
	if len(users) == 0 {
		t.Fatal("no activity uses the receiver")
	}

	// Rename the device and swap two commands' names; drop one.
	e.Name = "AVR"
	var mute, volUp, volDown = -1, -1, -1
	for i, c := range e.Commands {
		switch c.Name {
		case "Mute":
			mute = i
		case "Volume Up":
			volUp = i
		case "Volume Down":
			volDown = i
		}
	}
	if mute < 0 || volUp < 0 || volDown < 0 {
		t.Fatalf("commands: %+v", e.Commands)
	}
	e.Commands[volUp].Name, e.Commands[volDown].Name = "Volume Down", "Volume Up"
	e.Commands = append(e.Commands[:mute], e.Commands[mute+1:]...)
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	if w.Config.Device("Receiver") != nil || w.Config.Device("AVR") == nil {
		t.Fatal("not renamed")
	}
	a := w.Config.Activity(users[0])
	y, _ := yaml.Marshal(a)
	got := string(y)
	if strings.Contains(got, "Receiver") || strings.Contains(got, "AVR: Mute") {
		t.Errorf("%s still uses the old names or Mute:\n%s", users[0], got)
	}
	// Vol+ ran Volume Up; that command is now named Volume Down.
	if a.Buttons["Vol+"].Short != "AVR: Volume Down" {
		t.Errorf("Vol+ runs %q", a.Buttons["Vol+"].Short)
	}

	p, err := w.Plan(context.Background())
	if err != nil || !strings.Contains(titles(p), "Change device AVR") {
		t.Errorf("plan: %v\n%s", err, titles(p))
	}

	// A bad edit changes nothing.
	before, _ := yaml.Marshal(w.Config)
	e, _ = w.EditDevice("AVR")
	e.Commands[1].Name = e.Commands[0].Name
	err = w.SaveDevice(e)
	var fe *FieldError
	if !errors.As(err, &fe) || fe.Field != "command" || fe.Index != 1 {
		t.Errorf("duplicate name: %v", err)
	}
	after, _ := yaml.Marshal(w.Config)
	if string(before) != string(after) {
		t.Error("a failed edit changed the configuration")
	}
}

func TestSaveDeviceCodes(t *testing.T) {
	w := demoWorkspace(t)
	name := "Projector/Google TV"
	if !w.CanAddCommands(name) {
		t.Fatal("the projector has no IR protocol")
	}
	e, _ := w.EditDevice(name)
	e.Commands = append(e.Commands, CommandEdit{Name: "Extra", Code: "0xD9"})
	e.Keys["Red"] = profile.Key{Short: "Extra"}
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	d := w.Config.Device(name)
	last := d.Commands[len(d.Commands)-1]
	if last.Name != "Extra" || last.Code == nil || last.Code.Value != 0xD9 || d.Buttons["Red"].Short != "Extra" {
		t.Errorf("Extra: %+v, Red: %+v", last, d.Buttons["Red"])
	}
	e, _ = w.EditDevice(name)
	e.Commands[len(e.Commands)-1].Code = "0x1FF"
	if err := w.SaveDevice(e); err == nil {
		t.Error("a code out of range was saved")
	}
	if err := w.RemoveDevice(name); err != nil || w.Config.Device(name) != nil {
		t.Errorf("remove: %v", err)
	}
	if len(w.ActivitiesUsing(name)) != 0 {
		t.Error("activities still use the removed projector")
	}
}

// A command recoded under the same name keeps its record on the hub, so
// reading back after applying reads the edited devices' signals again.
func TestReadBackRecoded(t *testing.T) {
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
	name := "Projector/Google TV"
	e, _ := w.EditDevice(name)
	e.Commands[0].Code = "0x7F"
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	p, err := w.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	back := applied(t, w, c, p.Writes, nil, true)
	if got := back.Config.Device(name).Commands[0].Code; got == nil || got.Value != 0x7F {
		t.Errorf("read back %+v, want 0x7F", got)
	}
}

// Clearing a key of a device the hub has clears it.
func TestClearKey(t *testing.T) {
	w := demoWorkspace(t)
	name := "Projector/Google TV"
	e, _ := w.EditDevice(name)
	if _, ok := e.Keys["OK"]; !ok {
		t.Fatalf("keys: %v", e.Keys)
	}
	delete(e.Keys, "OK")
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	p, err := w.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != 1 || p.Changes[0].Warning || !slices.Contains(p.Changes[0].Details, "button      OK unbound") {
		t.Fatalf("plan:\n%s", titles(p))
	}
}

// A device's macros are added, renamed (the activities' keys follow) and
// removed.
func TestSaveDeviceMacros(t *testing.T) {
	w := demoWorkspace(t)
	name := "Projector/Google TV"
	e, _ := w.EditDevice(name)
	first := e.Commands[0].Name
	e.Macros = append(e.Macros, MacroEdit{Name: "Twice", Steps: []profile.DeviceStep{{Command: first}, {Seconds: 1}, {Command: first}}})
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	act := w.Config.Activities[0]
	act.Buttons["Blue"] = profile.Key{Short: name + ": Twice"}

	e, _ = w.EditDevice(name)
	last := len(e.Macros) - 1
	if e.Macros[last].Name != "Twice" || len(e.Macros[last].Steps) != 3 {
		t.Fatalf("macros: %+v", e.Macros)
	}
	e.Macros[last].Name = "Double"
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	if got := act.Buttons["Blue"].Short; got != name+": Double" {
		// act is the old copy: SaveDevice replaced the configuration.
		if got := w.Config.Activity(act.Name).Buttons["Blue"].Short; got != name+": Double" {
			t.Errorf("Blue runs %q", got)
		}
	}
	p, err := w.Plan(context.Background())
	if err != nil || len(p.Changes) == 0 {
		t.Errorf("plan: %v %+v", err, p)
	}

	e, _ = w.EditDevice(name)
	bad := e
	bad.Macros = append([]MacroEdit(nil), e.Macros...)
	bad.Macros[len(bad.Macros)-1].Name = first
	if err := w.SaveDevice(bad); err == nil {
		t.Error("a macro took a command's name")
	}
	e.Macros = e.Macros[:len(e.Macros)-1]
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	if _, ok := w.Config.Activity(act.Name).Buttons["Blue"]; ok {
		t.Error("Blue still runs the removed macro")
	}
}
