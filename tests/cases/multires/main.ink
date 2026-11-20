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

fn pass() (i32, bool)! {
    var p = Pair {};
    return p.Split(4);
};

fn main() void! {
    var p = Pair {};
    var n, ok, err = p.Split(7);
    assert(err == nil);
    fmt.Println(n, ok)
        or fail;

    var a, b, err2 = pass();
    assert(err2 == nil);
    fmt.Println(a, b)
        or fail;

    var c, d, err3 = p.Split(-1);
    if (err3 != nil) {
        fmt.Println("err", err3)
            or fail;
        return nil;
    };
    fmt.Println(c, d)
        or fail;
};
