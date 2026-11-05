package workspace

import (
	"errors"
	"strings"
	"testing"

	"github.com/brunoga/sofabaton/profile"
)

func TestNetworkBuild(t *testing.T) {
	s := NetworkSpec{
		Name: " Lights ",
		Commands: []NetworkCommand{
			{"Power On", profile.HTTP{Method: "POST", URL: " http://192.168.1.50:8123/api/webhook/on ", Headers: []string{"", " X-A: b ", ""}, ContentType: "application/json", Body: "{}"}},
			{"", profile.HTTP{}},
			{"OK", profile.HTTP{Method: "GET", URL: "http://192.168.1.50/toggle"}},
		},
		PowerOn: "Power On", PowerOff: "OK",
	}
	d, err := s.Build()
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Lights" || d.Category != networkCategory || len(d.Commands) != 2 || d.Buttons["OK"].Short != "OK" || d.Power == nil {
		t.Fatalf("built %+v", d)
	}
	if h := d.Commands[0].HTTP; h.URL != "http://192.168.1.50:8123/api/webhook/on" || len(h.Headers) != 1 || h.Headers[0] != "X-A: b" {
		t.Errorf("request %+v", h)
	}
	if d.Commands[1].HTTP.Method != "" {
		t.Errorf("GET kept: %+v", d.Commands[1].HTTP)
	}

	for _, bad := range []struct {
		h    profile.HTTP
		want string
	}{
		{profile.HTTP{URL: ""}, "needs an address"},
		{profile.HTTP{URL: "https://192.168.1.50/x"}, "plain http://"},
		{profile.HTTP{URL: "http://lights.local/x"}, "plain http://"},
		{profile.HTTP{URL: "http://192.168.1.50/x", Headers: []string{"Host: x"}}, "each header"},
	} {
		_, err := NetworkSpec{Name: "X", Commands: []NetworkCommand{{"A", bad.h}}}.Build()
		var fe *FieldError
		if !errors.As(err, &fe) || fe.Field != "command" || !strings.Contains(fe.Msg, bad.want) {
			t.Errorf("%+v: %v", bad.h, err)
		}
	}
}

// A network device's requests are edited like codes, and a new command
// of it is a request.
func TestEditNetworkDevice(t *testing.T) {
	w := demoWorkspace(t)
	d, err := NetworkSpec{Name: "Lights", Commands: []NetworkCommand{{"On", profile.HTTP{URL: "http://192.168.1.50/on"}}}}.Build()
	if err != nil {
		t.Fatal(err)
	}
	if err := w.AddDevice(d); err != nil {
		t.Fatal(err)
	}
	if !w.IsNetwork("Lights") || !w.CanAddCommands("Lights") || w.IsNetwork("Receiver") {
		t.Fatal("IsNetwork, CanAddCommands")
	}
	e, err := w.EditDevice("Lights")
	if err != nil || len(e.Commands) != 1 || e.Commands[0].HTTP == nil || e.Commands[0].Fixed {
		t.Fatalf("edit %+v, %v", e, err)
	}
	e.Commands[0].HTTP.URL = "http://192.168.1.50/toggle"
	e.Commands = append(e.Commands, CommandEdit{Name: "Off", HTTP: &profile.HTTP{URL: "http://lights/off"}})
	var fe *FieldError
	if err := w.SaveDevice(e); !errors.As(err, &fe) || fe.Index != 1 {
		t.Fatalf("a name saved: %v", err)
	}
	if got := w.Config.Device("Lights").Commands[0].HTTP.URL; got != "http://192.168.1.50/on" {
		t.Fatalf("changed by a failed save: %s", got)
	}
	e.Commands[1].HTTP.URL = "http://192.168.1.50/off"
	if err := w.SaveDevice(e); err != nil {
		t.Fatal(err)
	}
	if c := w.Config.Device("Lights").Commands; len(c) != 2 || c[0].HTTP.URL != "http://192.168.1.50/toggle" || c[1].HTTP.URL != "http://192.168.1.50/off" {
		t.Fatalf("saved %+v", c)
	}
}
