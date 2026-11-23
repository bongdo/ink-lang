use fmt;

fn main() void! {
    var out = int[6] {};
    var i = 0;
    go for (i = 0; i < 6; i += 1) limit 2 {
        out[i] = i * 10;
    };
    fmt.Println(i)
        or fail;
    fmt.Println(out)
        or fail;
};
