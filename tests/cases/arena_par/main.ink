use fmt;

type Frame struct {
    px: i32[],
    rows: i32,
};

fn fill(f: *Frame, y: i32) void {
    var row = y * f.rows;
    for (var x = i32(0); x < f.rows; x += 1) {
        f.px[row + x] = y * 10 + x;
    };
};

fn main() void! {
    var sum = i32(0);
    arena {
        var f = new[Frame]();
        f.rows = 4;
        f.px = make[i32](16);
        par for (var y = i32(0); y < i32(4); y += 1) limit 2 {
            fill(f, y);
        };
        var xs = f.px[:];
        for (var i = 0; i < len(xs); i += 1) {
            sum += xs[i];
        };
        arena {
            var tmp = make[i32](2);
            tmp[0] = 7;
            sum += tmp[0];
        };
    };
    fmt.Println(sum)
        or fail;
};
