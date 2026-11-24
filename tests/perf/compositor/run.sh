#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

frames=${FRAMES:-600}
win_w=${WIN_W:-1280}
win_h=${WIN_H:-720}

pkg-config --exists wlroots-0.18 || { echo "need wlroots 0.18 headers: pkg-config wlroots-0.18" >&2; exit 1; }
pkg-config --exists wayland-client || { echo "need libwayland-dev" >&2; exit 1; }
pkg-config --exists wayland-protocols || { echo "need wayland-protocols" >&2; exit 1; }

INK=${INK:-}
if [ -z "$INK" ]; then
	if command -v ink >/dev/null 2>&1; then
		INK=ink
	else
		INK=../../../bin/ink
	fi
fi

dep=.deps/go-wlroots
if [ ! -d "$dep" ]; then
	mkdir -p .deps
	"$INK" --help >/dev/null 2>&1 || true
	mod="$(go env GOMODCACHE)/github.com/swaywm/go-wlroots@v0.0.0-20250523114853-368024c750af"
	if [ ! -d "$mod" ]; then
		go mod download github.com/swaywm/go-wlroots
	fi
	cp -r "$mod" "$dep"
	chmod -R u+w "$dep"
	(cd "$dep" && patch -p1 < ../../wlroots-0.18-custom-mode.patch)
fi

: "${XDG_RUNTIME_DIR:=/tmp/ink-compositor-runtime}"
export XDG_RUNTIME_DIR
mkdir -p "$XDG_RUNTIME_DIR"
chmod 700 "$XDG_RUNTIME_DIR"

"$INK" build

scanner=$(command -v wayland-scanner || true)
	if [ -z "$scanner" ]; then
		scanner=$(ls /nix/store/*/bin/wayland-scanner 2>/dev/null | head -1)
	fi
	xml="$(pkg-config --variable=pkgdatadir wayland-protocols)/stable/xdg-shell/xdg-shell.xml"
mkdir -p client/build
"$scanner" client-header "$xml" client/build/xdg-shell-client-protocol.h
"$scanner" private-code "$xml" client/build/xdg-shell-protocol.c
cc -O2 -o client/build/fpsclient client/fpsclient.c client/build/xdg-shell-protocol.c \
	-Iclient/build $(pkg-config --cflags --libs wayland-client)

export WLR_BACKENDS="${WLR_BACKENDS:-headless}"
export WLR_RENDERER="${WLR_RENDERER:-pixman}"
export WLR_LIBINPUT_NO_DEVICES="${WLR_LIBINPUT_NO_DEVICES:-1}"
export WLR_HEADLESS_OUTPUTS="${WLR_HEADLESS_OUTPUTS:-1}"
export INK_OUT_W="${INK_OUT_W:-1280}"
export INK_OUT_H="${INK_OUT_H:-720}"
export INK_OUT_HZ="${INK_OUT_HZ:-1000}"

rm -f "$XDG_RUNTIME_DIR/wayland-0" "$XDG_RUNTIME_DIR/wayland-0.lock"
"$comp" > compositor.log 2>&1 &
pid=$!
trap 'kill "$pid" 2>/dev/null || true' EXIT
sleep 1

WAYLAND_DISPLAY="${WAYLAND_DISPLAY:-wayland-0}" ./client/build/fpsclient -n "$frames" -w "$win_w" -h "$win_h"
