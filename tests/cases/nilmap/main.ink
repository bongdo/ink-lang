use fmt;

struct Node {
    value: i32,
    next: *Node,
};

fn main() void! {
    var p = nil[*Node];
    if (p == nil[*Node]) {
        fmt.Println("nil")?;
    };
    var m = map[str, i32] {};
    m["a"] = 1;
    fmt.Println(has(m, "a"), m["b"])?;
    delete(m, "a");
    fmt.Println(has(m, "a"))?;
    var xs = i32[] {};
    xs = append(xs, 1, 2);
    fmt.Println(len(xs), xs[1])?;
    var arr = i32[3] { 1, 2 };
    fmt.Println(arr)?;
};
