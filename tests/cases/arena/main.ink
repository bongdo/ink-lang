use fmt;

type Point struct {
    X: i32,
    Y: i32,
};

fn sum() i32 {
    var total = i32(0);
    arena a {
        var p = a.New[Point]();
        p.X = 3;
        var xs = a.MakeSlice[i32](2);
        xs[0] = 4;
        xs[1] = 5;
        total = p.X + xs[0] + xs[1];
    };
    return total;
};

fn main() void! {
    fmt.Println(sum())
        or fail;
    return;
};
