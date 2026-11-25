use fmt;

fn main() void! {
    var total = i32(0);
    for (var i = 0; i < 10; i += 1) {
        if (i == 3) {
            continue;
        };
        if (i == 8) {
            break;
        };
        total += i32(i);
    };;
    fmt.Println(total)?;
    var xs = i32[] { 10, 20, 30 };
    for (var i, x in xs) {
        total += i32(i) + x;
    };;
    fmt.Println(total)?;
    var m = map[str, i32] { "a": 1, "bb": 2 };
    for (var k, v in m) {
        total += v + i32(len(k));
    };;
    fmt.Println(total)?;
    var n = 0;
    for () {
        n += 1;
        if (n == 3) {
            break;
        };
    };;
    fmt.Println(n)?;
    var sub = xs[1:3];
    fmt.Println(sub)?;
};
