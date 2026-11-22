# Arena frame

Rasterizes 480x270 into a row buffer, shades rows in parallel, copies them into
one persistent pixel buffer and uploads it once per frame.

```sh
GOFLAGS=-tags=nocgo ink run
```

`-tags=nocgo` makes raylib-go use purego instead of cgo, so the example builds
without raylib headers. At runtime it still needs `libraylib` and a display.

The per-frame part:

```ink
par for (var y = i32(0); y < i32(H); y += 1) limit workers {
    rows[y] = shadeRow(y, t);            // each worker owns its row
};
for (var y = 0; y < H; y += 1) {
    copy(px[y*RowLen:], rows[y][:]);     // arena-free publish step
};
rl.UpdateTexture(tex, px);
```

`rows` and `px` are allocated once, before the loop. The same code with an
`arena` around `rows` compiles and runs, but `ink check` warns:

```text
arena in a loop allocates 518400 bytes per iteration; allocate the buffer
before the loop and reuse it
```

Measured, 480x270, 240 frames, 4 cores:

| Per-frame buffer | p50 | p99 | GC cycles |
|---|---|---|---|
| `arena` block | 11.1 ms | 15.3 ms | 81 |
| allocated before the loop | 10.2 ms | 13.2 ms | 2 |

So a fixed-size frame buffer is not an arena case: the allocation is the same
every frame, and Go's arena still counts it as an allocation. Arenas pay off for
per-frame data whose size changes (visible objects, parsed input), where a fixed
buffer would have to grow.

Why else this shape:

- `shadeRow` takes scalars and returns a row by value.
- `rows[y]` is indexed by the loop variable, the one write shape `par` can prove
  disjoint. Writing into a captured `px` from a worker is rejected.
- raylib is called only on the main goroutine, because it is single-threaded.
