# Frame benchmark

CPU-only 2.5K frame: arena rows, `par for` shading, row copy into a persistent
buffer. No raylib, no window, no upload — it measures Ink codegen and the arena,
not the renderer.

```sh
ink run
```

240 frames, 2560×1440, 14.7 MB per frame, arithmetic LUT-like shader. Numbers
depend on the machine; compare runs on the same host, not across hosts.

Current baseline and the measured before/after are in
[`docs/proposals/frame-2.5k-measurements.md`](../../../docs/proposals/frame-2.5k-measurements.md).
