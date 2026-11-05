package workspace

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/brunoga/armrest/internal/demo"
	"github.com/brunoga/sofabaton/profile"
)

// A new order and a new Bluetooth device are planned and applied: the
// device paired, saying so, and the order as edited.
func TestOrderAndBluetooth(t *testing.T) {
	ctx := context.Background()
	c, err := demo.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	w, err := Read(ctx, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, a := range w.Config.Activities {
		names = append(names, a.Name)
	}
	slices.Reverse(names)
	if err := w.OrderActivities(names[1:]); err == nil {
		t.Error("an order without every activity")
	}
	if err := w.OrderActivities(append(names[:len(names)-1:len(names)-1], names[0])); err == nil {
		t.Error("an order naming one twice")
	}
	if err := w.OrderActivities(names); err != nil {
		t.Fatal(err)
	}
	box, err := profile.Load(strings.NewReader("device: Box\nbluetooth: {profile: d272ab7af5404b549eb260d367112165}\ncommands:\n  - {name: Play, data: 0c00b0}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.AddDevice(box); err != nil {
		t.Fatal(err)
	}
	p, err := w.Plan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(p); !strings.Contains(got, "Add device Box") || !strings.Contains(got, "Change the order of the activities") || strings.Contains(got, "order of the devices") {
		t.Fatalf("plan:\n%s", got)
	}
	var steps []string
	again := applied(t, w, c, p.Writes, func(_, _ int, step string) { steps = append(steps, step) }, true)
	if !slices.Contains(steps, "Pair Box with the hub now, from its Bluetooth settings or its remote's pairing.") {
		t.Errorf("steps:\n%s", strings.Join(steps, "\n"))
	}
	var got []string
	for _, a := range again.Config.Activities {
		got = append(got, a.Name)
	}
	if !slices.Equal(got, names) || again.Config.Device("Box") == nil || again.Config.Device("Box").Bluetooth == nil {
		t.Fatalf("after applying: %v, %+v", got, again.Config.Device("Box"))
	}
	if p, err := again.Plan(ctx); err != nil || len(p.Changes) != 0 {
		t.Errorf("after applying: %v\n%s", err, titles(p))
	}
}
