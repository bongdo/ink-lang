# tinywl on Ink

A port of wlroots' `tinywl` to Ink, on top of the Go binding `swaywm/go-wlroots`
(wlroots 0.18). It is a real Wayland compositor: backend, renderer, scene graph,
`xdg-shell`, seat, cursor and keyboard bindings. Ink is the policy layer; every
call that touches the GPU or the protocol goes through the binding.

`client/fpsclient.c` is a minimal Wayland client (shm buffer, `xdg_toplevel`)
that counts `wl_surface.frame` callbacks and reports frame-time percentiles.

`bench.sh` runs a fixed matrix of sizes, renderers and client counts.

## Run

```sh
# deps: wlroots 0.18 headers, libwayland-dev, wayland-protocols, cc
./run.sh            # one case, prints client JSON + INKSTAT from the compositor
./bench.sh          # the matrix below
```

Environment knobs: `INK_OUT_W`, `INK_OUT_H`, `INK_OUT_HZ` (headless output size
and refresh, Hz), `WLR_RENDERER` (`pixman` or `gles2`), `FRAMES`, `WIN_W`,
`WIN_H`.

The headless output has no mode of its own, so wlroots paces it at 60 Hz and a
frame measurement would just report 60. `main.ink` therefore sets a custom mode;
the binding does not expose `wlr_output_state_set_custom_mode`, so `run.sh`
applies `wlroots-0.18-custom-mode.patch` to a copy of the module in `.deps/`.
Raise `INK_OUT_HZ` to remove the pacer from the measurement: then fps is
throughput, and the useful number is the compositor's `work_us`.

## NixOS

`nix-shell -p wlroots_0_18` from nixpkgs 24.11 starts but cannot create an EGL
display: an old libwlroots against the current Mesa fails `eglQueryDevicesEXT`.
Build wlroots 0.18 from source against the *current* nixpkgs instead:

```sh
nix-shell -p meson ninja pkg-config wayland wayland-protocols wayland-scanner \
  libxkbcommon pixman libdrm mesa libgbm libglvnd hwdata seatd libinput \
  libdisplay-info libliftoff vulkan-loader xwayland libxcb libxcb-wm \
  libxcb-image libxcb-errors gcc --run '
    cd .wlroots/src &&
    meson setup build -Dprefix=$PWD/../.. -Dbuildtype=release -Dwerror=false \
      -Dexamples=false -Dbackends=drm,libinput -Drenderers=gles2 &&
    ninja -C build install'
```

Then export `PKG_CONFIG_PATH_FOR_TARGET=$PWD/.wlroots/lib/pkgconfig` (nixpkgs'
`pkg-config` wrapper ignores plain `PKG_CONFIG_PATH`) and
`LD_LIBRARY_PATH=$PWD/.wlroots/lib`.

Two `-Werror` failures are expected with a 2026 toolchain: `_FORTIFY_SOURCE` at
`-O0` (use `-Dbuildtype=release`) and a new `LIBINPUT_SWITCH_*` enum value
(`-Dwerror=false`). A build without `x11` also lacks `wlr/backend/x11.h`, which
the binding includes unconditionally: drop that include and the
`wlr_x11_output_set_title` branch in `.deps/go-wlroots/wlroots/output.go`.

## Measured

NixOS 26.05, x86_64, Intel Iris Xe (Alder Lake-P), headless output, one client
unless noted. `work_us` is the compositor's own frame time
(`SceneOutput.Commit` + `SendFrameDone`); fps is what the client received and is
capped by the pacer.

| case | output | renderer | client fps | work p50 | work p99 | work max |
|---|---|---|---|---|---|---|
| 720p window | 1280x720 @1kHz | gles2 | 961 | 13 µs | 62 µs | 104 µs |
| 1080p window | 2560x1440 @1kHz | gles2 | 719 | 51 µs | 223 µs | 389 µs |
| 1080p window | 2560x1440 @1kHz | pixman | 572 | 642 µs | 1490 µs | 1873 µs |
| 2 clients 720p | 2560x1440 @1kHz | gles2 | 453 + 900 | 57 µs | 226 µs | 273 µs |
| 8 clients 540p | 2560x1440 @1kHz | gles2 | ~2500 total | 59 µs | 197 µs | 221 µs |
| 1080p window | 3840x2160 @1kHz | gles2 | 686 | 55 µs | 223 µs | 403 µs |

RSS: 97 MB with gles2, 26 MB with pixman.

As a fraction of the frame budget: at 200 Hz (5 ms) gles2 uses 1.0 % at p50 and
4.5 % at p99; pixman uses 12.8 % and 29.8 %. At 240 Hz (4.17 ms) those become
1.2 % / 5.4 % and 15.4 % / 35.7 %.

Client-side 1 % low is where the real damage is: with two clients one of them
once waited 440 ms for a frame (1 % low 148 ms), and with eight clients one
waited 3 s. That is the thing to chase, not the average.

## What is verified

- Compositor starts, creates a Wayland socket, serves `wl_compositor` v5,
  `xdg_wm_base` v3, `wl_seat`, `wl_output` (`wayland-info` enumerates them).
- A client creates a toplevel, gets `configure`, maps, is focused, and receives
  frame callbacks.
- The numbers above, from three runs; the p50 for a given case repeats within a
  few µs.

Not verified: DRM/KMS and a real monitor, input devices, interactive
move/resize, and which Mesa driver the EGL fallback picks — `eglQueryDevicesEXT`
fails in the headless session, wlroots falls back to surfaceless, and the 50 µs
against pixman's 640 µs says hardware but does not prove it.

## Differences from the C tinywl

- `PreferredMode` is replaced by `Modes()[0]` or a custom mode: the Go binding
  returns an error when an output has no modes (headless), and Ink's `on err`
  block cannot return early from the enclosing function.
- `wlr_seat_keyboard_notify_enter` is skipped when no keyboard exists. The Go
  binding dereferences the keyboard, so the upstream Go port segfaults on the
  first mapped window in a headless session; this port does not.
