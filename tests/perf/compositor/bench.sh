#!/usr/bin/env bash
set -uo pipefail
cd "$(dirname "$0")"

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/tmp/inkbench-runtime}"
mkdir -p "$XDG_RUNTIME_DIR"
chmod 700 "$XDG_RUNTIME_DIR"

if [ ! -x client/build/fpsclient ]; then
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
fi

comp=${COMP:-$(ls -t bin/* 2>/dev/null | head -1)}
if [ -z "$comp" ]; then
	echo "build first: ./ink build" >&2
	exit 1
fi

case_out=/tmp/inkbench-cases
mkdir -p "$case_out"

run_case() {
	local label=$1 frames=$2 ow=$3 oh=$4 hz=$5 r=$6 ww=$7 wh=$8 n=$9
	pkill -x compositor 2>/dev/null
	sleep 0.4
	rm -f "$XDG_RUNTIME_DIR"/wayland-0 "$XDG_RUNTIME_DIR"/wayland-0.lock

	WLR_BACKENDS=headless WLR_RENDERER="$r" WLR_LIBINPUT_NO_DEVICES=1 WLR_HEADLESS_OUTPUTS=1 \
		INK_OUT_W="$ow" INK_OUT_H="$oh" INK_OUT_HZ="$hz" \
		"$comp" >"$case_out/$label.log" 2>&1 &
	local cpid=$!
	sleep 1.2
	if ! kill -0 "$cpid" 2>/dev/null; then
		echo "CASE $label FAILED to start:"
		sed -n '1,5p' "$case_out/$label.log"
		return
	fi
	local rss0 rss1
	rss0=$(ps -o rss= -p "$cpid" | tr -d ' ')

	local pids=()
	for i in $(seq 1 "$n"); do
		WAYLAND_DISPLAY=wayland-0 ./client/build/fpsclient -n "$frames" -w "$ww" -h "$wh" \
			>"$case_out/$label-$i.json" 2>&1 &
		pids+=($!)
	done
	for p in "${pids[@]}"; do wait "$p"; done
	rss1=$(ps -o rss= -p "$cpid" | tr -d ' ')

	echo "CASE $label renderer=$r out=${ow}x${oh}@${hz} win=${ww}x${wh} clients=$n rss_kb=$rss1"
	grep INKSTAT "$case_out/$label.log" | tail -1 | sed 's/^/  /'
	for i in $(seq 1 "$n"); do
		sed "s/^/  client$i: /" "$case_out/$label-$i.json" 2>/dev/null
	done
	kill "$cpid" 2>/dev/null
	sleep 0.3
}

run_case ref-720p-pixman 1200 1280 720 1000 pixman 1280 720 1
run_case 1440p-pixman 600 2560 1440 1000 pixman 1920 1080 1
run_case 1440p-gles2 1200 2560 1440 1000 gles2 1920 1080 1
run_case 1440p-pixman-ceiling 300 2560 1440 10000 pixman 1920 1080 1
run_case 1440p-gles2-ceiling 600 2560 1440 10000 gles2 1920 1080 1
run_case 1440p-gles2-4clients 1200 2560 1440 1000 gles2 1280 720 4
run_case 1440p-gles2-16clients 600 2560 1440 1000 gles2 640 360 16
run_case 4k-gles2 600 3840 2160 1000 gles2 1920 1080 1
