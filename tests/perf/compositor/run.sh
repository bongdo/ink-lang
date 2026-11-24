#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

frames=${1:-600}

pkg-config --exists wlroots-0.18 || { echo "need wlroots 0.18 headers: pkg-config wlroots-0.18" >&2; exit 1; }
pkg-config --exists wayland-client || { echo "need libwayland-dev" >&2; exit 1; }
pkg-config --exists wayland-protocols || { echo "need wayland-protocols" >&2; exit 1; }

: "${XDG_RUNTIME_DIR:=/tmp/ink-compositor-runtime}"
export XDG_RUNTIME_DIR
mkdir -p "$XDG_RUNTIME_DIR"
chmod 700 "$XDG_RUNTIME_DIR"

INK=${INK:-}
if [ -z "$INK" ]; then
	if command -v ink >/dev/null 2>&1; then
		INK=ink
	else
		INK=../../../bin/ink
	fi
fi
"$INK" build

xml="$(pkg-config --variable=pkgdatadir wayland-protocols)/stable/xdg-shell/xdg-shell.xml"
mkdir -p client/build
wayland-scanner client-header "$xml" client/build/xdg-shell-client-protocol.h
wayland-scanner private-code "$xml" client/build/xdg-shell-protocol.c
cc -O2 -o client/build/fpsclient client/fpsclient.c client/build/xdg-shell-protocol.c \
	-Iclient/build $(pkg-config --cflags --libs wayland-client)

export WLR_BACKENDS="${WLR_BACKENDS:-headless}"
export WLR_RENDERER="${WLR_RENDERER:-pixman}"
export WLR_LIBINPUT_NO_DEVICES="${WLR_LIBINPUT_NO_DEVICES:-1}"
export WLR_HEADLESS_OUTPUTS="${WLR_HEADLESS_OUTPUTS:-1}"

rm -f "$XDG_RUNTIME_DIR/wayland-0" "$XDG_RUNTIME_DIR/wayland-0.lock"
./bin/compositor &
pid=$!
trap 'kill "$pid" 2>/dev/null || true' EXIT
sleep 1

WAYLAND_DISPLAY="${WAYLAND_DISPLAY:-wayland-0}" ./client/build/fpsclient "$frames"
