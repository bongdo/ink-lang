use fmt;

fn main() void! {
    fmt.Println("start")
        or fail;
    if (true) {
        defer fmt.Println("block");
        fmt.Println("in block")
            or fail;
    };
    fmt.Println("after")
        or fail;
    for (var i = 0; i < 2; i += 1) {
        defer fmt.Println("iter", i);
        fmt.Println("body", i)
            or fail;
    };
    fmt.Println("end")
        or fail;
};
