package ui

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/armrest/internal/workspace"
)

// learnWait is how long learning waits for a key: the hub keeps
// listening, so this only bounds a wait nobody ends.
var learnWait = 2 * time.Minute

// learn has the hub learn a command from the device's own remote, and
// adds it to the editor, to name; saving keeps it, as other edits.
func (ed *deviceEditor) learn() {
	a := ed.app
	c := a.client
	if c == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), learnWait)
	alive := newPulse()
	again := wrapped(i18n.T("The hub could not read that. Press the key once again, briefly."))
	again.Importance = widget.WarningImportance
	again.Hide()
	d := dialog.NewCustom(i18n.T("Learning a command"), i18n.T("Cancel"), container.NewVBox(
		wrapped(i18n.T("Point the device's own remote at the hub, close to it, and press the key once, briefly: a key held down is not learned.")),
		alive, again), a.win)
	d.SetOnClosed(cancel)
	d.Show()
	alive.Start()
	a.async(func() {
		data, err := a.learn(ctx, c, func() { fyne.Do(again.Show) })
		fyne.Do(func() {
			alive.Stop()
			if errors.Is(err, context.Canceled) {
				return // closed
			}
			d.Hide()
			if errors.Is(err, context.DeadlineExceeded) {
				dialog.ShowInformation(i18n.T("Nothing learned"), i18n.T("The hub learned no key in %s. Try again, with the remote close to the hub.", took(learnWait)), a.win)
				return
			}
			if err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			ed.addCmd(workspace.CommandEdit{Name: ed.learnedName(), Fixed: true, Data: hex.EncodeToString(data)})
			ed.refreshPickers()
			r := ed.cmds[len(ed.cmds)-1]
			a.win.Canvas().Focus(r.name)
		})
	})
}

// learnedName is a name for a learned command no other command has, for
// the user to change.
func (ed *deviceEditor) learnedName() string {
	used := map[string]bool{}
	for _, r := range ed.cmds {
		used[strings.ToLower(strings.TrimSpace(r.name.Text))] = true
	}
	for i := 1; ; i++ {
		n := i18n.T("Learned %d", i)
		if !used[strings.ToLower(n)] {
			return n
		}
	}
}
