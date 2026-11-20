use fmt;

fn main() void! {
    var r = runes("ab");
    fmt.Println(len(r), r[0])
        or fail;
    fmt.Println("ab"[1])
        or fail;
    var word = "é";
    fmt.Println(word[0], runes(word)[0])
        or fail;
    var b = bytes("hi");
    fmt.Println(len(b))
        or fail;
    var s = str(r);
    fmt.Println(s)
        or fail;
};
