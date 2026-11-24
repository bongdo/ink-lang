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
    var rows = Row[](H);                         // Row = u8[480*4]
    go for (var y = i32(0); y < i32(H); y += 1) limit workers {
        rows[y] = shadeRow(y, t);                // each worker owns its row
    };
    for (var y = 0; y < H; y += 1) {
        copy(px[y*RowLen:], rows[y][:]);         // publish to the upload buffer
    };
};
rl.UpdateTexture(tex, px);
```

`rows` comes from a per-type scratch buffer: the first frame allocates it, every
next frame reuses the same memory, cleared and handed out again. Nothing reaches
the GC while the loop runs, and the block starts zeroed. The buffer is released
after the loop ends. So the frame loop stops allocating after the first frame.

Why else this shape:

- `shadeRow` takes scalars and returns a row by value, so arena memory never
  crosses a function boundary.
- `rows[y]` is indexed by the loop variable, the one write shape `go` can prove
  disjoint. Writing into a captured `px` from a worker is rejected.
- `new` and `Row[](n)` run before `go`; a worker shares the block's batch.
- raylib is called only on the main goroutine, because it is single-threaded.

The pixel buffer `px` stays outside the arena: it lives across frames and is
passed to `UpdateTexture`, and arena memory cannot leave the block. `copy` is the
boundary.
