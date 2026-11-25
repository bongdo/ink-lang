use fmt;

struct Point {
    X: i32,
    Y: i32,
};

fn sum() i32 {
    var total = i32(0);
    arena {
        var p = new[Point]();
        p.X = 3;
        var xs = i32[](2);
        xs[0] = 4;
        xs[1] = 5;
        total = p.X + xs[0] + xs[1];
    };
    return total;
};

fn main() void! {
    fmt.Println(sum())?;
    return;
};
