# Arena frame

CPU rasterizes a frame into an arena-allocated pixel buffer, rows in parallel,
one texture upload per frame.

```sh
GOFLAGS=-tags=nocgo ink run
```

`-tags=nocgo` makes raylib-go use purego instead of cgo, so the example builds
without raylib headers. At runtime it still needs `libraylib` and a display.

The buffer lives exactly one frame:

```ink
arena {
    var f = new[Frame]();
    f.px = make[u8](W*H*4);
    var bands = workers * 4;
    par for (var b = i32(0); b < bands; b += 1) limit workers {
        fillRows(f, b * per, min((b+1) * per, i32(H)));
    };
    raylib.UpdateTexture(tex, f.px);
};
```

- `new` and `make` run before `par`; workers only write into the memory the
  arena already handed out, each into its own rows.
- Four bands per worker keep cores busy while one worker is slower; one
  goroutine per row costs more than it saves.
- `UpdateTexture` reads the buffer during the call and copies it to the GPU.
- `arena` frees the frame after the workers join and after the upload.

Never call raylib from a `par` worker: raylib is single-threaded.

480x270, 120 frames, 4 cores, `-tags=nocgo`, Go 1.27:

| Fill | Total | Per frame |
|---|---|---|
| sequential | 526 ms | 4.38 ms |
| `par for` per row, `limit 4` | 208 ms | 1.73 ms |
| 8 bands, `limit 4` | 172 ms | 1.43 ms |
