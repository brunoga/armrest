package ui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/brunoga/armrest/internal/backend"
	"github.com/brunoga/armrest/internal/i18n"
	"github.com/brunoga/armrest/internal/workspace"
	"github.com/brunoga/sofabaton/hub"
	"github.com/brunoga/sofabaton/profile"
)

// The browse screen's tabs, in order.
const (
	hubTab = iota
	devicesTab
	activitiesTab
	settingsTab
)

// browseScreen shows the hub in tabs: the hub itself (what it is, saving
// it, leaving it), its devices and its activities. Undo, Redo and the
// changes to review are on every tab.
type browseScreen struct {
	app  *App
	hub  backend.Hub
	tabs *container.AppTabs
	// tabNames are the tabs' names, which the bar shows as its title
	// (title): phones show the tabs' icons only.
	tabNames   []string
	title      *widget.Label
	barBack    *widget.Button
	barBackBox *fyne.Container
	barTop     fyne.CanvasObject
	// hubName, hubKind and hubInfo say what the hub is, on the hub's
	// tab, over its activities' and devices' tiles.
	hubName                    *widget.Label
	hubKind                    *chip
	hubInfo                    *widget.Label
	activityTiles, deviceTiles *fyne.Container
	// The firmware card: the check's words, its buttons, the remotes'.
	fwStatus            *widget.Label
	fwActions           *fyne.Container
	fwRemotes           *fyne.Container
	devices, activities *browseList

	status *widget.Label
	busy   *pulse
	// failed is set while the status says a key, macro or favorite
	// failed.
	failed bool
	// retry holds Reconnect, which connects again to a dropped hub.
	retry  *fyne.Container
	footer *fyne.Container
	// changes offers to review the edits, when there are any.
	changes *fyne.Container
	review  *widget.Button
	pending *widget.Label
	// pendingBox holds pending, in muted text.
	pendingBox fyne.CanvasObject
	// bottom holds changes and the status; it moves to the tab shown, so
	// on phones it sits above the tabs.
	bottom *fyne.Container
	slots  []*fyne.Container
	// pages are the tabs' contents, each with its slot for bottom.
	pages []*fyne.Container
	// undoBtn and redoBtn take back and make again the last edit.
	undoBtn, redoBtn *iconButton
	// root is the screen's content.
	root fyne.CanvasObject
}

// browseList is a tab's list, of devices or of activities, and the one
// picked: beside it on wide windows, instead of it on narrow ones.
type browseList struct {
	s     *browseScreen
	rows  []browseRow
	list  *widget.List
	split *listDetail
	// shown is the row whose detail is shown, if any.
	shown *browseRow
	tab   *container.TabItem
	// build lists the rows.
	build func() []browseRow
}

// browseRow is a row of a list: an activity, a device, the row that
// adds one, or the one that changes their order.
type browseRow struct {
	activity    *profile.Activity
	device      *profile.Device
	add         bool
	addActivity bool
	reorder     bool
}

func (r browseRow) text() string {
	switch {
	case r.activity != nil:
		return r.activity.Name
	case r.device != nil:
		return r.device.Name
	case r.add:
		return i18n.T("Add a device…")
	case r.addActivity:
		return i18n.T("Add an activity…")
	case r.reorder:
		return i18n.T("Change the order…")
	}
	return ""
}

func (a *App) showBrowse(h backend.Hub) *browseScreen {
	a.saveEdits(h, a.work)
	s := &browseScreen{app: a, hub: h}
	a.browse = s
	s.root = s.content(h)
	a.show(s.root, s.back)
	return s
}

// back leaves the device or activity shown, on phones, else goes to the
// hub's tab; from there, nowhere.
func (s *browseScreen) back() bool {
	i := s.tabs.SelectedIndex()
	for j, b := range []*browseList{s.devices, s.activities} {
		if i == devicesTab+j && b.split.narrow && b.split.showing {
			b.split.back()
			return true
		}
	}
	if i != hubTab {
		s.tabs.SelectIndex(hubTab)
		return true
	}
	return false
}

// undo takes back the last edit.
func (s *browseScreen) undo() {
	if what := s.app.work.Undo(); what != "" {
		s.edited(i18n.T("Undone: %s.", what))
	}
}

