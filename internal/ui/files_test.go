package ui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/netip"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/demo"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/hubsim"
	"github.com/brunoga/sofabaton/irsource"
)

// memFile is a file the test pickers save to and open.
type memFile struct{ bytes.Buffer }

func (*memFile) Close() error { return nil }

// filePickers make u save to and open f, recording the name offered.
func filePickers(u *App, f *memFile, offered *string) {
	u.pickers = pickers{
		open: func(_ fyne.Window, _ []string, done func(io.ReadCloser, string, error)) {
			done(io.NopCloser(bytes.NewReader(f.Bytes())), *offered, nil)
		},
		save: func(_ fyne.Window, name string, done func(io.WriteCloser, error)) {
			*offered = name
			f.Reset()
			done(f, nil)
		},
	}
}

// A hub saved with changes opens without the hub, can be changed more,
// and is applied later to its hub, reviewed first.
func TestFiles(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	u.now = func() time.Time { return time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC) }
	var f memFile
	var name string
	filePickers(u, &f, &name)

	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	if err := u.work.RemoveActivity("Play PC"); err != nil {
		t.Fatal(err)
	}
	u.backToBrowse()
	hubAction(t, u, "Save to a file…")
	if name != "X2 HUB 2026-11-02.armrest" || !strings.Contains(texts(w), "Saved the hub and your changes to a file.") {
		t.Fatalf("saved as %q:\n%s", name, texts(w))
	}
	hubAction(t, u, "Disconnect")

	menuItem(t, u, "Open a saved file…")
	got := texts(w)
	if !strings.Contains(got, "From the file X2 HUB 2026-11-02.armrest, saved just now") || listed(u, "Play PC") || !u.browse.changes.Visible() || u.client != nil {
		t.Fatalf("opened:\n%s", got)
	}
	assertScreens(t, w, "file")
	// Changed more, without the hub.
	if err := u.work.RemoveActivity("Play Switch 2"); err != nil {
		t.Fatal(err)
	}
	u.backToBrowse()

	test.Tap(button(t, w, "Review changes"))
	got = texts(w)
	if !strings.Contains(got, "if it is as the file has it") || !strings.Contains(got, "Delete activity Play PC") || !strings.Contains(got, "Delete activity Play Switch 2") {
		t.Fatalf("review without the hub:\n%s", got)
	}
	test.Tap(button(t, w, "Connect to the hub and apply"))
	got = texts(w)
	if !strings.Contains(got, "Applying X2 HUB 2026-11-02.armrest") || !strings.Contains(got, "Connect to X2 HUB to apply") {
		t.Fatalf("not asked for the hub:\n%s", got)
	}
	assertScreens(t, w, "file-hubs")
	// Back to the file, and again.
	test.Tap(button(t, w, "Back to the file"))
	if listed(u, "Play Switch 2") || u.client != nil {
		t.Fatal("not back at the file with its changes")
	}
	test.Tap(button(t, w, "Review changes"))
	test.Tap(button(t, w, "Connect to the hub and apply"))

	// The hub is read, and the changes reviewed against it.
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	got = texts(w)
	if u.file != nil || u.client == nil || !strings.Contains(got, "Applying makes these changes on X2 HUB, in") {
		t.Fatalf("after connecting:\n%s", got)
	}
	test.Tap(button(t, w, "Apply to the hub"))
	if got := texts(w); !strings.Contains(got, "Your changes are on the hub.") || listed(u, "Play PC") || listed(u, "Play Switch 2") || u.work.Changed() {
		t.Fatalf("after applying:\n%s", got)
	}
}

