package workspace

import (
	"fmt"
	"strings"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/sofabaton/profile"
)

// Methods are the HTTP methods a network command can use.
var Methods = []string{"GET", "POST", "PUT", "DELETE"}

// networkCategory is the app's category for Wi-Fi devices.
const networkCategory profile.Category = 16

// NetworkSpec is a network device being made, as the user typed it: one
// whose commands are HTTP requests the hub sends.
type NetworkSpec struct {
	Name, Brand string
	Commands    []NetworkCommand
	// PowerOn and PowerOff name the commands that switch it on and off;
	// empty if none.
	PowerOn, PowerOff string
}

// NetworkCommand is a network command as typed: a name and its request.
type NetworkCommand struct {
	Name string
	HTTP profile.HTTP
}

// CheckRequest says, in the user's terms, what is wrong with a command's
// request, if anything.
func CheckRequest(command string, h profile.HTTP) error {
	if strings.TrimSpace(h.URL) == "" {
		return fmt.Errorf("%s", i18n.T("%s: the request needs an address, such as http://192.168.1.50:8080/lights/on.", command))
	}
	r, err := h.Request()
	if err != nil {
		return fmt.Errorf("%s", i18n.T("%s: the address is a plain http:// address with the device's IPv4 address, such as http://192.168.1.50:8080/lights/on: the hub cannot look names up or use https.", command))
	}
	if _, err := r.Data(); err != nil {
		return fmt.Errorf("%s", i18n.T("%s: each header is a name, a colon and a value, on one line (the hub writes Host, Content-Type and Content-Length itself).", command))
	}
	return nil
}

// tidyRequest is h with its fields trimmed and empty headers left out.
func tidyRequest(h profile.HTTP) profile.HTTP {
	h.URL = strings.TrimSpace(h.URL)
	h.ContentType = strings.TrimSpace(h.ContentType)
	if h.Method == "GET" {
		h.Method = ""
	}
	var headers []string
	for _, l := range h.Headers {
		if l = strings.TrimSpace(l); l != "" {
			headers = append(headers, l)
		}
	}
	h.Headers = headers
	return h
}

// Build makes the network device the spec describes. Commands whose name
// is a key of the remote's are set on that key, as in Spec.Build.
func (s NetworkSpec) Build() (*profile.Device, error) {
	name := strings.TrimSpace(s.Name)
	if name == "" {
		return nil, &FieldError{Field: "name", Msg: i18n.T("The device needs a name.")}
	}
	d := &profile.Device{Name: name, Brand: strings.TrimSpace(s.Brand), Category: networkCategory, Buttons: profile.Buttons{}}
	seen := map[string]bool{}
	for i, c := range s.Commands {
		cname := strings.TrimSpace(c.Name)
		if cname == "" && strings.TrimSpace(c.HTTP.URL) == "" {
			continue // an empty row
		}
		if cname == "" {
			return nil, &FieldError{Field: "command", Index: i, Msg: i18n.T("Command %d needs a name.", i+1)}
		}
		if seen[strings.ToLower(cname)] {
			return nil, &FieldError{Field: "command", Index: i, Msg: i18n.T("There are two commands named %q.", cname)}
		}
		seen[strings.ToLower(cname)] = true
		h := tidyRequest(c.HTTP)
		if err := CheckRequest(cname, h); err != nil {
			return nil, &FieldError{Field: "command", Index: i, Msg: err.Error()}
		}
		d.Commands = append(d.Commands, profile.Command{Name: cname, HTTP: &h})
		if key, ok := KeyFor(cname); ok {
			if _, taken := d.Buttons[key]; !taken {
				d.Buttons[key] = profile.Key{Short: cname}
			}
		}
	}
	if len(d.Commands) == 0 {
		return nil, &FieldError{Field: "commands", Msg: i18n.T("Add at least one command.")}
	}
	if len(d.Buttons) == 0 {
		d.Buttons = nil
	}
	on, off := strings.TrimSpace(s.PowerOn), strings.TrimSpace(s.PowerOff)
	if on != "" || off != "" {
		if on == "" || off == "" {
			return nil, &FieldError{Field: "power", Msg: i18n.T("Pick both the command that turns the device on and the one that turns it off (the same one if it has a single power key).")}
		}
		for _, n := range []string{on, off} {
			if !seen[strings.ToLower(n)] {
				return nil, &FieldError{Field: "power", Msg: i18n.T("There is no command named %q.", n)}
			}
		}
		d.Power = &profile.Power{On: on, Off: off}
	}
	if err := d.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("the device is not valid"), err)
	}
	return d, nil
}

// IsNetwork reports whether the device's commands are HTTP requests.
func (w *Workspace) IsNetwork(device string) bool {
	d := w.Config.Device(device)
	if d == nil {
		return false
	}
	for _, c := range d.Commands {
		if c.HTTP != nil {
			return true
		}
	}
	return false
}
