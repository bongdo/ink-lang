use fmt;
use errors;

fn load(ok: bool) i32! {
    if (!ok) {
        return errors.New("boom");
    };
    return 42;
};

fn main() void! {
    var v = load(true)
        or fail;
    fmt.Println(v)
        or fail;
    var w = i32(0);
    w = load(false)
        on err {
            fmt.Println("caught:", err)
                or fail("print");
        };
    fmt.Println(w)
        or fail;
    var z = load(true)
        or fail("unreachable");
    fmt.Println(z)
        or fail;
};
