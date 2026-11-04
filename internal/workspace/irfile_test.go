package workspace

import (
	"fmt"
	"strings"
	"testing"
)

// A made-up Flipper file: its keys go on the remote's, its power on the
// device's power; the raw signal is a command too, sent as it is.
func TestFromIRFile(t *testing.T) {
	const file = `Filetype: IR signals file
Version: 1
#
name: Power
type: parsed
protocol: NEC
address: 04 00 00 00
command: 08 00 00 00
#
name: Vol_up
type: parsed
protocol: NEC
address: 04 00 00 00
command: 02 00 00 00
#
name: Ok
type: parsed
protocol: NEC
address: 04 00 00 00
command: 44 00 00 00
#
name: Netflix
type: raw
frequency: 38000
duty_cycle: 0.330000
data: 9000 4500
`
	im, err := FromIRFile("Living_Room_TV.ir", strings.NewReader(file))
	if err != nil {
		t.Fatal(err)
	}
	d := im.Device
	if d.Name != "Living Room TV" || len(d.Commands) != 4 || d.Commands[3].Data == "" {
		t.Fatalf("%s: %+v", d.Name, d.Commands)
	}
	for key, cmd := range map[string]string{"Power": "Power", "Vol+": "Volume Up", "OK": "OK"} {
		if d.Buttons[key].Short != cmd {
			t.Errorf("%s runs %q", key, d.Buttons[key].Short)
		}
	}
	if d.Power == nil || d.Power.On != "Power" || d.Power.Off != "Power" {
		t.Errorf("power %+v", d.Power)
	}
	if len(im.Skipped) != 0 {
		t.Errorf("skipped %+v", im.Skipped)
	}

	odd := "Filetype: IR signals file\nVersion: 1\nname: Power\ntype: parsed\nprotocol: Samsung36\naddress: 01 00 00 00\ncommand: 02 00 00 00\n"
	if _, err := FromIRFile("x.ir", strings.NewReader(odd)); err == nil || !strings.Contains(err.Error(), "Samsung36") {
		t.Errorf("a file of codes the hub cannot send: %v", err)
	}
}

// A Sofabaton code set's power is the one it names, not one guessed from
// its keys' names.
func TestFromSofabatonSet(t *testing.T) {
	hexOf := func(s string) string { return fmt.Sprintf("%X", s) }
	set := fmt.Sprintf(`{"keys": [
		{"fid": 1, "keyname": "Power", "format": "1", "irCode": %q},
		{"fid": 5907, "keyname": "Power on", "format": "1", "irCode": %q},
		{"fid": 5912, "keyname": "Power off", "format": "1", "irCode": %q}],
		"power": {"type": 1, "on": [{"fid": 1}], "off": [{"fid": 1}]}}`,
		hexOf("P:NEC R:38400 D:4 S:251 F:8"), hexOf("P:NEC R:38400 D:4 S:251 F:196"), hexOf("P:NEC R:38400 D:4 S:251 F:197"))
	im, err := FromIRFile("c1.sofabaton", strings.NewReader(set))
	if err != nil {
		t.Fatal(err)
	}
	if p := im.Device.Power; p == nil || p.On != "Power" || p.Off != "Power" {
		t.Errorf("power %+v, want the toggle", p)
	}
	im, err = FromIRFile("c1.sofabaton", strings.NewReader(strings.Replace(set, `"type": 1`, `"type": 0`, 1)))
	if err != nil || im.Device.Power != nil {
		t.Errorf("a set of no power: %+v, %v", im.Device.Power, err)
	}
}
