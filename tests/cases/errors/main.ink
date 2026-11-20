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
        or err {
            fmt.Println("caught:", err)
                or panic("print");
        };
    fmt.Println(w)
        or fail;
    var z = load(true)
        or panic("unreachable");
    fmt.Println(z)
        or fail;
};
