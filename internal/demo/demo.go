// Package demo is a simulated hub, for trying the app without a hub and
// for its tests. It serves a dump of a real X2's configuration.
package demo

import (
	"bytes"
	"context"
	_ "embed"

	"github.com/brunoga/sofabaton/dump"
	"github.com/brunoga/sofabaton/hub"
)

//go:embed demo.json
var data []byte

// Open returns a client of a fresh simulated hub. Writes change only that
// simulation.
func Open(ctx context.Context) (*hub.Client, error) {
	d, err := Dump()
	if err != nil {
		return nil, err
	}
	return d.Open(ctx)
}

// Dump is the demo hub's dump.
func Dump() (*dump.File, error) {
	return dump.Load(bytes.NewReader(data))
}
