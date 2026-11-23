use fmt;

fn load(x: i32) i32! {
    return x * 10;
};

fn main() void! {
    var a = i32(0);
    var b = i32(0);
    go {
        a = load(1)
            or fail;
        b = load(2)
            or fail;
    };
    fmt.Println(a, b)
        or fail;
    var xs = i32[] { 1, 2, 3, 4 };
    var out = i32[4] {};
    go for (var i = 0; i < len(xs); i += 1) limit 2 {
        out[i] = xs[i] * 2;
    };
    fmt.Println(out)
        or fail;
    var sum = i32(0);
    sum = load(3)
        or fail;
    fmt.Println(sum)
        or fail;
};
