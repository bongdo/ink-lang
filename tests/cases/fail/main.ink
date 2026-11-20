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
        or err {
            fail(err, "handled");
        };
    return n;
};

fn inPar() i32! {
    par {
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
        or err {
            fmt.Println("wrapped:", err)
                or fail;
        };
    var b = i32(0);
    b = handled()
        or err {
            fmt.Println("handled:", err)
                or fail;
        };
    var c = i32(0);
    c = inPar()
        or err {
            fmt.Println("par:", err)
                or fail;
        };
    _ = a;
    _ = b;
    _ = c;
    return;
};
