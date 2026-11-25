use fmt;

fn fill(y: int, f: int) i32 {
    return i32(y * 10 + f);
};

fn main() void! {
    var total = i32(0);
    for (var f = 0; f < 3; f += 1) {
        arena {
            var xs = i32[](5);
            par for (var y = 0; y < len(xs); y += 1) limit 2 {
                xs[y] = fill(y, f);
            };;
            for (var y = 0; y < len(xs); y += 1) {
                total += xs[y];
            };;
        };
    };;
    fmt.Println(total)?;
};