// A file applied to another hub restores it there, once the user agrees.
func TestFileOtherHub(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	var f memFile
	name := "other.armrest"
	filePickers(u, &f, &name)
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	if err := u.work.RemoveActivity("Play PC"); err != nil {
		t.Fatal(err)
	}
	d := *u.work.Dump
	d.Info.ID, d.Info.Name = "0123456789ab", "Bedroom"
	u.work.Dump = &d
	if err := u.work.Save(&f, time.Time{}); err != nil {
		t.Fatal(err)
	}
	hubAction(t, u, "Disconnect")
	menuItem(t, u, "Open a saved file…")
	hubAction(t, u, "Restore to a hub…")
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	if got := texts(w); !strings.Contains(got, "Restore onto another hub") || !strings.Contains(got, "saved from another hub, Bedroom") {
		t.Fatalf("not asked:\n%s", got)
	}
	test.Tap(button(t, w, "Review"))
	got := texts(w)
	if !strings.Contains(got, "Delete activity Play PC") || u.file != nil {
		t.Fatalf("the restore's review:\n%s", got)
	}
	test.Tap(button(t, w, "Apply to the hub"))
	if got := texts(w); !strings.Contains(got, "Your changes are on the hub.") || listed(u, "Play PC") {
		t.Fatalf("after restoring:\n%s", got)
	}
}

// A backup restored onto its hub, changed since, undoes the change; or
// Cancel goes back to the file.
func TestBackup(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	d, _ := demo.Dump()
	m, err := d.Model()
	if err != nil {
		t.Fatal(err)
	}
	sim := hubsim.FromModel(m) // one hub, kept across connections
	u.back.Dial = func(ctx context.Context, _ netip.Addr) (*hub.Client, error) {
		return hub.Open(ctx, hubsim.Pipe(sim), hub.Options{})
	}
	var f memFile
	var name string
	filePickers(u, &f, &name)
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	hubAction(t, u, "Save to a file…")
	if !strings.Contains(texts(w), "Saved the hub to a file.") {
		t.Fatalf("saving:\n%s", texts(w))
	}
	// The hub changes.
	if err := u.work.RemoveActivity("Play PC"); err != nil {
		t.Fatal(err)
	}
	u.backToBrowse()
	test.Tap(button(t, w, "Review changes"))
	test.Tap(button(t, w, "Apply to the hub"))
	if listed(u, "Play PC") {
		t.Fatal("not deleted")
	}
	hubAction(t, u, "Disconnect")

	menuItem(t, u, "Open a saved file…")
	if !listed(u, "Play PC") || u.browse.changes.Visible() {
		t.Fatal("the backup")
	}
	restore := func() {
		hubAction(t, u, "Restore to a hub…")
		test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
		if got := texts(w); !strings.Contains(got, "The hub changed since") {
			t.Fatalf("not asked:\n%s", got)
		}
	}
	restore()
	test.Tap(button(t, w, "Cancel"))
	if u.client != nil || u.file == nil || !listed(u, "Play PC") {
		t.Fatal("cancel did not go back to the file")
	}
	restore()
	test.Tap(button(t, w, "Review"))
	if got := texts(w); !strings.Contains(got, "Add activity Play PC") {
		t.Fatalf("the restore's review:\n%s", got)
	}
	test.Tap(button(t, w, "Apply to the hub"))
	if got := texts(w); !strings.Contains(got, "Your changes are on the hub.") || !listed(u, "Play PC") || u.work.Changed() {
		t.Fatalf("after restoring:\n%s", got)
	}
}

// A device is added from a Flipper file of its codes: its keys set, what
// the hub cannot send left out, and said.
func TestImportIRFile(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	var f memFile
	name := "Living_Room_TV.ir"
	filePickers(u, &f, &name)
	f.WriteString(`Filetype: IR signals file
Version: 1
#
name: Power
type: parsed
protocol: NEC
address: 04 00 00 00
command: 08 00 00 00
#
name: Vol_up
type: parsed
protocol: NEC
address: 04 00 00 00
command: 02 00 00 00
#
name: Netflix
type: raw
frequency: 38000
duty_cycle: 0.330000
data: 9000 4500
`)
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "From a file…"))
	got := texts(w)
	for _, want := range []string{"3 commands, 2 of them on the remote's keys. Power: Power."} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	assertScreens(t, w, "import-ir")
	test.Tap(button(t, w, "Add"))
	d := u.work.Config.Device("Living Room TV")
	if d == nil || d.Buttons["Vol+"].Short != "Volume Up" {
		t.Fatalf("added %+v", d)
	}
	if !strings.Contains(texts(w), "Added Living Room TV.") {
		t.Errorf("after adding:\n%s", texts(w))
	}
}

