use fmt;

fn load(x: i32) i32! {
    return x * 10;
};

fn main() void! {
    var a = i32(0);
    var b = i32(0);
    par {
        a = load(1)?;
        b = load(2)?;
    };
    fmt.Println(a, b)?;
    var xs = i32[] { 1, 2, 3, 4 };
    var out = i32[4] {};
    par for (var i = 0; i < len(xs); i += 1) limit 2 {
        out[i] = xs[i] * 2;
    };;
    fmt.Println(out)?;
    var sum = i32(0);
    sum = load(3)?;
    fmt.Println(sum)?;
};
