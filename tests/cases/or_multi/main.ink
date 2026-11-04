use fmt;
use errors;

type Pair struct {
    fn Split(n: i32) i32, bool or error {
        if (n < 0) {
            return 0, false, errors.New("negative");
        };
        return n, true;
    };
};

fn passthrough(n: i32) i32, bool or error {
    var p = Pair {};
    return p.Split(n) or return;
};

fn main() void or error {
    var p = Pair {};

    var a, b = p.Split(7) or return;
    fmt.Println(a, b) or return;

    var c, d = passthrough(8) or return;
    fmt.Println(c, d) or return;

    var x = i32(0);
    var y = false;
    x, y = p.Split(3) or return;
    fmt.Println(x, y) or return;

    var e, f = p.Split(-1) or err {
        fmt.Println("handled") or return;
        return;
    };
    fmt.Println(e, f) or return;
};
