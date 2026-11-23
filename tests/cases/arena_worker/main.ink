use fmt;

fn main() void! {
    var total = i32(0);
    arena {
        var out = make[i32](4);
        go for (var y = i32(0); y < i32(4); y += 1) limit 2 {
            arena {
                var tmp = make[i32](2);
                tmp[0] = y * 10;
                out[y] = tmp[0];
            };
        };
        for (var i = 0; i < 4; i += 1) {
            total += out[i];
        };
    };
    fmt.Println(total)
        or fail;
};
