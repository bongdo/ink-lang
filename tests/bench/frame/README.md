# Frame benchmark

CPU-only 2.5K frame: four variants of the same shade work, 240 frames each.
No raylib, no window, no upload — it measures Ink codegen, the arena and the
pixel format, not the renderer.

| variant | data flow | shader |
|---|---|---|
| A `arena u8 + copy` | arena rows, copy into a persistent byte buffer | 4 byte stores per pixel |
| B `arena u32 + copy` | same, rows are `u32[W]` | one `u32` store per pixel |
| C `u32 direct` | persistent `u32` frame buffer, no arena and no copy | one `u32` store per pixel |
| D `u32 + LUT` | same as C | 256-entry table, index advances by 31 |

```sh
ink run
```

240 frames, 2560×1440, 14.7 MB per frame. `D` changes the shader on purpose:
the real 2.5K measurement was LUT shading, while A–C use the synthetic
arithmetic form. Compare runs on the same host, not across hosts. On a 4 vCPU
Neoverse-N1, Go 1.27, no `-race` (2026-11-25):

| variant | p50 | p99 | max |
|---|---:|---:|---:|
| A | 2.80 ms | 5.4–5.7 ms | 6.3–8.4 ms |
| B | 2.34 ms | 4.5–5.1 ms | 5.6–6.2 ms |
| C | 1.94 ms | 3.9 ms | 3.9–5.2 ms |
| D | 0.97 ms | 2.2–2.5 ms | 2.5–3.6 ms |

A and B skip the arena clear: sema proves `rows` is fully written before any
read and the runtime hands out `SliceUninit` instead of `Slice`. That removed
~0.3 ms from both and left the checksum unchanged. The same run before the
change was A 3.09–3.25 and B 2.62–2.68.

The spread is host noise: p50 is stable, p99 is not. The baseline and the
earlier measurements are in
[`docs/proposals/frame-2.5k-measurements.md`](../../../docs/proposals/frame-2.5k-measurements.md).
