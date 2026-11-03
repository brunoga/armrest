package workspace

import (
	"strings"
	"testing"
)

// A made-up Flipper file: its keys go on the remote's, its power on the
// device's power; the raw signal is left out.
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
	if d.Name != "Living Room TV" || len(d.Commands) != 3 {
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
	if len(im.Skipped) != 1 || im.Skipped[0].Name != "Netflix" {
		t.Errorf("skipped %+v", im.Skipped)
	}

	raw := "Filetype: IR signals file\nVersion: 1\nname: Power\ntype: raw\nfrequency: 38000\ndata: 1 2\n"
	if _, err := FromIRFile("x.ir", strings.NewReader(raw)); err == nil || !strings.Contains(err.Error(), "raw timings") {
		t.Errorf("a file of raw codes: %v", err)
	}
}
