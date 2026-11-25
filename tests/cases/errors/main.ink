use fmt;

fn load(ok: bool) i32! {
    check(ok, "boom");
    return 42;
};

fn main() void! {
    var v = load(true)?;
    fmt.Println(v)?;
    var w = i32(0);
    w = load(false)
        on err { fmt.Println("caught:", err)?; };
    fmt.Println(w)?;
    var z = load(true)?;
    fmt.Println(z)?;
};
