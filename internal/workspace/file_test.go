package workspace

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/brunoga/armrest/internal/demo"
)

// A saved workspace opens with its edits; a hub's dump opens as it is.
func TestSaveOpen(t *testing.T) {
	ctx := context.Background()
	w := demoWorkspace(t)
	if err := w.RemoveActivity("Play PC"); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	var b bytes.Buffer
	if err := w.Save(&b, now); err != nil {
		t.Fatal(err)
	}
	o, saved, err := Open(ctx, bytes.NewReader(b.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Equal(now) || !o.Same(w) || !o.Changed() || o.Config.Activity("Play PC") != nil {
		t.Fatal("the edits did not come back")
	}

	d, _ := demo.Dump()
	b.Reset()
	if err := d.Write(&b); err != nil {
		t.Fatal(err)
	}
	o, saved, err = Open(ctx, &b)
	if err != nil || !saved.IsZero() || o.Changed() || len(o.Config.Devices) != 3 {
		t.Fatalf("a dump: %v", err)
	}

	for _, bad := range []string{"", "hello", "{}", `{"armrest": 1}`, `{"armrest": 99, "dump": {}}`} {
		if _, _, err := Open(ctx, strings.NewReader(bad)); err == nil {
			t.Errorf("opened %q", bad)
		}
	}
}
