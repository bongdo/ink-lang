use fmt;

fn fill(xs: i32[]) void {
    xs[0] = 42;
};

fn main() void! {
    var a = i32[] { 1, 2, 3 };
    var b = a;
    b[0] = 9;
    fmt.Println("alias", a[0], b[0])?;

    fill(a);
    fmt.Println("param", a[0])?;

    var c = i32[] { 1, 2, 3 };
    var d = append(c, 4);
    d[0] = 8;
    fmt.Println("grow", c[0], d[0], len(c), len(d))?;

    var m = i32[](2, cap: 4);
    m[0] = 1;
    m[1] = 2;
    var n = append(m, 3);
    n[0] = 5;
    fmt.Println("cap", m[0], n[0], len(m), cap(m))?;

    var s = a[1:];
    s[0] = 6;
    fmt.Println("slice", a[1], len(s), cap(s))?;

    var dst = i32[2] {};
    var copied = copy(dst[:], a);
    dst[0] = 7;
    fmt.Println("copy", copied, a[0], dst[0])?;

    var empty: i32[] = nil;
    empty = append(empty, 1, 2);
    assert(cap(empty) >= len(empty));
    fmt.Println("nil", len(empty), empty[1])?;

    var spread = append(i32[] { 0 }, a...);
    fmt.Println("spread", spread)?;

    var zeros = i32[3] {};
    fmt.Println("array", zeros[0], len(zeros))?;
};