// redo makes the last edit undone again.
func (s *browseScreen) redo() {
	if what := s.app.work.Redo(); what != "" {
		s.edited(i18n.T("Redone: %s.", what))
	}
}

// edited shows the workspace after an edit made here, saying what it was.
func (s *browseScreen) edited(status string) {
	s.app.saveEdits(s.hub, s.app.work)
	s.reload()
	s.showChanges()
	s.setStatus(false, status)
}

// showUndo enables Undo and Redo when they have something to do.
func (s *browseScreen) showUndo() {
	for _, b := range []struct {
		btn *iconButton
		can bool
	}{{s.undoBtn, s.app.work.CanUndo()}, {s.redoBtn, s.app.work.CanRedo()}} {
		if b.can {
			b.btn.Enable()
		} else {
			b.btn.Disable()
		}
	}
}

// setStatus shows text at the bottom, with the pulse over it while busy.
func (s *browseScreen) setStatus(busy bool, text string) {
	s.failed = false
	s.status.SetText(text)
	s.retry.Hide()
	if busy {
		s.busy.Start()
	} else {
		s.busy.Stop()
	}
	s.updateFooter()
}

// clearStatus hides the status.
func (s *browseScreen) clearStatus() {
	s.setStatus(false, "")
}

// offerReconnect shows Reconnect beside the status.
func (s *browseScreen) offerReconnect() {
	s.retry.Show()
	s.updateFooter()
}

// updateFooter shows the bar at the bottom, one line where it can be:
// the status, or, with none, that there are changes to review; and
// Reconnect and Review changes beside it. Without either, it is hidden.
func (s *browseScreen) updateFooter() {
	changes := s.changes.Visible()
	if changes && s.status.Text == "" {
		s.pendingBox.Show()
	} else {
		s.pendingBox.Hide()
	}
	s.status.Hidden = s.status.Text == ""
	s.footer.Hidden = s.status.Hidden && !changes && !s.retry.Visible() && !s.busy.Visible()
	s.footer.Refresh()
	s.relayout()
}

// relayout lays the tab shown out again, after the bar at its bottom
// grew or shrank.
func (s *browseScreen) relayout() {
	if s.tabs != nil {
		s.pages[s.tabs.SelectedIndex()].Refresh()
	}
}

// state is how the row's device or activity differs from the hub's, in
// words, and the kind of chip saying it.
func (s *browseScreen) state(r browseRow) (string, int) {
	st := workspace.Unchanged
	switch {
	case r.activity != nil:
		st = s.app.work.ActivityState(r.activity)
	case r.device != nil:
		st = s.app.work.DeviceState(r.device)
	}
	switch st {
	case workspace.New:
		return i18n.T("New"), chipNew
	case workspace.Edited:
		return i18n.T("Edited"), chipEdited
	}
	return "", chipInfo
}

// controls drive the device or activity named name on the hub, or are
// nil while it is new (not on the hub yet) or there is no hub.
func (s *browseScreen) controls(name string, isNew bool) *controls {
	if isNew || s.app.client == nil {
		return nil
	}
	// do does f, saying doing meanwhile and done once done, or, quietly
	// (doing and done empty), only if it failed: as a remote, which
	// says nothing when a key works.
	do := func(doing, failed, done string, f func(context.Context, *workspace.Workspace, *hub.Client) error) {
		w, c := s.app.work, s.app.client
		if c == nil {
			s.setStatus(false, i18n.T("Not connected to the hub: reconnect first."))
			if !s.app.reconnecting {
				s.offerReconnect()
			}
			return
		}
		if doing != "" {
			s.setStatus(true, doing)
		}
		s.app.async(func() {
			ctx, cancel := context.WithTimeout(context.Background(), ConnectTimeout)
			defer cancel()
			err := f(ctx, w, c)
			fyne.Do(func() {
				switch {
				case err != nil:
					s.setStatus(false, fmt.Sprintf("%s: %v", failed, err))
					s.failed = true
				case done != "":
					s.setStatus(false, done)
				case s.failed:
					// What failed before works now.
					s.clearStatus()
				}
			})
		})
	}
	return &controls{
		press: func(key string, hold bool) {
			failed := i18n.T("Could not press %s", key)
			if hold {
				failed = i18n.T("Could not hold %s", key)
			}
			do("", failed, "",
				func(ctx context.Context, w *workspace.Workspace, c *hub.Client) error {
					return w.Press(ctx, c, name, key, hold)
				})
		},
		run: func(macro string) {
			do("", i18n.T("Could not run %s", macro), "",
				func(ctx context.Context, w *workspace.Workspace, c *hub.Client) error {
					return w.RunMacro(ctx, c, name, macro)
				})
		},
		send: func(device, command string) {
			do(i18n.T("Sending %s…", command), i18n.T("Could not send %s", command), i18n.T("Sent %s. Did %s respond?", command, device),
				func(ctx context.Context, w *workspace.Workspace, c *hub.Client) error {
					return w.Send(ctx, c, device, command)
				})
		},
	}
}

