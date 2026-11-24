use fmt;
use sort;
use time;

const W = 2560;

const H = 1440;

const RowLen = W * 4;

const Frames = 240;

type Row = u8[RowLen];

fn shadeRow(y: int, t: int) Row {
    var row = Row {};
    for (var x = 0; x < W; x += 1) {
        var v = u8((x * 31 + y * 17 + t * 13) & 255);
        var i = x * 4;
        row[i] = v;
        row[i + 1] = v >> 1;
        row[i + 2] = u8(255) - v;
        row[i + 3] = 255;
    };
    return row;
};

fn p50(xs: f64[]) f64 {
    return xs[len(xs) / 2];
};

fn p99(xs: f64[]) f64 {
    return xs[len(xs) * 99 / 100];
};

fn main() void! {
    var px = u8[](RowLen * H);
    var total = f64[](Frames);
    var t0 = time.Now();

    for (var f = 0; f < Frames; f += 1) {
        var start = time.Now();
        arena {
            var rows = Row[](H);
            go for (var y = 0; y < H; y += 1) limit 4 {
                rows[y] = shadeRow(y, f);
            };
            for (var y = 0; y < H; y += 1) {
                copy(px[y * RowLen:], rows[y][:]);
            };
        };
        total[f] = f64(time.Since(start).Microseconds()) / 1000.0;
    };

    var sum = u64(0);
    for (var i = 0; i < len(px); i += 1) {
        sum += u64(px[i]);
    };

    sort.Float64s(total);
    fmt.Println("frames", Frames)
        or fail;
    fmt.Println("avg ms", f64(time.Since(t0).Milliseconds()) / f64(Frames))
        or fail;
    fmt.Println("p50 ms", p50(total), "p99 ms", p99(total), "max ms", total[Frames - 1])
        or fail;
    fmt.Println("sum", sum)
        or fail;
};
