use fmt;

fn mk(n: i32) i32[] {
    var xs = make[i32](n);
    xs[0] = 1;
    return xs;
};

fn bad() void! {
    fail(error("boom"));
};

fn main() void! {
    var xs = mk(1);
    fmt.Println(xs[0])
        or fail;
    bad()
        or err {
            fmt.Println(err)
                or fail;
        };
    return;
};