// selectActivity shows the activity named name.
func (s *browseScreen) selectActivity(name string) {
	s.tabs.SelectIndex(activitiesTab)
	s.activities.pick(name)
}

// selectDevice shows the device named name.
func (s *browseScreen) selectDevice(name string) {
	s.tabs.SelectIndex(devicesTab)
	s.devices.pick(name)
}

// showChanges shows the pending-changes bar if the workspace has edits.
func (s *browseScreen) showChanges() {
	s.showUndo()
	if !s.app.work.Changed() {
		s.changes.Hide()
		s.updateFooter()
		return
	}
	// Changes are planned against the hub as it is: not before the
	// background check read it.
	if s.app.checking {
		s.review.Disable()
		s.pending.SetText(i18n.T("Your changes are not on the hub yet. Review them once the hub is checked."))
	} else {
		s.review.Enable()
		s.pending.SetText(i18n.T("Your changes are not on the hub yet."))
	}
	s.changes.Show()
	s.updateFooter()
}

// reload shows the workspace again after it changed, keeping the detail
// shown if it is still there.
func (s *browseScreen) reload() {
	s.devices.reload()
	s.activities.reload()
	s.showHub()
}

func (s *browseScreen) content(h backend.Hub) fyne.CanvasObject {
	s.devices = s.newList(i18n.T("Pick a device to see its commands and keys."), func() []browseRow {
		var rows []browseRow
		for _, d := range s.app.work.Config.Devices {
			rows = append(rows, browseRow{device: d})
		}
		rows = append(rows, browseRow{add: true})
		if len(s.app.work.Config.Devices) > 1 {
			rows = append(rows, browseRow{reorder: true})
		}
		return rows
	})
	s.activities = s.newList(i18n.T("Pick an activity to see what it does."), func() []browseRow {
		var rows []browseRow
		for _, a := range s.app.work.Config.Activities {
			rows = append(rows, browseRow{activity: a})
		}
		rows = append(rows, browseRow{addActivity: true})
		if len(s.app.work.Config.Activities) > 1 {
			rows = append(rows, browseRow{reorder: true})
		}
		return rows
	})

	s.status = wrapped("")
	s.status.TextStyle.Italic = true
	s.busy = newPulse()
	s.busy.Hide()
	s.retry = container.NewCenter(widget.NewButtonWithIcon(i18n.T("Reconnect"), theme.ViewRefreshIcon(), func() {
		s.retry.Hide()
		s.app.reconnect()
	}))
	s.retry.Hide()
	s.review = widget.NewButtonWithIcon(i18n.T("Review changes"), theme.DocumentIcon(), s.app.showReview)
	s.review.Importance = widget.HighImportance
	s.pending = wrapped("")
	s.changes = container.NewCenter(s.review)
	s.changes.Hide()
	// One bar: what happened, or that there are changes, and what to do.
	s.pendingBox = quiet(s.pending)
	text := container.NewVBox(s.status, s.pendingBox)
	s.footer = container.NewBorder(container.NewVBox(widget.NewSeparator(), s.busy), nil, nil, nil,
		container.NewPadded(container.New(&besideOrUnder{}, text, container.NewHBox(s.retry, s.changes))))
	s.footer.Hide()
	s.bottom = container.NewStack(newFill(colorPage), s.footer)
	s.undoBtn = newIconButton(i18n.T("Undo"), theme.ContentUndoIcon(), s.undo)
	s.redoBtn = newIconButton(i18n.T("Redo"), theme.ContentRedoIcon(), s.redo)

	slot := func(o fyne.CanvasObject) fyne.CanvasObject {
		c := container.NewStack()
		s.slots = append(s.slots, c)
		page := container.NewBorder(nil, c, nil, nil, o)
		s.pages = append(s.pages, page)
		return page
	}
	// In the tabs' order: slots are found by the tab's index.
	hub := container.NewTabItemWithIcon(i18n.T("Hub"), theme.HomeIcon(), slot(s.hubContent(h)))
	s.devices.tab = container.NewTabItemWithIcon(i18n.T("Devices"), theme.ComputerIcon(), slot(s.devices.split))
	s.activities.tab = container.NewTabItemWithIcon(i18n.T("Activities"), theme.MediaPlayIcon(), slot(s.activities.split))
	about := container.NewTabItemWithIcon(i18n.T("Settings"), theme.SettingsIcon(), slot(s.app.settingsAndAbout(s.app.backToBrowse)))
	s.tabs = container.NewAppTabs(hub, s.devices.tab, s.activities.tab, about)
	for _, t := range s.tabs.Items {
		s.tabNames = append(s.tabNames, t.Text)
	}
	if fyne.CurrentDevice().IsMobile() {
		s.tabs.SetTabLocation(container.TabLocationBottom)
	}
	s.tabs.OnSelected = func(*container.TabItem) {
		i := s.tabs.SelectedIndex()
		s.app.tab = i
		s.updateBar()
		for j, c := range s.slots {
			if j == i {
				c.Objects = []fyne.CanvasObject{s.bottom}
			} else {
				c.Objects = nil
			}
			c.Refresh()
		}
	}
	s.tabs.SelectIndex(s.app.tab)
	s.tabs.OnSelected(nil)
	s.showChanges()
	return container.NewBorder(s.header(), nil, nil, nil, newFitTabs(s.tabs))
}

