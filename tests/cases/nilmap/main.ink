use fmt;

struct Node {
    value: i32,
    next: *Node,
};

fn main() void! {
    var p = nil[*Node];
    if (p == nil[*Node]) {
        fmt.Println("nil")
            or fail;
    };
    var m = map[str, i32] {};
    m["a"] = 1;
    fmt.Println(has(m, "a"), m["b"])
        or fail;
    delete(m, "a");
    fmt.Println(has(m, "a"))
        or fail;
    var xs = i32[] {};
    xs = append(xs, 1, 2);
    fmt.Println(len(xs), xs[1])
        or fail;
    var arr = i32[3] { 1, 2 };
    fmt.Println(arr)
        or fail;
};
