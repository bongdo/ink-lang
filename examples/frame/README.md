# Arena frame

Rasterizes 480x270 into an arena-allocated row buffer, shades rows in parallel,
copies them into one persistent pixel buffer and uploads it once per frame.

```sh
GOFLAGS=-tags=nocgo ink run
```

`-tags=nocgo` makes raylib-go use purego instead of cgo, so the example builds
without raylib headers. At runtime it still needs `libraylib` and a display.

The per-frame part:

```ink
arena {
    var rows = make[Row](H);                      // Row = u8[480*4]
    par for (var y = i32(0); y < i32(H); y += 1) limit workers {
        rows[y] = shadeRow(y, t);                 // each worker owns its row
    };
    for (var y = 0; y < H; y += 1) {
        copy(px[y*RowLen:], rows[y][:]);          // the only way out
    };
};
rl.UpdateTexture(tex, px);
```

Why this shape:

- `shadeRow` takes scalars and returns a row by value, so arena memory never
  crosses a function boundary. The compiler rejects passing an arena value to a
  call, so a helper cannot take `*Frame` or the arena slice.
- `rows[y]` is indexed by the loop variable, the one write shape `par` can prove
  disjoint. Writing into a captured `px` from a worker is rejected.
- `copy` moves bytes into normal memory and `UpdateTexture` reads that buffer,
  never the arena.
- `new` and `make` run before `par`; raylib itself is called only on the main
  goroutine, because it is single-threaded.

480x270, 60 frames, 4 cores, `-tags=nocgo`, Go 1.27:

| Fill | Total | Per frame |
|---|---|---|
| sequential | 254 ms | 4.23 ms |
| `par for` per row, `limit 4` | 115 ms | 1.92 ms |

The row is heavy (480 pixels of `sin`+`cos`), so one goroutine per row pays for
itself. A cheap element per goroutine is slower than the sequential loop.
