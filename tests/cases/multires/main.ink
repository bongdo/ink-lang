use fmt;

struct Pair {
    fn Split(n: i32) (i32, bool)! {
        check(n >= 0, "negative");
        return n, true;
    };
};

fn pass() (i32, bool)! {
    var p = Pair {};
    return p.Split(4);
};

fn main() void! {
    var p = Pair {};
    var n, ok, err = p.Split(7);
    assert(err == nil);
    fmt.Println(n, ok)?;

    var a, b, err2 = pass();
    assert(err2 == nil);
    fmt.Println(a, b)?;

    var c, d, err3 = p.Split(-1);
    if (err3 != nil) {
        fmt.Println("err", err3)?;
        return nil;
    };
    fmt.Println(c, d)?;
};