// header is the bar: the tab's name (or, a device or activity shown
// alone, its name and Back), the hub's under it, and undo and redo.
func (s *browseScreen) header() fyne.CanvasObject {
	s.barBack = widget.NewButtonWithIcon(i18n.T("Back"), theme.NavigateBackIcon(), func() { s.back() })
	s.barBackBox = container.NewCenter(s.barBack)
	s.barBackBox.Hide()
	top, title := bar(s.barBackBox, "", s.app.where(), container.NewCenter(container.NewHBox(s.undoBtn, s.redoBtn)))
	s.title, s.barTop = title, top
	for _, b := range []*browseList{s.devices, s.activities} {
		b.split.onChange = s.updateBar
	}
	s.updateBar()
	return top
}

// updateBar shows the tab's name in the bar, or the name of the device or
// activity shown alone, with Back.
func (s *browseScreen) updateBar() {
	if s.title == nil {
		return
	}
	i := s.tabs.SelectedIndex()
	title, back := s.tabNames[i], false
	for j, b := range []*browseList{s.devices, s.activities} {
		if i == devicesTab+j && b.split.detailAlone() {
			title, back = b.split.title, true
		}
	}
	s.title.SetText(title)
	if back != s.barBackBox.Visible() {
		s.barBackBox.Hidden = !back
		s.barTop.Refresh() // laid out again, Back in or out
	}
}

