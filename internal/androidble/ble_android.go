//go:build android

package androidble

/*
#cgo LDFLAGS: -landroid -llog
#include <jni.h>
#include <stdint.h>
#include <stdlib.h>

const char *ble_init(uintptr_t javaVM, uintptr_t activity, void *dex, int dexLen);
int ble_permission(void);
uintptr_t ble_connect(jlong h, const char *mac);
int ble_discover(uintptr_t gatt);
void ble_tune(uintptr_t gatt);
int ble_mtu(uintptr_t gatt, int mtu);
uintptr_t ble_characteristic(uintptr_t gatt, const char *svc, const char *chr);
int ble_notify(uintptr_t gatt, uintptr_t chr);
int ble_write(uintptr_t gatt, uintptr_t chr, void *value, int n);
void ble_close(uintptr_t gatt, uintptr_t chr);
*/
import "C"

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
	"unsafe"

	"fyne.io/fyne/v2/driver"

	"github.com/brunoga/sofabaton/sifli"
)

//go:embed bridge.dex
var bridgeDex []byte

var (
	initOnce sync.Once
	initErr  error
	// dexMem keeps the dex where the Java side reads it from.
	dexMem unsafe.Pointer
)

// start loads the bridge once, from the app's Java context.
func start() error {
	initOnce.Do(func() {
		initErr = driver.RunNative(func(c any) error {
			ac, ok := c.(*driver.AndroidContext)
			if !ok {
				return errors.New("bluetooth: no Android context")
			}
			dexMem = C.CBytes(bridgeDex) // the in-memory loader keeps reading it
			if msg := C.ble_init(C.uintptr_t(ac.VM), C.uintptr_t(ac.Ctx), dexMem, C.int(len(bridgeDex))); msg != nil {
				return fmt.Errorf("bluetooth: %s", C.GoString(msg))
			}
			return nil
		})
	})
	return initErr
}

// Supported reports whether this build can update a remote: Android's.
const Supported = true

// ErrPermission is Dial's error until the user lets the app use
// Bluetooth (Android 12 and later ask once).
var ErrPermission = errors.New("bluetooth: allow Armrest to connect to nearby devices, then try again")

// The connections, by handle, for the callbacks.
var (
	connsMu sync.Mutex
	conns   = map[int64]*conn{}
	next    int64
)

type conn struct {
	handle int64
	gatt   C.uintptr_t
	chr    C.uintptr_t
	mtu    int
	// events carry the callbacks' results.
	state   chan int // connection state: 2 connected, 0 disconnected
	ops     chan int // status of discovery, MTU, descriptor and writes
	mtuSeen chan int
	notes   chan []byte
	// closed is closed once the link is down: dropped, or closed.
	closed    chan struct{}
	dropOnce  sync.Once
	closeOnce sync.Once
}

// drop marks the link down.
func (c *conn) drop() { c.dropOnce.Do(func() { close(c.closed) }) }

// How long connecting and each operation may take.
const (
	connectTimeout = 30 * time.Second
	opTimeout      = 10 * time.Second
)

// Dial connects to the device at addr, as the app does: directly by its
// address, then the update service's notifications and a large MTU.
func Dial(addr net.HardwareAddr) sifli.Dial {
	return func(ctx context.Context) (sifli.Conn, error) {
		if err := start(); err != nil {
			return nil, err
		}
		if C.ble_permission() == 0 {
			return nil, ErrPermission
		}
		c := &conn{
			state: make(chan int, 8), ops: make(chan int, 8), mtuSeen: make(chan int, 2),
			notes: make(chan []byte, 512), closed: make(chan struct{}),
		}
		connsMu.Lock()
		next++
		c.handle = next
		conns[c.handle] = c
		connsMu.Unlock()
		mac := C.CString(strings.ToUpper(addr.String()))
		defer C.free(unsafe.Pointer(mac))
		c.gatt = C.ble_connect(C.jlong(c.handle), mac)
		if c.gatt == 0 {
			c.forget()
			return nil, fmt.Errorf("bluetooth: cannot connect to %v (is Bluetooth on?)", addr)
		}
		if err := c.open(ctx); err != nil {
			_ = c.Close()
			return nil, err
		}
		return c, nil
	}
}

