# tinywl on Ink

A port of wlroots' `tinywl` to Ink, on top of the Go binding `swaywm/go-wlroots`
(wlroots 0.18). It is a real Wayland compositor: backend, renderer, scene graph,
`xdg-shell`, seat, cursor and keyboard bindings. Ink is the policy layer; every
call that touches the GPU or the protocol goes through the binding.

`client/fpsclient.c` is a minimal Wayland client (shm buffer, `xdg_toplevel`)
that counts `wl_surface.frame` callbacks and prints the delivered rate.

## Run

```sh
# deps: wlroots 0.18 headers, libwayland-dev, wayland-protocols, cc
./run.sh 600
```

Headless virtual output, software renderer:

```
frames=600 seconds=9.704 fps=61.8
```

**Read that number as the virtual output's refresh rate, not as a GPU result.**
With `WLR_BACKENDS=headless` the output has no mode and no refresh
(`o.Refresh()` returns 0), so wlroots paces frames at its 60 Hz default. The
compositor's own per-frame work is invisible next to that; the CPU side was
measured separately in `../frames`.

Real numbers need a display and DRM:

```sh
WLR_BACKENDS=drm WLR_RENDERER=gles2 ./run.sh 600   # as root or with seat access
```

Then the same client reports what the monitor actually gets, and the 1% low
becomes meaningful.

## What is verified

- Compositor starts, creates a Wayland socket, serves `wl_compositor` v5,
  `xdg_wm_base` v3, `wl_seat`, `wl_output` (`wayland-info` enumerates them).
- A client creates a toplevel, gets `configure`, maps, is focused, and receives
  frame callbacks paced by the output.
- 600 frames in 9.7 s, twice, 61.7–61.8 fps.

Not verified: DRM/KMS output, GPU renderer, input devices, interactive
move/resize (ported but never exercised without a pointer).

## Differences from the C tinywl

- `PreferredMode` is replaced by `Modes()[0]`: the Go binding returns an error
  when an output has no modes (headless), and Ink's `on err` block cannot return
  early from the enclosing function.
- `wlr_seat_keyboard_notify_enter` is skipped when no keyboard exists. The Go
  binding dereferences the keyboard, so the upstream Go port segfaults on the
  first mapped window in a headless session; this port does not.
