use fmt;

fn main() void! {
    var word = "é";
    var b: byte = word[0];
    fmt.Println(b)
        or fail;
};
