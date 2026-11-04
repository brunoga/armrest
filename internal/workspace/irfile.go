package workspace

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/irfile"
	"github.com/brunoga/sofabaton/profile"
)

// IRFileExts are the extensions of the files of IR codes FromIRFile reads.
var IRFileExts = []string{".ir", ".csv", ".json", ".conf", ".lircd", ".girr", ".xml", ".txt"}

// Imported is a device made of a file of IR codes, with the codes left
// out of it.
type Imported struct {
	Device  *profile.Device
	Skipped []irfile.Skipped
}

// FromIRFile makes a device of a file of IR codes named file: Flipper
// Zero's .ir, irdb's .csv, SmartIR's .json, LIRC's .conf, Girr, or codes
// one a line in a .txt (Pronto, sendir, Broadlink, Unfolded Circle). Its
// commands named like a key of the remote are set on that key, and its
// power commands make its power; codes the X2 cannot send are left out,
// and listed.
func FromIRFile(file string, r io.Reader) (*Imported, error) {
	sigs, err := irfile.Read(file, r)
	if err != nil {
		return nil, err
	}
	name := strings.Join(strings.Fields(strings.ReplaceAll(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)), "_", " ")), " ")
	d, skipped := irfile.Device(name, sigs)
	if len(d.Commands) == 0 {
		if len(skipped) > 0 {
			return nil, errors.New(i18n.T("%s cannot be added: none of its codes can be sent by the hub yet (%s)", filepath.Base(file), skipped[0].Why))
		}
		return nil, errors.New(i18n.T("%s cannot be added: it has no codes", filepath.Base(file)))
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
		return nil, fmt.Errorf("%s: %s: %w", filepath.Base(file), i18n.T("the device is not valid"), err)
	}
	return &Imported{Device: d, Skipped: skipped}, nil
}
