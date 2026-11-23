use fmt;

fn f(n: i32, p: *i32) str {
    match (n) {
        case -1 => {
            return "neg";
        };
        else => {};
    };
    return match (p) {
        case nil => "nil";
        else => "set";
    };
};

fn main() void! {
    var x = i32(1);
    fmt.Println(f(-1, nil[*i32]))
        or fail;
    fmt.Println(f(0, &x))
        or fail;
};
