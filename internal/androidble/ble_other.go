//go:build !android

// Package androidble connects to Bluetooth Low Energy devices on Android,
// through Android's own Bluetooth APIs, for updating the remote's
// firmware (sifli). Android calls back a Java class, BleBridge (java/),
// compiled to bridge.dex (make bridge-dex) and loaded at run time.
package androidble

import (
	"context"
	"errors"
	"net"

	"github.com/brunoga/sofabaton/sifli"
)

// Supported reports whether this build can update a remote: Android's.
const Supported = false

// ErrPermission is Dial's error until the user lets the app use Bluetooth.
var ErrPermission = errors.New("bluetooth: not allowed")

// Dial is Android's only.
func Dial(net.HardwareAddr) sifli.Dial {
	return func(context.Context) (sifli.Conn, error) {
		return nil, errors.New("updating the remote needs Armrest on Android")
	}
}
