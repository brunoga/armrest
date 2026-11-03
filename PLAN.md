# Armrest: plan

Armrest is a GUI app for Sofabaton hubs. It must not
look like Sofabaton's own: no "Sofabaton" in the app's name or icon, only
in descriptions ("for Sofabaton X1, X1S and X2 hubs").

Goal: one Go codebase that sets up and edits a hub's devices,
activities, keys, macros, favorites, icons and colors, without
Sofabaton's cloud. It builds for Windows, macOS, Linux and Android, and
later iOS. It is a separate app from `sbx`, in its own repository, built on
the `github.com/brunoga/sofabaton` library.

## 1. Decisions

### 1.1 Toolkit: Fyne (decided)

| | Fyne | Cogent Core | Gio | Wails / go-app |
|---|---|---|---|---|
| Targets | all six (web via WASM) | all six (web via WASM) | all six | desktop (Wails), web only (go-app) |
| Maturity | v2 stable since 2021, v2.8 out; the largest Go GUI community | v0.x (first release 2024-07); API still moving; small team | stable core, small ecosystem | stable |
| Fit for a forms-and-lists app | good: stock widgets for lists, forms, dialogs, tabs and drag-reorder lists | good: Material 3 look, more styling freedom | poor: immediate mode, every widget by hand | UI in HTML and JS, not Go |
| Headless UI tests | `fyne.io/fyne/v2/test`: software rendering to PNG (used in codelab to review UIs without a display) | has snapshot tests too, less proven | by hand | browser tooling |
| Mobile packaging | `fyne package -os android/ios`, `fyne-cross` | `core build` | `gogio` | — |