// hubContent is the hub's tab, its home: what the hub is, its
// activities and devices as tiles to open, its firmware, saving it to a
// file (and, for a file, restoring it), and leaving it.
func (s *browseScreen) hubContent(h backend.Hub) fyne.CanvasObject {
	s.hubInfo = wrapped("")
	s.hubName = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	s.hubName.SizeName = theme.SizeNameHeadingText
	s.hubName.Wrapping = fyne.TextWrapWord
	s.hubKind = newChip("", chipInfo)
	s.activityTiles, s.deviceTiles = tileGrid(nil), tileGrid(nil)
	s.showHub()
	img := canvas.NewImageFromResource(hubTile)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSquareSize(headingTile * Scale()))
	hero := widget.NewCard("", "", container.NewBorder(nil, nil, container.NewCenter(img), nil,
		container.NewVBox(container.NewBorder(nil, nil, nil, container.NewCenter(s.hubKind), s.hubName), quiet(s.hubInfo))))

	files := []fyne.CanvasObject{newNavRow(theme.NewPrimaryThemedResource(theme.DocumentSaveIcon()), i18n.T("Save to a file…"),
		i18n.T("The hub with your changes: a backup, or to apply later, without the hub."), s.app.saveToFile)}
	leave := newNavRow(theme.NewPrimaryThemedResource(theme.LogoutIcon()), i18n.T("Disconnect"), i18n.T("Leave the hub for another app, or another hub."), s.app.disconnect)
	if s.app.offline() {
		files = append(files, newNavRow(theme.NewPrimaryThemedResource(theme.UploadIcon()), i18n.T("Restore to a hub…"),
			i18n.T("Make a hub hold what this file has. You see every change before anything is written."), s.app.applyFile))
		leave = newNavRow(theme.NewPrimaryThemedResource(theme.LogoutIcon()), i18n.T("Close the file"), "", s.app.closeFile)
	}
	parts := []fyne.CanvasObject{hero,
		groupTitle(i18n.T("Activities")), s.activityTiles,
		groupTitle(i18n.T("Devices")), s.deviceTiles}
	if fw := s.firmwareCard(); fw != nil {
		parts = append(parts, fw)
	}
	parts = append(parts, group(i18n.T("Files"), files...), group("", leave))
	return scrolled(container.NewVBox(parts...))
}

// showHub says what the hub is, on its tab, and shows its activities
// and devices.
func (s *browseScreen) showHub() {
	a, info, cfg := s.app, s.app.work.Dump.Info, s.app.work.Config
	var text, kind string
	switch {
	case a.offline() && !a.file.saved.IsZero():
		text, kind = i18n.T("From the file %s, saved %s: %s, firmware %d.", a.file.name, ago(a.file.saved, a.now()), info.Model, info.Firmware), i18n.T("File")
	case a.offline():
		text, kind = i18n.T("From the file %s: %s, firmware %d.", a.file.name, info.Model, info.Firmware), i18n.T("File")
	case s.hub.Demo:
		text, kind = i18n.T("A demo hub: a simulated %s, firmware %d.", info.Model, info.Firmware), i18n.T("Demo")
	default:
		text = i18n.T("%s, firmware %d, at %s.", info.Model, info.Firmware, s.hub.Addr.String())
	}
	devs := i18n.N("%d device", "%d devices", len(cfg.Devices), len(cfg.Devices))
	acts := i18n.N("%d activity", "%d activities", len(cfg.Activities), len(cfg.Activities))
	text += "\n" + i18n.T("%s and %s.", devs, acts)
	s.hubInfo.SetText(text)
	s.hubName.SetText(info.Name)
	s.hubKind.SetText(kind, chipInfo)

	var acTiles, devTiles []fyne.CanvasObject
	for _, act := range cfg.Activities {
		acTiles = append(acTiles, newLaunchTile(tile(act.Icon, activityIcon, act.Color), act.Name, func() { s.selectActivity(act.Name) }))
	}
	acTiles = append(acTiles, newLaunchTile(addTile, i18n.T("New activity"), s.addActivity))
	for _, d := range cfg.Devices {
		devTiles = append(devTiles, newLaunchTile(tile(d.Icon, deviceIcon, d.Color), d.Name, func() { s.selectDevice(d.Name) }))
	}
	devTiles = append(devTiles, newLaunchTile(addTile, i18n.T("New device"), s.app.showAddDevice))
	s.activityTiles.Objects, s.deviceTiles.Objects = acTiles, devTiles
	s.activityTiles.Refresh()
	s.deviceTiles.Refresh()
}

// deviceTile is the tile of the device named name.
func (s *browseScreen) deviceTile(name string) fyne.Resource {
	if d := s.app.work.Config.Device(name); d != nil {
		return tile(d.Icon, deviceIcon, d.Color)
	}
	return tile(0, deviceIcon, 0)
}

