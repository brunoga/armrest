package workspace

import (
	"errors"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/brunoga/sofabaton/record"
)

func TestBuild(t *testing.T) {
	s := Spec{
		Name: " Projector ", Brand: "Acme", Protocol: "NEC", System: "0x00CE",
		Commands: []CommandSpec{
			{"Power On", "0x01"}, {"Power Off", "0x02"}, {"Volume Up", "0x10"}, {"Select", "40"}, {"", ""}, {"Input HDMI", "0x30"},
		},
		PowerOn: "Power On", PowerOff: "Power Off",
	}
	d, err := s.Build()
	if err != nil {
		t.Fatal(err)
	}
	y, _ := yaml.Marshal(d)
	got := string(y)
	for _, want := range []string{"device: Projector", "brand: Acme", "protocol: NEC", "system: 0x00CE",
		"name: Select\n      code: 40", "Vol+: Volume Up", "OK: Select", "\"on\": Power On"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if len(d.Buttons) != 2 {
		t.Errorf("keys %v, want OK and Vol+ only", d.Buttons)
	}

	k := Spec{Name: "AVR", Protocol: "DenonK", Device: "4", Subdevice: "1", Commands: []CommandSpec{{"Mute", "2312"}}}
	if _, err := k.Build(); err != nil {
		t.Errorf("DenonK: %v", err)
	}

	// RC5 has no descriptor: its commands are raw timings.
	r := Spec{Name: "Amp", Protocol: "RC5", Device: "16", Commands: []CommandSpec{{"Volume Up", "16"}}}
	rd, err := r.Build()
	if err != nil {
		t.Fatalf("RC5: %v", err)
	}
	sig, err := rd.Commands[0].Signal(rd)
	if err != nil {
		t.Fatal(err)
	}
	if raw, err := record.ParseRawIR(sig); err != nil || raw.Carrier != 36000 || rd.IR != nil || rd.Buttons["Vol+"].Short != "Volume Up" {
		t.Errorf("RC5: %+v, ir %v, keys %v (%v)", raw, rd.IR, rd.Buttons, err)
	}
	if _, err := (Spec{Name: "Amp", Protocol: "RC5", Device: "32", Commands: []CommandSpec{{"A", "1"}}}).Build(); err == nil {
		t.Error("RC5 device 32 built")
	}

	for _, tc := range []struct {
		spec  Spec
		field string
		index int
	}{
		{Spec{Protocol: "NEC", System: "0x00CE", Commands: []CommandSpec{{"A", "1"}}}, "name", 0},
		{Spec{Name: "X", Protocol: "NEC", System: "zz", Commands: []CommandSpec{{"A", "1"}}}, "system", 0},
		{Spec{Name: "X", Protocol: "NEC", System: "0xCE"}, "commands", 0},
		{Spec{Name: "X", Protocol: "NEC", System: "0xCE", Commands: []CommandSpec{{"A", "1"}, {"B", "0x100"}}}, "command", 1},
		{Spec{Name: "X", Protocol: "NEC", System: "0xCE", Commands: []CommandSpec{{"A", "1"}, {"a", "2"}}}, "command", 1},
		{Spec{Name: "X", Protocol: "NEC", System: "0xCE", Commands: []CommandSpec{{"", "1"}}}, "command", 0},
		{Spec{Name: "X", Protocol: "NEC", System: "0xCE", Commands: []CommandSpec{{"A", "1"}}, PowerOn: "A"}, "power", 0},
		{Spec{Name: "X", Protocol: "DenonK", Device: "16", Subdevice: "1", Commands: []CommandSpec{{"A", "1"}}}, "device", 0},
		{Spec{Name: "X", Protocol: "Denon", Device: "2", Commands: []CommandSpec{{"A", "300"}}}, "command", 0},
	} {
		_, err := tc.spec.Build()
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != tc.field || fe.Index != tc.index {
			t.Errorf("%+v: got %v, want a %s error at %d", tc.spec, err, tc.field, tc.index)
		}
	}
}

func TestKeyFor(t *testing.T) {
	for name, want := range map[string]string{"Volume Up": "Vol+", "select": "OK", "Play/Pause": "Play", "Channel Down": "Ch-", "7": "7", "Power": "Power"} {
		if got, ok := KeyFor(name); !ok || got != want {
			t.Errorf("%q: got %q, want %q", name, got, want)
		}
	}
	if _, ok := KeyFor("Input HDMI"); ok {
		t.Error("Input HDMI is a key")
	}
}
