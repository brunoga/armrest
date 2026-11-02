# Armrest

Set up Sofabaton X1, X1S and X2 hubs from your computer or phone, without
Sofabaton's app or cloud: devices, activities, keys, macros and favorites.

Armrest is a [Fyne](https://fyne.io) app built on
[`github.com/brunoga/sofabaton`](https://github.com/brunoga/sofabaton), the
Go library behind the `sbx` command-line tool. It runs on Linux, Windows,
macOS and Android; iOS comes later.

**Status: early.** It finds hubs and lists their activities and devices.
Editing comes next: see [PLAN.md](PLAN.md).

## Running it

```sh
go run github.com/brunoga/armrest/cmd/armrest@latest
```

Fyne needs a C compiler and, on Linux, the OpenGL and X11 headers (on
Debian and Ubuntu: `sudo apt install gcc libgl1-mesa-dev xorg-dev`). See
[Fyne's prerequisites](https://docs.fyne.io/started/).

For Android, CI builds an APK on every change (the `armrest-android`
artifact of a CI run); install it with `adb install -r Armrest.apk`.

The hub takes one app at a time: close Sofabaton's app (and `sbx`) before
connecting. A hub connected to another app does not answer discovery
either. No hub? Open the demo hub, a simulated X2.

## Development

```sh
make run      # run the app
make check    # tidy, vet, lint and the tests
make android  # an APK (needs the fyne tool and the Android NDK)
```

The UI tests render each screen with Fyne's software driver and compare it
with the screenshots in `internal/ui/testdata` (light and dark). When a
screen changes on purpose, look at the new images the failing test wrote
to `testdata/failed/`, then accept them with `make screens`.

## License

Apache 2.0: see [LICENSE](LICENSE). Sofabaton is a trademark of its owner.
This project is not affiliated with or endorsed by Sofabaton.
