package workspace

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/brunoga/sofabaton/irfile"
	"github.com/brunoga/sofabaton/profile"
)

// IRFileExts are the extensions of the files of IR codes FromIRFile reads.
var IRFileExts = []string{".ir", ".csv"}

// Imported is a device made of a file of IR codes, with the codes left
// out of it.
type Imported struct {
	Device  *profile.Device
	Skipped []irfile.Skipped
}

// FromIRFile makes a device of a file of IR codes named file: Flipper
// Zero's .ir or irdb's .csv. Its commands named like a key of the remote
// are set on that key, and its power commands make its power; codes the
// X2 cannot send yet are left out, and listed.
func FromIRFile(file string, r io.Reader) (*Imported, error) {
	sigs, err := irfile.Read(file, r)
	if err != nil {
		return nil, err
	}
	name := strings.Join(strings.Fields(strings.ReplaceAll(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)), "_", " ")), " ")
	d, skipped := irfile.Device(name, sigs)
	if len(d.Commands) == 0 {
		why := "it has no codes"
		if len(skipped) > 0 {
			why = "none of its codes can be sent by the hub yet (" + skipped[0].Why + ")"
		}
		return nil, fmt.Errorf("%s cannot be added: %s", filepath.Base(file), why)
	}
	d.Buttons = profile.Buttons{}
	has := map[string]bool{}
	for _, c := range d.Commands {
		has[c.Name] = true
		if key, ok := KeyFor(c.Name); ok {
			if _, taken := d.Buttons[key]; !taken {
				d.Buttons[key] = profile.Key{Short: c.Name}
			}
		}
	}
	if len(d.Buttons) == 0 {
		d.Buttons = nil
	}
	switch {
	case has["Power On"] && has["Power Off"]:
		d.Power = &profile.Power{On: "Power On", Off: "Power Off"}
	case has["Power"]:
		d.Power = &profile.Power{On: "Power", Off: "Power"}
	}
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("%s: the device is not valid: %w", filepath.Base(file), err)
	}
	return &Imported{Device: d, Skipped: skipped}, nil
}
