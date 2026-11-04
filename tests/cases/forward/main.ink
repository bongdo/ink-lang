use fmt;

var a = b;

var b = c;

var c = i32(7);

fn main() void or error {
    fmt.Println(a, b, c) or return;
};
