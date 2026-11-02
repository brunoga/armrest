package workspace

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/brunoga/sofabaton/profile"
	"github.com/brunoga/sofabaton/templates"
)

// Template is a device template the app ships with.
type Template struct {
	// Name is the template's name in the library: "ir/ugoos/am6b-plus".
	Name string
	// Model and Brand are the device's: "Ugoos AM6B Plus", "Ugoos".
	Model, Brand string
	// Summary is the first line of its notes.
	Summary string
	// Commands is how many commands it has.
	Commands int
}

// Templates lists the templates, by brand and model.
func Templates() []Template {
	var out []Template
	for _, name := range templates.Names() {
		data, err := templates.Read(name)
		if err != nil {
			continue
		}
		var top struct {
			Template string `yaml:"template"`
			Brand    string `yaml:"brand"`
			Commands []any  `yaml:"commands"`
		}
		if yaml.Unmarshal(data, &top) != nil || top.Template == "" {
			continue
		}
		t := Template{Name: name, Model: top.Template, Brand: top.Brand, Commands: len(top.Commands)}
		if first, _, _ := bytes.Cut(data, []byte("\n")); bytes.HasPrefix(first, []byte("# ")) {
			t.Summary = strings.TrimSpace(string(first[2:]))
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Brand != out[j].Brand {
			return strings.ToLower(out[i].Brand) < strings.ToLower(out[j].Brand)
		}
		return strings.ToLower(out[i].Model) < strings.ToLower(out[j].Model)
	})
	return out
}

// Matches reports whether the template matches a search: every word of
// it is in the brand, model or summary.
func (t Template) Matches(search string) bool {
	hay := strings.ToLower(t.Brand + " " + t.Model + " " + t.Summary)
	for _, w := range strings.Fields(strings.ToLower(search)) {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

// Device is a new device from the template, named name (the model if
// empty).
func (t Template) Device(name string) (*profile.Device, error) {
	data, err := templates.Read(t.Name)
	if err != nil {
		return nil, err
	}
	y, err := profile.FromTemplate(data, t.Name, strings.TrimSpace(name))
	if err != nil {
		return nil, err
	}
	d, err := profile.Load(bytes.NewReader(y))
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", t.Name, err)
	}
	return d, nil
}
