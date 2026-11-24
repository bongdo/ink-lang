# Frame budget benchmark

Measures the CPU cost of the compositor policy work Ink would own: layout math
for N windows, heap versus `arena { }`, plus the timer floor and 1% low.

```sh
ink -C tests/perf/frames test -v -count=1
```

Not wired into `bin/check`: it prints numbers and asserts only that work
happened, so it is a measurement tool, not a gate. Compare runs on the same
machine; run `-count=3` and read the middle one.

Measured 2026-11-24, linux/arm64, no GPU, 64 windows, 3000 frames:

| variant | mean | p99 | 1% low |
|---|---|---|---|
| timer only | 0.04 us | 0.04 | 0.04 |
| reused slice | 0.18 us | 0.20 | 0.20 |
| heap alloc | 0.21 us | 0.24 | 0.43 |
| `arena` | 0.30 us | 0.40 | 2.20 |
| `arena` + copy out | 0.34 us | 0.36 | 2.00 |

The arena is ~120 ns slower than reusing a heap slice and ~85 ns slower than a
fresh allocation at this size: `Scratch.Begin/End` take a mutex pair, the block
pushes a cleanup closure onto `DeferStack`, and `End` clears the used region.
