#!/usr/bin/env bash
# Checks that a build of the app needs nothing but what its OS provides:
# the C library, the system's OpenGL, and the windowing system's client
# libraries. Anything else (a MinGW runtime DLL, a Homebrew library) would
# have to ship with the app.
#
# usage: check-deps.sh <binary or .apk>
set -euo pipefail
bin=$1

os=$(uname -s)
case "$bin" in *.apk) os=Android ;; esac

case "$os" in
Android)
	# The app's native libraries may only need the NDK's stable system
	# libraries, which every Android device has.
	dir=$(mktemp -d)
	trap 'rm -rf "$dir"' EXIT
	unzip -q "$bin" 'lib/*' -d "$dir"
	deps=$(for so in "$dir"/lib/*/*.so; do readelf -d "$so"; done | sed -n 's/.*Shared library: \[\(.*\)\]/\1/p' | sort -u)
	allowed='^(libc|libm|libdl|liblog|libandroid|libEGL|libGLESv[123]|libGLESv1_CM|libvulkan|libz|libjnigraphics|libmediandk|libOpenSLES|libaaudio|libnativewindow)\.so$'
	;;
Linux)
	# The libraries named in the binary itself; GLFW loads the optional
	# ones (Xrandr, Xcursor, xkbcommon...) with dlopen when present.
	deps=$(readelf -d "$bin" | sed -n 's/.*Shared library: \[\(.*\)\]/\1/p')
	allowed='^(libc|libm|libdl|libpthread|librt|libGL|libEGL|libX11|libwayland-client|ld-linux.*)\.so'
	;;
Darwin)
	deps=$(otool -L "$bin" | tail -n +2 | awk '{print $1}')
	allowed='^(/usr/lib/|/System/Library/)'
	;;
MINGW* | MSYS* | CYGWIN*)
	deps=$(objdump -p "$bin" | sed -n 's/.*DLL Name: //p')
	# Windows' own DLLs; MinGW's runtimes (libgcc_s, libwinpthread,
	# libstdc++) are not, and must be linked in statically.
	allowed='^(kernel32|user32|gdi32|shell32|ole32|oleaut32|opengl32|advapi32|winmm|ws2_32|ntdll|msvcrt|dwmapi|imm32|shlwapi|comdlg32|version|setupapi|bcrypt|bcryptprimitives|crypt32|userenv|powrprof|uxtheme|api-ms-win-[a-z0-9-]+)\.dll$'
	;;
*)
	echo "check-deps: unknown OS $(uname -s)" >&2
	exit 2
	;;
esac

echo "$bin needs:"
echo "$deps" | sed 's/^/  /'
bad=$(echo "$deps" | grep -viE "$allowed" || true)
if [ -n "$bad" ]; then
	echo "not provided by the OS:" >&2
	echo "$bad" | sed 's/^/  /' >&2
	exit 1
fi
