use fmt;
use errors;

type Pair struct {
    fn Split(n: i32) (i32, bool)! {
        if (n < 0) {
            return 0, false, errors.New("negative");
        };
        return n, true;
    };
};

fn passthrough(n: i32) (i32, bool)! {
    var p = Pair {};
    return p.Split(n)
        or fail;
};

fn main() void! {
    var p = Pair {};

    var a, b = p.Split(7)
        or fail;
    fmt.Println(a, b)
        or fail;

    var c, d = passthrough(8)
        or fail;
    fmt.Println(c, d)
        or fail;

    var x = i32(0);
    var y = false;
    x, y = p.Split(3)
        or fail;
    fmt.Println(x, y)
        or fail;

    var e, f = p.Split(-1)
        on err {
            fmt.Println("handled")
                or fail;
            return;
        };
    fmt.Println(e, f)
        or fail;
};
