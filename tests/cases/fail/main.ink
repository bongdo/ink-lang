fn bad() i32! {
    check(false, "boom");
};

fn wrapped() i32! {
    var n = bad()?;
    return n;
};

fn handled() i32! {
    var n = bad()
        on err {
            print("handled:", err);
        };
    return n;
};

fn inGo() i32! {
    par {
        {
            var n = bad()?;
            _ = n;
        };
    };
    return 0;
};

fn main() void! {
    var a = i32(0);
    a = wrapped()
        on err {
            print("wrapped:", err);
        };
    var b = i32(0);
    b = handled()
        on err {
            print("handled:", err);
        };
    var c = i32(0);
    c = inGo()
        on err {
            print("spawn:", err);
        };
    _ = a;
    _ = b;
    _ = c;
    return;
};
