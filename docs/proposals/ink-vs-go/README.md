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
| C, one thread, direct | 1.0 ms | 1.2 ms | `-O3 -march=native`, compiler vectorizes |
| C, four threads, hot | **0.28 ms** best, 0.37 median | 0.57–0.97 ms | contiguous chunks, buffer in cache |
| C, four threads, cold | 0.57 ms | 0.88 ms | cache evicted before each frame |

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
- C is not at 1.5 ms either: the optimized direct version is **0.28 ms hot /
  0.57 ms cold**. The 1.5 ms number was one thread plus a per-row `memcpy`.
- Go without SIMD stops around 2.3 ms with four workers, Ink around 3.1 ms with
  the arena rule. The gap to C is mostly vectorization plus the extra copy, not
  the language syntax.

## The C ceiling

The earlier C number (1.5 ms) was one thread, a row buffer and `memcpy` per row.
The direct version writes into `px`, lets the compiler vectorize, and splits
rows over four pthreads:

```sh
cd docs/proposals/ink-vs-go/c
gcc -O3 -march=native -funroll-loops -pthread main.c -o c_frame
./c_frame 4          # hot
./c_frame 4 cold     # evict the buffer before every frame
```

Four threads: **0.28 ms p50 best, 0.37 ms median** when the 14.7 MB buffer stays
in cache; **0.57 ms p50** cold. Store-only is ~0.18 ms hot and ~0.26 ms cold, so
the shader is close to the memory floor. One thread is ~1.0 ms; without
auto-vectorization it is ~3.6 ms.

Hand-written NEON was slower than the compiler's auto-vectorized scalar loop
(3.2 ms vs 1.0 ms single-thread). On this CPU, let the compiler vectorize.

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
