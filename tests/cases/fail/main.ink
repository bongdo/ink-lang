use fmt;
use errors;

fn bad() i32! {
    return errors.New("boom");
};

fn wrapped() i32! {
    var n = bad()
        or fail("ctx");
    return n;
};

fn handled() i32! {
    var n = bad()
        on err {
            fail(err, "handled");
        };
    return n;
};

fn inGo() i32! {
    go {
        {
            var n = bad()
                or fail("worker");
            _ = n;
        };
    };
    return 0;
};

fn main() void! {
    var a = i32(0);
    a = wrapped()
        on err {
            fmt.Println("wrapped:", err)
                or fail;
        };
    var b = i32(0);
    b = handled()
        on err {
            fmt.Println("handled:", err)
                or fail;
        };
    var c = i32(0);
    c = inGo()
        on err {
            fmt.Println("go:", err)
                or fail;
        };
    _ = a;
    _ = b;
    _ = c;
    return;
};
