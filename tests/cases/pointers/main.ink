use fmt;

struct Point {
    x: i32,
};

fn byValue(p: Point) void {
    p.x = 99;
};

fn bump(p: *Point) void {
    p.x += 1;
};

fn main() void! {
    var a = Point { x: 1 };
    var b = a;
    b.x = 2;
    fmt.Println("copy", a.x, b.x)
        or fail;

    byValue(a);
    fmt.Println("param", a.x)
        or fail;

    var p = &a;
    p.x = 7;
    bump(p);
    fmt.Println("ptr", a.x, *p)
        or fail;

    var q = &a;
    fmt.Println("alias", p == q)
        or fail;

    var snap = *p;
    p.x = 8;
    fmt.Println("snapshot", snap.x, a.x)
        or fail;

    var f = &a.x;
    *f = 5;
    fmt.Println("field", a.x)
        or fail;

    var xs = i32[] { 1, 2 };
    var e = &xs[1];
    *e = 6;
    fmt.Println("elem", xs[1])
        or fail;
};
