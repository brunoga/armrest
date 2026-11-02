package ui

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/brunoga/armrest/internal/backend"
)

// memFile is a file the test pickers save to and open.
type memFile struct{ bytes.Buffer }

func (*memFile) Close() error { return nil }

// filePickers make u save to and open f, recording the name offered.
func filePickers(u *App, f *memFile, offered *string) {
	u.pickers = pickers{
		open: func(_ fyne.Window, done func(io.ReadCloser, string, error)) {
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
	menu(t, u, "Save to a file…")
	if name != "X2 HUB 2026-11-02.armrest" || !strings.Contains(texts(w), "Saved the hub and your changes to a file.") {
		t.Fatalf("saved as %q:\n%s", name, texts(w))
	}
	menu(t, u, "Disconnect")

	test.Tap(button(t, w, "Open a file…"))
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

// A file applied to another hub is refused, and stays open.
func TestFileOtherHub(t *testing.T) {
	u, w := newApp(t, backend.Hub{Name: "Lounge", Model: "X2", Firmware: 9, Addr: lounge})
	var f memFile
	name := "other.armrest"
	filePickers(u, &f, &name)
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	if err := u.work.RemoveActivity("Play PC"); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	d := *u.work.Dump
	d.Info.ID = "0123456789ab"
	u.work.Dump = &d
	if err := u.work.Save(&b, time.Time{}); err != nil {
		t.Fatal(err)
	}
	f.Write(b.Bytes())
	menu(t, u, "Disconnect")
	test.Tap(button(t, w, "Open a file…"))
	test.Tap(button(t, w, "Review changes"))
	test.Tap(button(t, w, "Connect to the hub and apply"))
	test.Tap(button(t, w, "Lounge (X2, 192.168.1.20)"))
	if got := texts(w); !strings.Contains(got, "is not the hub other.armrest is from") || u.file == nil || u.client != nil {
		t.Fatalf("applied to another hub:\n%s", got)
	}
}
