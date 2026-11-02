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
| 3 Activities | Activity editing: devices and inputs, keys, macros, favorites, icon and color. Review and apply. | an activity made and edited only in the app works on the remote |
| 4 Devices | Device editing, the add-device wizard from templates and codes, "send now" | a device added from a template in the app works |
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
- [ ] Phase 1: the library work (L1–L4) in `sofabaton`.
