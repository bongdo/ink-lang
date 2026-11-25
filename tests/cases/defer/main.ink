use fmt;

fn main() void! {
    fmt.Println("start")?;
    if (true) {
        defer fmt.Println("block");
        fmt.Println("in block")?;
    };
    fmt.Println("after")?;
    for (var i = 0; i < 2; i += 1) {
        defer fmt.Println("iter", i);
        fmt.Println("body", i)?;
    };;
    fmt.Println("end")?;
};
