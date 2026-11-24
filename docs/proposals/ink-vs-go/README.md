# Ink vs hand-written Go: 2.5K frame

Comparison of the same frame loop written in Ink and in Go. Numbers are from
this machine (4 cores, Go 1.27), 240 frames, no raylib. They are noise-sensitive:
run each binary several times.

## Lines

| File | Lines | Non-empty | What it is |
|---|---:|---:|---|
| `tests/bench/frame/main.ink` | 69 | 57 | Ink source |
| `tests/bench/frame/zz_ink.go` | 208 | 146 | generated Go, without `//line` |
| `direct/main.go` | 69 | 61 | hand Go, writes rows straight into `px` |
| `scratch/main.go` | 73 | 65 | hand Go, rows buffer + copy (Ink shape) |

The generated file includes two copies of `shadeRow` (original + `_s_into_`),
chunk partitioning, `WaitGroup`, per-worker `DeferStack`, timings and the main
wrapper. `//line` directives add 62 lines.

## Results

| Version | p50 | p99 | Notes |
|---|---:|---:|---|
| Ink (generated Go) | **3.1 ms** | 5–7 ms | arena + `go for` + copy, fused |
| hand Go, same shape | 3.3 ms | 6–14 ms | rows buffer + copy, no fusion |
| hand Go, direct | **2.3 ms** | 4.5–4.9 ms | workers write straight into `px` |
| C, one thread | 1.5 ms | — | earlier synthetic fill, not this harness |

All versions produce the same checksum `2114150400`.

## Why the direct Go is faster

It removes the intermediate row buffer and the copy: workers write into the
persistent upload buffer, each worker owns a contiguous range. There is no
arena, no zeroing, no second pass.

Ink forbids exactly this shape: arena memory cannot leave the block, and a
captured buffer cannot be written from `go for`. That rule is what makes reuse
and the arena safe without escape analysis. The price is the copy: roughly
0.8 ms here.

## Why Ink is not slower than the same-shape Go

Ink's chunking, empty-mark removal, row-copy fusion and copy elision are not in
the hand-written same-shape version. Without them, the same shape is 3.3 ms;
Ink is 3.1 ms. So the compiler work is real, not a wash.

## What Ink actually removes

The Ink frame loop is:

```ink
arena {
    var rows = Row[](H);
    go for (var y = 0; y < H; y += 1) limit 4 {
        rows[y] = shadeRow(y, f);
    };
    for (var y = 0; y < H; y += 1) {
        copy(px[y * RowLen:], rows[y][:]);
    };
};
```

The equivalent Go needs `runtime.GOMAXPROCS`, `sync.WaitGroup`, chunk bounds,
`defer wg.Done`, and the copy loop. Ink also handles arena reuse, zeroing,
cleanup on `return`/`break`/`panic`, and the result copy.

## Verdict

- Ink is not a faster language than hand-written Go; on the same shape it is
  slightly faster only because of the compiler passes above.
- The safe shape costs ~0.8 ms against an unsafe direct Go loop. If the product
  can accept "workers write the final buffer directly", plain Go wins.
- The direct Go version is 69 lines, the same as the Ink source, but the
  concurrency is explicit. Ink hides ~15 lines of worker machinery and a class
  of races; it does not hide the shader or the memory traffic.
- C/SIMD is the only route to ~1.5 ms; Go without SIMD stops around 2.3 ms with
  four workers, and Ink around 3.1 ms with the arena rule.

## Run

```sh
cd docs/proposals/ink-vs-go
go run ./direct
go run ./scratch
```

```sh
cd tests/bench/frame
ink run
```
