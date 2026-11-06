package workspace

import (
	"errors"
	"testing"

	"github.com/brunoga/sofabaton/profile"
)

func TestBluetoothBuild(t *testing.T) {
	s := BluetoothSpec{
		Name: " TV Box ",
		Commands: []BluetoothCommand{
			{"", "OK"},
			{"", ""},
			{"Start", "Play/Pause"},
			{"Power", "Power"},
		},
		PowerOn: "Power", PowerOff: "Power",
	}
	d, err := s.Build()
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "TV Box" || d.Category != bluetoothCategory || d.Bluetooth == nil || d.Bluetooth.Profile != OtherBluetoothProfile || d.Power == nil {
		t.Fatalf("built %+v", d)
	}
	want := []profile.Command{{Name: "OK", Data: "0c0041"}, {Name: "Start", Data: "0c00cd"}, {Name: "Power", Data: "0c0030"}}
	if len(d.Commands) != len(want) {
		t.Fatalf("commands %+v", d.Commands)
	}
	for i, c := range want {
		if d.Commands[i] != c {
			t.Errorf("command %d: %+v, want %+v", i, d.Commands[i], c)
		}
	}
	if d.Buttons["OK"].Short != "OK" || d.Buttons["Power"].Short != "Power" {
		t.Errorf("keys %+v", d.Buttons)
	}

	for _, bad := range []struct {
		commands []BluetoothCommand
		index    int
	}{
		{[]BluetoothCommand{{"A", ""}}, 0},                  // no key
		{[]BluetoothCommand{{"A", "OK"}, {"a", "Back"}}, 1}, // the same name
	} {
		var fe *FieldError
		if _, err := (BluetoothSpec{Name: "X", Commands: bad.commands}).Build(); !errors.As(err, &fe) || fe.Field != "command" || fe.Index != bad.index {
			t.Errorf("%+v: %v", bad.commands, err)
		}
	}
}

// The keys' usages are those of the catalog's Android Box profile.
func TestHIDKeys(t *testing.T) {
	for name, usage := range map[string]string{"A": "070004", "Z": "07001d", "1": "07001e", "0": "070027", "F1": "07003a", "F12": "070045", "right": "07004f"} {
		if k, ok := HIDKeyNamed(name); !ok || k.Usage != usage {
			t.Errorf("%s: %+v, want %s", name, k, usage)
		}
	}
	seen := map[string]bool{}
	for _, k := range HIDKeys {
		if seen[k.Name] {
			t.Errorf("%s twice", k.Name)
		}
		seen[k.Name] = true
	}
	for _, n := range UsualBluetoothKeys {
		if !seen[n] {
			t.Errorf("usual key %s is not a key", n)
		}
	}
}
