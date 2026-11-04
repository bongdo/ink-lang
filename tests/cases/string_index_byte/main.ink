use fmt;

fn main() void or error {
    var word = "é";
    var b: byte = word[0];
    fmt.Println(b) or return;
};
