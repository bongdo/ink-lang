use fmt;

type Point struct {
    X: i32,
    Y: i32,
};

fn main() void! {
    var total = i32(0);
    for (var i = 0; i < 3; i += 1) {
        arena {
            var p = new[Point]();
            var xs = i32[](4);
            total += p.X + xs[0];
            p.X = 7;
            xs[0] = 7;
        };
    };
    fmt.Println(total)
        or fail;
};
