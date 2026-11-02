package cache

import (
	"testing"
	"time"

	"github.com/brunoga/armrest/internal/demo"
)

func TestCache(t *testing.T) {
	c := Cache{Dir: t.TempDir()}
	d, err := demo.Dump()
	if err != nil {
		t.Fatal(err)
	}
	if c.Load(d.Info.ID) != nil {
		t.Fatal("an entry before saving")
	}
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	if err := c.Save(Entry{Saved: now, Dump: d, Edits: []byte("devices: []\n")}); err != nil {
		t.Fatal(err)
	}
	e := c.Load(d.Info.ID)
	if e == nil || !e.Saved.Equal(now) || len(e.Dump.Entities) != len(d.Entities) || string(e.Edits) != "devices: []\n" {
		t.Fatalf("loaded %+v", e)
	}
	if _, err := e.Dump.Model(); err != nil {
		t.Errorf("the cached dump does not load: %v", err)
	}
	if err := c.Forget(d.Info.ID); err != nil || c.Load(d.Info.ID) != nil {
		t.Errorf("forget: %v", err)
	}
	if c.Load("../x") != nil || c.Forget("../x") == nil {
		t.Error("a path as a hub id")
	}
}
