package workspace

import (
	"errors"
	"strings"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/profile"
)

// OrderDevices lists the devices in the order of names, every device's:
// the order the remote and the app list them in.
func (w *Workspace) OrderDevices(names []string) error {
	cfg, err := cloneConfig(w.Config)
	if err != nil {
		return err
	}
	if cfg.Devices, err = ordered(cfg.Devices, names, func(d *profile.Device) string { return d.Name }); err != nil {
		return err
	}
	w.commit(cfg, i18n.T("the order of the devices"))
	return nil
}

// OrderActivities lists the activities in the order of names, as
// OrderDevices does the devices.
func (w *Workspace) OrderActivities(names []string) error {
	cfg, err := cloneConfig(w.Config)
	if err != nil {
		return err
	}
	if cfg.Activities, err = ordered(cfg.Activities, names, func(a *profile.Activity) string { return a.Name }); err != nil {
		return err
	}
	w.commit(cfg, i18n.T("the order of the activities"))
	return nil
}

// ordered is all, in the order of names, which name each of them once.
func ordered[T any](all []T, names []string, name func(T) string) ([]T, error) {
	if len(names) != len(all) {
		return nil, errors.New("workspace: an order names each one once")
	}
	out := make([]T, 0, len(all))
	used := map[int]bool{}
	for _, n := range names {
		found := false
		for i, x := range all {
			if !used[i] && strings.EqualFold(name(x), n) {
				out, used[i], found = append(out, x), true, true
				break
			}
		}
		if !found {
			return nil, errors.New("workspace: an order names each one once")
		}
	}
	return out, nil
}
