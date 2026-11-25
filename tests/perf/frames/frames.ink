use fmt;
use sort;
use time;

const Frames = 3000;
const Windows = 64;

struct Rect {
    X: f64,
    Y: f64,
    W: f64,
    H: f64,
};

fn tile(i: int, n: int, w: f64, h: f64, gap: f64) Rect {
    var each = (h - gap * f64(n + 1)) / f64(n);
    return Rect {
        X: gap,
        Y: gap + f64(i) * (each + gap),
        W: w - gap * 2.0,
        H: each,
    };
};

struct Stats {
    Mean: f64,
    P50: f64,
    P99: f64,
    Low1: f64,
    Fps: f64,
    LowFps: f64,
};

fn stats(xs: f64[]) Stats {
    var ys = f64[](len(xs));
    copy(ys, xs);
    sort.Float64s(ys);
    var sum = 0.0;
    for (var x in ys) {
        sum += x;
    };;
    var mean = sum / f64(len(ys));
    var lowN = len(ys) / 100;
    if (lowN < 1) {
        lowN = 1;
    };
    var lowSum = 0.0;
    for (var i = len(ys) - lowN; i < len(ys); i += 1) {
        lowSum += ys[i];
    };;
    var low = lowSum / f64(lowN);
    return Stats {
        Mean: mean,
        P50: ys[len(ys) / 2],
        P99: ys[(len(ys) * 99) / 100],
        Low1: low,
        Fps: 1000.0 / mean,
        LowFps: 1000.0 / low,
    };
};

fn report(name: str, s: Stats) void {
    var line = fmt.Sprintf("%-12s mean %7.3f us  p50 %7.3f  p99 %7.3f  1%%low %7.3f  cap %6.0f fps  1%%low %6.0f fps", name, s.Mean, s.P50, s.P99, s.Low1, s.Fps, s.LowFps);
    _ = fmt.Println(line);
};

fn main() void {
    _ = fmt.Println("run `ink test -v`");
};

test "baseline: timer only" {
    var xs = f64[](0, cap: Frames);
    for (var f = 0; f < Frames; f += 1) {
        var t0 = time.Now();
        var us = f64(time.Since(t0).Nanoseconds()) / 1000.0;
        xs = append(xs, us);
    };;
    report("timer", stats(xs));
};

test "frame: heap alloc" {
    var xs = f64[](0, cap: Frames);
    for (var f = 0; f < Frames; f += 1) {
        var t0 = time.Now();
        var rects = Rect[](Windows);
        for (var i = 0; i < Windows; i += 1) {
            rects[i] = tile(i, Windows, 1920.0, 1080.0, 8.0);
        };;
        var acc = 0.0;
        for (var r in rects) {
            acc += r.W + r.H;
        };;
        var us = f64(time.Since(t0).Nanoseconds()) / 1000.0;
        xs = append(xs, us);
        assert(acc > 0.0);
    };;
    report("heap", stats(xs));
};

test "frame: arena" {
    var xs = f64[](0, cap: Frames);
    for (var f = 0; f < Frames; f += 1) {
        var t0 = time.Now();
        var acc = 0.0;
        arena {
            var rects = Rect[](Windows);
            for (var i = 0; i < Windows; i += 1) {
                rects[i] = tile(i, Windows, 1920.0, 1080.0, 8.0);
            };;
            for (var r in rects) {
                acc += r.W + r.H;
            };;
        };
        var us = f64(time.Since(t0).Nanoseconds()) / 1000.0;
        xs = append(xs, us);
        assert(acc > 0.0);
    };;
    report("arena", stats(xs));
};

test "frame: reused slice" {
    var rects = Rect[](Windows);
    var xs = f64[](0, cap: Frames);
    for (var f = 0; f < Frames; f += 1) {
        var t0 = time.Now();
        for (var i = 0; i < Windows; i += 1) {
            rects[i] = tile(i, Windows, 1920.0, 1080.0, 8.0);
        };;
        var acc = 0.0;
        for (var r in rects) {
            acc += r.W + r.H;
        };;
        var us = f64(time.Since(t0).Nanoseconds()) / 1000.0;
        xs = append(xs, us);
        assert(acc > 0.0);
    };;
    report("reused", stats(xs));
};

test "frame: heap 256 windows" {
    var xs = f64[](0, cap: Frames);
    for (var f = 0; f < Frames; f += 1) {
        var t0 = time.Now();
        var rects = Rect[](256);
        for (var i = 0; i < 256; i += 1) {
            rects[i] = tile(i, 256, 3840.0, 2160.0, 4.0);
        };;
        var acc = 0.0;
        for (var r in rects) {
            acc += r.W + r.H;
        };;
        var us = f64(time.Since(t0).Nanoseconds()) / 1000.0;
        xs = append(xs, us);
        assert(acc > 0.0);
    };;
    report("heap256", stats(xs));
};

test "frame: arena 256 windows" {
    var xs = f64[](0, cap: Frames);
    for (var f = 0; f < Frames; f += 1) {
        var t0 = time.Now();
        var acc = 0.0;
        arena {
            var rects = Rect[](256);
            for (var i = 0; i < 256; i += 1) {
                rects[i] = tile(i, 256, 3840.0, 2160.0, 4.0);
            };;
            for (var r in rects) {
                acc += r.W + r.H;
            };;
        };
        var us = f64(time.Since(t0).Nanoseconds()) / 1000.0;
        xs = append(xs, us);
        assert(acc > 0.0);
    };;
    report("arena256", stats(xs));
};

test "frame: arena + copy out" {
    var xs = f64[](0, cap: Frames);
    var out = Rect[](Windows);
    for (var f = 0; f < Frames; f += 1) {
        var t0 = time.Now();
        arena {
            var rects = Rect[](Windows);
            for (var i = 0; i < Windows; i += 1) {
                rects[i] = tile(i, Windows, 1920.0, 1080.0, 8.0);
            };;
            copy(out, rects);
        };
        var acc = 0.0;
        for (var r in out) {
            acc += r.W + r.H;
        };;
        var us = f64(time.Since(t0).Nanoseconds()) / 1000.0;
        xs = append(xs, us);
        assert(acc > 0.0);
    };;
    report("arena+copy", stats(xs));
};