// addActivity adds an activity, once there is a device for it.
func (s *browseScreen) addActivity() {
	if len(s.app.work.Config.Devices) == 0 {
		s.setStatus(false, i18n.T("Add a device first: an activity needs at least one."))
		return
	}
	s.app.showEditActivity("")
}

// newList makes a tab's list, whose rows build lists, with hint shown
// beside it until one is picked.
func (s *browseScreen) newList(hint string, build func() []browseRow) *browseList {
	b := &browseList{s: s, build: build}
	b.rows = build()
	b.list = widget.NewList(
		func() int { return len(b.rows) },
		func() fyne.CanvasObject {
			icon := canvas.NewImageFromResource(theme.ContentAddIcon())
			icon.FillMode = canvas.ImageFillContain
			icon.SetMinSize(fyne.NewSquareSize(rowTile * Scale()))
			l := widget.NewLabel("")
			l.Truncation = fyne.TextTruncateEllipsis
			state := newChip("", chipNew)
			return listRow(container.NewBorder(nil, nil, icon, container.NewCenter(state), l))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			r := b.rows[i]
			row := o.(*fyne.Container).Objects[0].(*fyne.Container)
			l, icon, state := row.Objects[0].(*widget.Label), row.Objects[1].(*canvas.Image), row.Objects[2].(*fyne.Container).Objects[0].(*chip)
			l.SetText(r.text())
			text, kind := s.state(r)
			state.SetText(text, kind)
			switch {
			case r.activity != nil:
				icon.Resource = tile(r.activity.Icon, activityIcon, r.activity.Color)
			case r.device != nil:
				icon.Resource = tile(r.device.Icon, deviceIcon, r.device.Color)
			case r.reorder:
				icon.Resource = theme.NewPrimaryThemedResource(gripIcon)
			default:
				icon.Resource = theme.NewPrimaryThemedResource(theme.ContentAddIcon())
			}
			icon.Refresh()
		})
	b.list.OnSelected = func(i widget.ListItemID) {
		r := b.rows[i]
		switch {
		case r.add:
			b.list.Unselect(i)
			s.app.showAddDevice()
		case r.reorder:
			b.list.Unselect(i)
			s.app.showOrder(b == s.activities)
		case r.addActivity:
			b.list.Unselect(i)
			s.addActivity()
		case r.activity != nil:
			b.shown = &r
			act := r.activity
			s.app.check(act.Name)
			b.split.show(act.Name, activityView(act, s.app.work.Dump.Info.Model, s.controls(act.Name, s.app.work.ActivityState(act) == workspace.New), func() { s.app.showEditActivity(act.Name) }, s.deviceTile))
		case r.device != nil:
			b.shown = &r
			d := r.device
			s.app.check(d.Name)
			var export func()
			if workspace.HasIRCodes(d) {
				export = func() { s.app.exportCodes(d) }
			}
			b.split.show(d.Name, deviceView(d, s.app.work.Dump.Info.Model, s.controls(d.Name, s.app.work.DeviceState(d) == workspace.New), func() { s.app.showEditDevice(d.Name) }, export))
		}
	}
	// Inset from the window's edges, as the pages are.
	b.split = newListDetail(container.NewPadded(b.list))
	b.split.onBack = func() {
		b.shown = nil
		b.list.UnselectAll()
	}
	h := wrapped(hint)
	h.TextStyle.Italic = true
	b.split.placeholder = container.NewPadded(container.NewVBox(h))
	b.split.clear()
	return b
}

// pick shows the row named name.
func (b *browseList) pick(name string) {
	for i, r := range b.rows {
		if (r.activity != nil || r.device != nil) && r.text() == name {
			b.list.Select(i)
			return
		}
	}
}

// reload lists the rows again, keeping the detail shown if it is still
// there.
func (b *browseList) reload() {
	shown := b.shown
	b.rows = b.build()
	b.list.UnselectAll()
	b.list.Refresh()
	if shown == nil {
		return
	}
	for i, r := range b.rows {
		if (r.activity != nil || r.device != nil) && r.text() == shown.text() {
			b.list.Select(i)
			return
		}
	}
	b.split.clear()
}
