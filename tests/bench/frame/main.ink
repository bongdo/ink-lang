use fmt;
use sort;
use time;

const W = 2560;
const H = 1440;
const RowLen = W * 4;
const Frames = 240;

alias Row = u8[RowLen];
alias Pix = u32[W];

fn shadeBytes(y: int, t: int) Row {
    var row = Row {};
    for (var x = 0; x < W; x += 1) {
        var v = u8((x * 31 + y * 17 + t * 13) & 255);
        var i = x * 4;
        row[i] = v;
        row[i + 1] = v >> 1;
        row[i + 2] = u8(255) - v;
        row[i + 3] = 255;
    };;
    return row;
};

fn shadePixRow(y: int, t: int) Pix {
    var row = Pix {};
    for (var x = 0; x < W; x += 1) {
        var v = u32((x * 31 + y * 17 + t * 13) & 255);
        row[x] = v | (v >> 1) << 8 | (u32(255) - v) << 16 | u32(255) << 24;
    };;
    return row;
};

fn shadePix(dst: u32[], y: int, t: int) void {
    for (var x = 0; x < W; x += 1) {
        var v = u32((x * 31 + y * 17 + t * 13) & 255);
        dst[x] = v | (v >> 1) << 8 | (u32(255) - v) << 16 | u32(255) << 24;
    };;
};

fn shadeLUT(dst: u32[], lut: u32[], y: int, t: int) void {
    var v = (y * 17 + t * 13) & 255;
    for (var x = 0; x < W; x += 1) {
        dst[x] = lut[v];
        v = (v + 31) & 255;
    };;
};

fn p50(xs: f64[]) f64 {
    return xs[len(xs) / 2];
};

fn p99(xs: f64[]) f64 {
    return xs[len(xs) * 99 / 100];
};

fn report(name: str, xs: f64[]) void! {
    sort.Float64s(xs);
    fmt.Println(name, "p50", p50(xs), "p99", p99(xs), "max", xs[Frames - 1])?;
};

fn main() void! {
    var pxA = u8[](RowLen * H);
    var pxB = u32[](W * H);
    var pxC = u32[](W * H);
    var pxD = u32[](W * H);
    var lut = u32[](256);
    for (var v = 0; v < 256; v += 1) {
        var u = u32(v);
        lut[v] = u | (u >> 1) << 8 | (u32(255) - u) << 16 | u32(255) << 24;
    };;
    var a = f64[](Frames);
    var b = f64[](Frames);
    var c = f64[](Frames);
    var d = f64[](Frames);

    for (var f = 0; f < Frames; f += 1) {
        var start = time.Now();
        arena {
            var rows = Row[](H);
            par for (var y = 0; y < H; y += 1) limit 4 {
                rows[y] = shadeBytes(y, f);
            };;
            for (var y = 0; y < H; y += 1) {
                copy(pxA[y * RowLen:], rows[y][:]);
            };;
        };
        a[f] = f64(time.Since(start).Microseconds()) / 1000.0;
    };;

    for (var f = 0; f < Frames; f += 1) {
        var start = time.Now();
        arena {
            var rows = Pix[](H);
            par for (var y = 0; y < H; y += 1) limit 4 {
                rows[y] = shadePixRow(y, f);
            };;
            for (var y = 0; y < H; y += 1) {
                copy(pxB[y * W:], rows[y][:]);
            };;
        };
        b[f] = f64(time.Since(start).Microseconds()) / 1000.0;
    };;

    for (var f = 0; f < Frames; f += 1) {
        var start = time.Now();
        par for (var y = 0; y < H; y += 1) limit 4 {
            shadePix(pxC[y * W:], y, f);
        };;
        c[f] = f64(time.Since(start).Microseconds()) / 1000.0;
    };;

    for (var f = 0; f < Frames; f += 1) {
        var start = time.Now();
        par for (var y = 0; y < H; y += 1) limit 4 {
            shadeLUT(pxD[y * W:], lut, y, f);
        };;
        d[f] = f64(time.Since(start).Microseconds()) / 1000.0;
    };;

    report("A arena u8 + copy ", a)?;
    report("B arena u32 + copy", b)?;
    report("C u32 direct      ", c)?;
    report("D u32 + LUT shader", d)?;

    var sumA = u64(0);
    for (var y = 0; y < H; y += 1) {
        for (var x = 0; x < RowLen; x += 1) {
            sumA += u64(pxA[y * RowLen + x]);
        };;
    };;
    var sumC = u64(0);
    for (var y = 0; y < H; y += 1) {
        for (var x = 0; x < W; x += 1) {
            sumC += u64(pxC[y * W + x]);
        };;
    };;
    fmt.Println("sum bytes", sumA, "sum u32", sumC)?;
};

test "u32 pixel matches four byte stores" {
    var brow = shadeBytes(7, 3);
    var pix = shadePixRow(7, 3);
    for (var x = 0; x < W; x += 1) {
        var w = pix[x];
        assert_eq(u8(w & 255), brow[x * 4]);
        assert_eq(u8((w >> 8) & 255), brow[x * 4 + 1]);
        assert_eq(u8((w >> 16) & 255), brow[x * 4 + 2]);
        assert_eq(u8(w >> 24), brow[x * 4 + 3]);
    };;
};

test "LUT matches the arithmetic shader" {
    var lut = u32[](256);
    for (var v = 0; v < 256; v += 1) {
        var u = u32(v);
        lut[v] = u | (u >> 1) << 8 | (u32(255) - u) << 16 | u32(255) << 24;
    };;
    var got = Pix {};
    shadeLUT(got[:], lut, 7, 3);
    var want = shadePixRow(7, 3);
    for (var x = 0; x < W; x += 1) {
        assert_eq(got[x], want[x]);
    };;
};