func (c *conn) open(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	select {
	case s := <-c.state:
		if s != 2 {
			return errors.New("bluetooth: the remote did not connect: wake it (press a key) and keep it near")
		}
	case <-cctx.Done():
		return errors.New("bluetooth: the remote did not connect in time: wake it (press a key) and keep it near")
	}
	C.ble_tune(c.gatt)
	time.Sleep(time.Second) // as the app waits for the link to settle
	if C.ble_discover(c.gatt) == 0 {
		return errors.New("bluetooth: cannot look for the remote's services")
	}
	if err := c.await(ctx, c.ops, "finding the remote's services"); err != nil {
		return err
	}
	svc, chr := C.CString(sifli.ServiceUUID), C.CString(sifli.DataUUID)
	defer C.free(unsafe.Pointer(svc))
	defer C.free(unsafe.Pointer(chr))
	if c.chr = C.ble_characteristic(c.gatt, svc, chr); c.chr == 0 {
		return errors.New("bluetooth: the remote has no update service")
	}
	if C.ble_notify(c.gatt, c.chr) == 0 {
		return errors.New("bluetooth: cannot turn the remote's notifications on")
	}
	if err := c.await(ctx, c.ops, "turning the remote's notifications on"); err != nil {
		return err
	}
	c.mtu = 23
	if C.ble_mtu(c.gatt, 517) != 0 {
		select {
		case m := <-c.mtuSeen:
			c.mtu = m
		case <-time.After(opTimeout):
		}
	}
	return nil
}

// await waits for an operation's status on ch: 0 is success.
func (c *conn) await(ctx context.Context, ch chan int, what string) error {
	select {
	case st := <-ch:
		if st != 0 {
			return fmt.Errorf("bluetooth: %s: status %d", what, st)
		}
		return nil
	case <-c.closed:
		return fmt.Errorf("bluetooth: %s: the connection closed", what)
	case <-time.After(opTimeout):
		return fmt.Errorf("bluetooth: %s: no answer", what)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Write writes without response, one write at a time: Android takes the
// next once the last is done.
func (c *conn) Write(v []byte) error {
	select {
	case <-c.closed:
		return errors.New("bluetooth: the connection closed")
	default:
	}
	for try := 0; ; try++ {
		st := C.ble_write(c.gatt, c.chr, unsafe.Pointer(&v[0]), C.int(len(v)))
		if st == 0 {
			break
		}
		// Busy (the stack's queue is full): try again shortly.
		if try > 100 {
			return fmt.Errorf("bluetooth: writing: status %d", st)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return c.await(context.Background(), c.ops, "writing")
}

func (c *conn) Notification(ctx context.Context) ([]byte, error) {
	select {
	case n := <-c.notes:
		return n, nil
	case <-c.closed:
		return nil, errors.New("bluetooth: the connection closed")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (c *conn) MTU() int { return c.mtu }

func (c *conn) Close() error {
	c.closeOnce.Do(func() {
		c.drop()
		C.ble_close(c.gatt, c.chr)
		c.forget()
	})
	return nil
}

func (c *conn) forget() {
	connsMu.Lock()
	delete(conns, c.handle)
	connsMu.Unlock()
}

func lookup(h C.jlong) *conn {
	connsMu.Lock()
	defer connsMu.Unlock()
	return conns[int64(h)]
}

// send hands v to a callback's channel without blocking the Binder
// thread the callback runs on.
func send[T any](ch chan T, v T) {
	select {
	case ch <- v:
	default:
	}
}

//export goBleState
func goBleState(h C.jlong, status, state C.jint) {
	if c := lookup(h); c != nil {
		if state != 2 || status != 0 {
			send(c.state, 0)
			c.drop() // Close still lets go of it
			return
		}
		send(c.state, 2)
	}
}

//export goBleServices
func goBleServices(h C.jlong, status C.jint) {
	if c := lookup(h); c != nil {
		send(c.ops, int(status))
	}
}

//export goBleMtu
func goBleMtu(h C.jlong, mtu, status C.jint) {
	if c := lookup(h); c != nil && status == 0 {
		send(c.mtuSeen, int(mtu))
	}
}

//export goBleWrite
func goBleWrite(h C.jlong, status C.jint) {
	if c := lookup(h); c != nil {
		send(c.ops, int(status))
	}
}

//export goBleDescriptor
func goBleDescriptor(h C.jlong, status C.jint) {
	if c := lookup(h); c != nil {
		send(c.ops, int(status))
	}
}

//export goBleNotify
func goBleNotify(h C.jlong, b *C.jbyte, n C.jsize) {
	if c := lookup(h); c != nil {
		send(c.notes, C.GoBytes(unsafe.Pointer(b), C.int(n)))
	}
}