**Fyne**: stable API, every target, widgets for exactly this kind of
app, and the headless screenshot tests that let changes be reviewed in
CI and by me without a display. You already use it in codelab. Its
weakness is the look: it is recognisably Fyne. A custom theme and a few
custom widgets (the remote's key layout, icon tiles) fix most of that.

**Cogent Core** is the strongest alternative: it looks more modern and
is easier to style. But it is pre-1.0 and its API has been changing, a
real cost for an app meant to last. Revisit only if Fyne's look or
performance turns out to be a blocker in the phase 0 spike.

Not recommended: **Gio** (too low-level for a form-heavy app),
**Wails** (the UI would be JS, not Go, and no mobile), **go-app**
(web only).

### 1.2 A separate repository

`brunoga/armrest`, a module depending on `github.com/brunoga/sofabaton`.
This keeps Fyne, cgo and the mobile toolchains out of `sbx`'s build and
CI, and keeps the library honest: the app may only use its public API.
Whatever the app needs from the protocol gets added to the library first,
with tests, then used here.

### 1.3 Platforms, in order

1. **Desktop:** Linux, Windows and macOS.
2. **Android:** next, since it is the easiest to test on a real phone.
3. **iOS:** last. It needs an Apple developer account for signing, to be
   created when we get there.

**The web is out of scope for now.** The hub talks UDP (discovery,
CALL_ME) and then connects back to the client over TCP (port 8200 and
up), which a browser can do neither of. A web build would need a small
server holding the hub connection. The backend interface (section 2)
keeps that possible later, without planning for it now.

The hub takes one client at a time: while the official app (or `sbx`) is
connected, CALL_ME is refused. The app must say so plainly, not fail with
a timeout.

### 1.4 Principles

- **Usability first**, in every screen and every phase. Common tasks take
  few steps, wording is plain (no protocol terms), every wait shows what
  is happening, errors say what to do next, and nothing is written to the
  hub without a review that can be backed out of.
- **Layouts adapt to the window, not the device.** A narrow window (a
  phone, a split screen) gets one column and drills down: list, then
  detail. A wide one (a tablet, a desktop) shows the list beside the
  detail. Text columns keep a readable width, and touch targets stay
  finger-sized everywhere.
- **A professional look** comes in the polish phase, with images you will
  provide; until then, Fyne's default theme, kept tidy.

## 2. Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ ui/        Fyne screens and widgets (no protocol code)        │
├─────────────────────────────────────────────────────────────┤
│ workspace/ the document: a hub's snapshot + the user's edits, │
│            validation, undo, the plan of what Apply writes    │
├─────────────────────────────────────────────────────────────┤
│ backend/   discover and connect: real hubs on the network,   │
│            and the demo hub; hands the UI a library client    │
│   └── demo    a simulated hub (hubsim) seeded with a dump     │
├─────────────────────────────────────────────────────────────┤
│ github.com/brunoga/sofabaton: hub, profile, dump, templates   │
└─────────────────────────────────────────────────────────────┘
cmd/armrest/         the app (all targets)
```

- **Edits are local until applied.** The workspace starts from a
  snapshot of the hub, as profiles. The user edits freely, with undo and
  validation as they type. **Review** shows the plan, the same list
  `sbx apply -n` prints, grouped by device and activity. **Apply** writes
  it with live progress. This is the library's existing model
  (Plan/Apply, PlanActivity, Restore) lifted to the whole hub.
- **The workspace is a file too.** A snapshot plus edits can be saved and
  opened offline, so a configuration can be prepared without the hub and
  applied later, or kept as a backup. Opening a dump is the same thing.
- **The demo backend** is the hub simulator seeded from a dump. It gives
  a working app with no hub, used for UI tests, screenshots, the docs and
  people trying the app.
- **Long operations** (snapshot about 35 s, deleting a favorite about
  14 s, creating a device about 1 min) run off the UI thread, with a
  progress bar listing the step being done, and can be cancelled between
  steps.

## 3. Library work first (in `sofabaton`)

| # | What | Why |
|---|---|---|
| L1 | `Snapshot`: the whole hub as profiles in one call (devices, activities, templates used), with a progress callback | the workspace's starting point; today it is a dump plus per-entity reads |
| L2 | Whole-hub plan and apply: target profiles vs the hub, including creating and deleting whole devices and activities, as one ordered plan (generalising `Restore`) | Review and Apply |
| L3 | Progress and cancellation on long operations (context and a callback per step) | the UI's progress bar |
| L4 | A clear "hub busy: another app is connected" error from CALL_ME refusals | the most common failure users will hit |
| L5 | Device and activity order (the hub's lists, `0x51`), and the remaining TODOs of write-flows | reordering in the UI |
| L6 | Transport options for mobile: choosing the listen port, a manual hub IP when mDNS is blocked | phones and guest networks |
| L7 | Icon drawings: upstream's X2 icon paths (MIT) as SVGs, with attribution, plus names (done) | icon pickers and tiles |
| L8 | Later: learning IR from a remote (if the hub supports it on the wire), Bluetooth devices, raw-data commands in new devices | the add-device wizard's other paths |

## 4. Screens

1. **Hubs:** discovered hubs, a manual IP, the demo hub, and open a saved
   workspace or dump. Shows the model, firmware and name.
2. **Home:** two lists of tiles in the remote's order, devices and
   activities, with icons and colors. Plus the pending-changes badge and
   **Review**.
3. **Device:**
   - Name, brand, icon and color.
   - **Commands:** drag to reorder, rename, delete, "send now" to test,
     and add one from an IR code (protocol plus code) or a descriptor.
   - **Keys:** a drawing of the X2 remote. Click a key, then pick a
     command (short and long press).
   - **Power:** the on and off commands.
   - **Inputs:** an ordered pick of commands.
   - **Macros:** the step editor (commands, waits, holds).
4. **Activity:**
   - Name, icon and color.
   - **Devices**, each with the input it is switched to.
   - **Power on and power off:** the step editor, with "reset to the
     app's default".
   - **Keys:** the remote drawing, picking from the activity's devices'
     commands and macros.
   - **Macros** across devices.
   - **Favorites:** an ordered list.
5. **Add a device:** a searchable template library (by transport and
   maker), or from scratch (protocol plus codes). Bluetooth comes later.
6. **Review and apply:** the plan as a grouped, readable list, with
   warnings (for example "removing this device removes it from 2
   activities"). Then live progress, and errors with what was already
   done.
7. **Backup and restore:** dump to a file, restore
   from a file with the plan first.
8. **Settings and diagnostics:** the frame log (as `-v`) to attach to bug
   reports, the version, licenses.

Shared widgets: the step editor, the remote's key layout, the icon and
color picker, a command picker (device → command, searchable), and
drag-reorder lists.

## 5. Testing

- **workspace and backend:** unit tests against the hub simulator, as
  in the library: edit → plan → apply → snapshot again equals the edits.
- **UI:** Fyne's test driver. Interaction tests (tap, type, drag) on
  every screen against the demo backend, and screenshot tests rendering
  each screen to PNG. Checked into `testdata`, they are compared in CI
  and reviewed when they change: every screen at phone (360×740) and
  tablet or desktop (1024×768) sizes, in light and dark.
- **Live:** a manual checklist per release on a real X2, as we have done
  with `sbx` (create, edit and delete each kind of thing; check the
  remote).

## 6. Builds and CI

- **Pull requests:** `make check` (tidy, vet, lint, tests including
  screenshots) on Linux, kept fast as in `sofabaton`.
- **Release workflow:**
  - Windows, Linux and macOS: native runners, or `fyne-cross`.
  - Android: an APK and AAB, with the NDK.
  - iOS (last): a macOS runner and Xcode. Needs an Apple developer
    account for signing.
- **Platform permissions:**
  - iOS: Local Network usage and Bonjour service entries (needed for
    mDNS and the hub's connect-back).
  - Android: INTERNET and a multicast lock for discovery.
  - macOS: the incoming-connection prompt.

## 7. Phases

| Phase | Content | Done when |
|---|---|---|
| 0 Spikes | A Fyne app that discovers the hub and lists devices: on Linux, Windows, macOS and Android. | each platform lists your hub's devices; the risks below are known, not guessed |
| 1 Library | L1–L4 in `sofabaton` | the whole-hub plan and apply pass against the simulator and on the X2 |
| 2 Browse | Hubs, Home, read-only Device and Activity screens, the demo backend, screenshot tests | your hub's configuration shows correctly |
| 3 Devices | **Creating devices from scratch** (the main reason for the app): the add-device wizard from the built-in templates or from an IR protocol and codes, "send now" to test each command, then device editing (commands, keys, power, inputs, macros, icon and color). Review and apply. | a device made only in the app works on the remote |
| 4 Activities | Activity editing: devices and inputs, keys, macros, favorites, icon and color | an activity made and edited only in the app works on the remote |
| 5 Workspaces | Saving and opening workspaces, backup and restore, undo | offline edits applied later |
| 6 Android | Packaging, touch layouts, permissions | installed on your phone, edits your hub |
| 7 iOS | Packaging, signing, Local Network permissions | installed on an iPhone, edits your hub |
| 8 Release | A professional look (theme, layouts, images you will provide), icon, accessibility, translations, store listings | v1.0 |

## 8. Risks, to settle in phase 0 (iOS's in phase 7)

- **iOS** (settled in phase 7, not 0): listening for the hub's
  connect-back and mDNS under iOS's Local Network rules, and whether
  Fyne's packaging lets us add the Info.plist keys.
- **Android:** multicast for discovery on some phones, and the emulator's
  NAT blocking the connect-back (test on a real phone).
- **One client at a time:** users will have the official app open. Clear
  messaging.
- **Fyne's look:** a custom theme and a few custom widgets. If that falls
  short, the UI layer is the only part a move to Cogent Core would touch.
- **Trademarks:** keep the app's name and icon clearly ours. Several of
  the hub's icons are service logos (Netflix, YouTube…), shown as the
  hub's own icon set.

## 9. Status

- [x] Name (Armrest), toolkit (Fyne), platform order (desktop, Android,
      iOS), no web for now.
- [x] Phase 0: the hubs screen (discovery, connect by address, the demo
      hub) and a read-only home screen (activities, devices), with
      screenshot tests in light and dark. CI builds Linux, Windows, macOS
      and an Android APK, each needing only what its OS provides. Checked
      on the X2 from Linux and from an Android phone (2026-11-02).
- [x] Phase 1 (library, sofabaton #49 and #50): `profile.Snapshot`,
      `profile.ApplyConfig` (match by id or name, create, delete,
      prune), consistent config edits (`RenameDevice`, `RenameCommand`,
      `RemoveDevice`, `RemoveActivity`), progress and cancellation,
      `dump.ReadProgress`, and reads retried when the hub drops one. The
      busy error was already there (`transport.ErrRefused`).
- [x] Phase 2: reading a hub keeps its lossless dump and takes the
      configuration from a simulated copy made from it (35 s on the X2,
      against 80 s for reading the profiles from the hub; a read that
      does not hold together is read again). The browse screen lists
      activities and devices and shows each read-only: beside the list
      on wide windows, one at a time with Back on phones.
- [x] Faster start: each hub's last dump is cached on the device and
      shown at once; the hub is checked for changes in the background,
      fetching command signals only for commands whose entry changed
      (`dump.Refresh`): 14 s instead of 31 s on the X2, and nobody waits
      for it. Writes never trust the cache: applying reads each device
      and activity it changes from the hub. Later, maybe: if entity
      records change with their contents, a 1 s check of the lists.
- [x] A lost connection while reading (a phone that slept) is explained,
      with Try again; it used to show an empty hub.
- [x] Phase 3a: Review and Apply, and adding a device from a built-in
      template. Review applies the edits to a simulated copy of the hub
      and lists what that wrote; Apply makes the same writes on the hub
      with progress (Stop between writes), then reads the hub back. New
      and edited devices and activities are marked in the list. Checked
      on the X2: a ZZ-test device added (31 writes, 1m38s) and deleted
      (1m35s), each plan exactly that.
- [x] Phase 3b: a device from scratch: name and brand, the IR protocol
      (NEC, Denon's two) and its parameters, commands as name and code
      (suggested names), power on and off; commands named like a key go
      on it (Volume Up on Vol+). Problems are said in words, at the
      field.
- [x] Phase 3c: Test sends a device's command from the hub. The device
      editor: name, brand, icon and color; commands (rename, recode, add,
      move, delete; renames carry to the activities, deletes take what
      used them); keys (press and hold); power; inputs; deleting the
      device. Checked on the X2: a ZZ-test device added, edited and
      deleted, each read back exactly as intended.
- [x] Found on the X2: the hub keeps a command's record when its signal
      is rewritten, so cached signals cannot be trusted. The background
      check reads every signal again; reading back after applying reads
      the written devices' signals again; Review waits for the check; if
      the hub changed under the user's edits, they choose.
- [x] Clearing a key of a device on the hub. The official app does it
      (Customize Remote buttons, Clear assignment) with the frame that
      clears an activity's key, 0x10 [device, key]; the hub tells it
      from a command delete by the id (captured on the X2, 2026-11-02;
      library #57). Checked live: TestLiveAddDevice clears Home.
- [x] Phase 3d: device macros in the editor (steps that send a command
      or wait; renames carry to the activities' keys, removals take the
      keys that ran them). Keys can run macros too.
- [x] Phase 4: activities: adding one ("Add an activity…") and the
      activity editor: name, icon and color; devices and their inputs;
      keys (press and hold: the devices' commands and macros, and the
      activity's macros); what starting and ending it does (the usual
      steps, recognized when the hub's are those, or custom: turn on,
      turn off, switch input, send, wait); macros; favorites; deleting
      it. Saving untouched changes nothing (tested for every activity
      and device). Checked on the X2: a ZZ-test activity created (8
      writes, 2m05s), edited (7, 1m44s) and deleted (1m28s), each read
      back exactly as intended.
- [x] Phase 5a: undo and redo of every edit (Undo and Redo on the
      browse screen; Ctrl+Z, Ctrl+Shift+Z and Ctrl+Y on desktop), Discard
      included. Edits not yet applied are kept with the cached hub, so
      they outlive the app being closed (a phone does it on its own), and
      are checked against the hub on the next visit like any edits.
- [x] Phase 5b: saving a workspace to a file (`.armrest`: the hub's dump
      and the edits as YAML, in JSON) from the browse screen's menu, and
      opening it (or an sbx dump) from the hubs screen, without the hub:
      browse, edit, undo, review. "Connect to the hub and apply" reads
      its hub whole and reviews the edits against it; a hub changed since
      asks first (5c).
- [x] Phase 5c: backup and restore. A saved file is a backup; "Restore
      to a hub…" (the file's menu) or applying its changes makes a hub
      hold what the file has. On its own hub as it was, that is the
      file's changes; on its hub changed since (matched by id: renames
      are renamed back) or on another hub (matched by name; the hub's
      others are deleted), the user agrees to a restore first, then
      reviews it. Undo takes it back before applying.
- [x] The remote's drawing for keys, in the device and activity editors:
      its keys grouped as on the remote (the X1 remotes' missing keys
      left out), those set marked; tapping one shows what pressing and
      holding it runs, to change or clear it, and the keys set are listed
      under it with what they run. Schematic for now; the remote's real
      layout and look come with phase 8.
- [x] The browse screen in tabs: Hub (what it is, saving to a file,
      restoring a file, disconnecting), Devices and Activities, at the
      bottom on phones, at the top elsewhere; it opens on Hub, and comes
      back from an editor to the tab it left. Undo, Redo and the changes
      to review are on every tab.
- [x] Phase 6 (part): Back (Escape on desktop) goes back in the app:
      out of an editor, from a device or activity to the list on phones,
      to the hub's tab, and only then out of the app; it does nothing
      while writing. The screen stays on while reading or writing the
      hub.
- [x] Phase 8 (first pass): the app's own theme, from its icon's teal,
      cream and orange, light and dark: a teal app bar on every screen,
      cream pages behind white cards, rounder corners, and everything a
      size larger (text of about 16, 17 on phones). Devices and
      activities show the X2's icon in their color, as the remote does
      (the drawings from the library, sofabaton #58); the color keys
      are in their colors. Next, with the user's images: the hubs
      screen, a font of the app's own.
- [x] Polish: one sign of work everywhere, a thin line sweeping
      across (under the progress bar while reading or writing, alone
      while connecting, searching, checking or sending). An About tab
      (and page, from the hubs screen): the icon, version, author,
      links, what the app is built with, the license, and a text size
      (Smaller to Largest; normal is about 18 on desktop, 17 on
      phones). The tabs show only their icons where their names do not
      fit; the color keys show only their colors.
- [x] Drag to reorder, with a mouse or a finger, by a handle of dots
      on each row: a device's commands (in its editor and a new
      device's), an activity's start and end steps, its macros' steps
      and its favorites. Move up and down stay in the rows' menus.
      Devices and activities themselves wait on the library (their
      order is not written yet).
- [x] A device or activity opens on its remote: the X2's keys, those
      it sets lit; a tap presses one on the hub, a long press (a right
      click) holds it, as the hub has them (a key bound to one of the
      activity's macros runs it). Beside the remote, its macros (each
      with Run) and, for activities, favorites (each with Run), and its
      details (commands with Test, keys, power, inputs, start and end
      steps); Edit stays at hand. Starting an activity as the remote
      does (the hub knowing it is on) waits on a capture of the app
      doing it.
- [x] Tap feedback on Android: the remote's keys (pressed and held), the
      key pad's, and a row picked up to drag give the phone's own tap
      feedback (View.performHapticFeedback, through JNI: no permission,
      and the phone's touch feedback setting decides). Fyne has no sound
      or vibration of its own; desktops give none.
- [ ] Later: the profile library server (section 10).

## 10. The profile library server (later; not built yet)

An **optional** service the app can connect to for device profiles
(remote-control code sets), so users rarely type codes by hand. The app
works fully without it: the built-in templates and creating from scratch
stay first-class, and the server's address is a setting (ours by
default, self-hostable).

**What it does:**

- **Browse and search** public profiles by brand, model, device type and
  transport, and download one into the add-device wizard.
- **Upload** a device's profile from the app (one the user made or
  edited). An upload is **private**: only its owner sees it until an
  admin reviews it and makes it public. Owners can update, withdraw and
  delete their own uploads from the app; every operation a user needs
  is in the app.
- **Admin UI** (web, admins only): the review queue, approve or reject
  with a note to the uploader, edit metadata, merge duplicates, unpublish
  or delete, manage users and admins, and see an audit log.

**Data:** a profile is the same YAML the library reads (`profile.Load`,
like `templates/`), with metadata (brand, model, type, transport,
source, notes), an owner, a state (private, in review, public,
rejected), versions (every change keeps the previous one), and download
counts. Uploads never carry anything from the user's hub beyond the
device's profile: no hub ids, names or other devices.

**Security** (designed in from the start, not added later):

- **Transport:** HTTPS only (HSTS); the app refuses plain HTTP except to
  a self-hosted address the user explicitly marks as trusted.
- **Accounts:** sign-in through an established provider (OpenID Connect:
  Google, GitHub, Apple) or passkeys; no passwords stored by us. The app
  uses the OAuth device or PKCE flow, keeps tokens in the platform's
  secure storage (Keychain, Android Keystore), and tokens are
  short-lived with refresh and revocation.
- **Authorization:** every request is checked server-side: users act
  only on their own uploads; admin actions need the admin role and a
  recent sign-in (step-up). Object ids are not guessable, and listing
  never leaks private uploads.
- **Untrusted input:** uploads are parsed strictly with the library
  (size limits, nesting limits, known fields only, no YAML anchors or
  aliases), validated (protocols, code ranges, command counts), and
  stored as data, never executed or rendered as HTML. Text fields are
  length-limited and escaped in the admin UI. The parsers stay fuzzed.
- **Abuse:** rate limits per user and per address on uploads, sign-ins
  and searches; upload quotas; reports from users; moderation before
  anything is public.
- **Admin UI:** separate admin origin or path with strict CSP, CSRF
  protection, SameSite cookies, session timeouts, and an audit log of
  every admin action.
- **Operations:** least-privilege service account, secrets outside the
  code (environment or a secret store), dependency and container
  scanning in CI, encrypted backups with tested restores, logs without
  tokens or personal data, and a published security contact.
- **Privacy:** collect only what is needed (account id, display name,
  uploads); users can export and delete their account and uploads.
- **Content:** uploaders agree that their profiles are shared under an
  open license; IR codes are facts about devices, but brand names and
  logos stay out of the app's look (see Trademarks).

**Shape:** a Go server in its own repository (or `cmd/` here, decided
then), a small JSON API used by the app, the admin UI as server-rendered
pages, and a database (SQLite to start, PostgreSQL if it grows). The
library gains a profile-metadata type the app, the server and `sbx`
share.
