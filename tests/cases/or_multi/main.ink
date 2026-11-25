use fmt;

struct Pair {
    fn Split(n: i32) (i32, bool)! {
        check(n >= 0, "negative");
        return n, true;
    };
};

fn passthrough(n: i32) (i32, bool)! {
    var p = Pair {};
    return p.Split(n)?;
};

fn main() void! {
    var p = Pair {};

    var a, b = p.Split(7)?;
    fmt.Println(a, b)?;

    var c, d = passthrough(8)?;
    fmt.Println(c, d)?;

    var x = i32(0);
    var y = false;
    x, y = p.Split(3)?;
    fmt.Println(x, y)?;

    var e, f = p.Split(-1)
        on err {
            fmt.Println("handled")?;
            return;
        };
    fmt.Println(e, f)?;
};
