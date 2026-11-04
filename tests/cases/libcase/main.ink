use fmt;
use "./lib";

fn main() void or error {
    fmt.Println(lib.Square(7)) or return;
};
