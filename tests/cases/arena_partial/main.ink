use fmt;

fn main() void! {
    var total = i32(0);
    for (var f = 0; f < 2; f += 1) {
        arena {
            var xs = i32[](4);
            for (var y = 0; y < 4; y += 1) {
                if (y == 2 && f == 1) {
                    continue;
                };
                xs[y] = i32(y + 1);
            };;
            if (f == 0) {
                xs[2] = 9;
            };
            for (var y = 0; y < 4; y += 1) {
                total += xs[y];
            };;
        };
    };;
    fmt.Println(total)?;
};
