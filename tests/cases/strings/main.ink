use fmt;

fn main() void! {
    var r = runes("ab");
    fmt.Println(len(r), r[0])?;
    fmt.Println("ab"[1])?;
    var word = "é";
    fmt.Println(word[0], runes(word)[0])?;
    var b = bytes("hi");
    fmt.Println(len(b))?;
    var s = str(r);
    fmt.Println(s)?;
};