// A device's codes are saved to a file in the format picked: those the
// format cannot hold are left out, and said.
func TestExportCodes(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	var f memFile
	name := "Living_Room_TV.ir"
	filePickers(u, &f, &name)
	f.WriteString(`Filetype: IR signals file
Version: 1
#
name: Power
type: parsed
protocol: NEC
address: 04 00 00 00
command: 08 00 00 00
#
name: Netflix
type: raw
frequency: 38000
duty_cycle: 0.330000
data: 9000 4500 560
`)
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Add a device…")
	test.Tap(button(t, w, "From a file…"))
	test.Tap(button(t, w, "Add"))
	pickRow(t, u, "Living Room TV")
	test.Tap(button(t, w, "Details"))
	test.Tap(button(t, w, "Save codes to a file…"))
	walk(w, func(o fyne.CanvasObject) {
		if s, ok := o.(*widget.Select); ok {
			s.SetSelected("irdb (.csv)")
		}
	})
	test.Tap(button(t, w, "Save…"))
	if name != "Living Room TV.csv" {
		t.Errorf("offered %q", name)
	}
	if got := f.String(); got != "functionname,protocol,device,subdevice,function\nPower,NEC1,4,251,8\n" {
		t.Errorf("file:\n%s", got)
	}
	got := texts(w)
	for _, want := range []string{"Saved 1 code of Living Room TV to a file (irdb).", "Netflix: irdb files hold protocols' codes, not timings"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// Searching adds online results after the templates; picking one gets
// its file and adds the device once confirmed.
func TestOnlineSearch(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	entry := irsource.Entry{Source: irsource.Flipper, Kind: "TVs", Brand: "Acme", Model: "Acme 55X1", Path: "TVs/Acme/Acme_55X1.ir"}
	u.irIndex = func(_ context.Context, s irsource.Source) ([]irsource.Entry, error) {
		if s == irsource.Flipper {
			return []irsource.Entry{entry}, nil
		}
		return nil, errors.New("offline")
	}
	u.irFetch = func(_ context.Context, e irsource.Entry) ([]byte, error) {
		return []byte("Filetype: IR signals file\nVersion: 1\nname: Power\ntype: parsed\nprotocol: NEC\naddress: 04 00 00 00\ncommand: 08 00 00 00\n"), nil
	}
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	pickRow(t, u, "Add a device…")
	var search *widget.Entry
	walk(w, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok && search == nil {
			search = e
		}
	})
	// Browsing: a type, then a maker, with no words.
	var kind, maker *widget.Select
	walk(w, func(o fyne.CanvasObject) {
		if s, ok := o.(*widget.Select); ok {
			if kind == nil {
				kind = s
			} else {
				maker = s
			}
		}
	})
	if !maker.Disabled() {
		t.Error("maker enabled before a type is picked")
	}
	kind.SetSelected("TV")
	if maker.Disabled() || len(maker.Options) != 2 || maker.Options[1] != "Acme" {
		t.Fatalf("TV's makers %v", maker.Options)
	}
	maker.SetSelected("Acme")
	if l := list(t, w); l.Length() != 1 {
		t.Fatalf("Acme TVs: %d", l.Length())
	}
	kind.SetSelected("Projector")
	if l := list(t, w); l.Length() != 0 {
		t.Fatalf("Acme projectors: %d", l.Length())
	}
	kind.SetSelected("Any type")

	test.Type(search, "acme")
	got := texts(w)
	if !strings.Contains(got, "TVs · Flipper-IRDB") {
		t.Fatalf("no online result in:\n%s", got)
	}
	assertScreens(t, w, "online-search")
	l := list(t, w)
	l.Select(0)
	test.Tap(button(t, w, "Add"))
	if d := u.work.Config.Device("Acme 55X1"); d == nil || d.Buttons["Power"].Short != "Power" {
		t.Fatalf("added %+v", d)
	}
}
