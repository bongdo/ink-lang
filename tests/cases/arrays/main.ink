use fmt;
use ./lib;

struct Point {
    x: i32,
    y: i32,
};

alias Byte = u8;

struct Stack[Item] {
    data: Item[],

    fn Push(x: Item) void {
        data = append(data, x);
    };
};

const N = 3;

fn main() void! {
    var points = Point[N] {};
    points[0] = Point { x: 1, y: 2 };
    points[1] = Point { x: 3, y: 4 };
    points[2] = Point { x: 5, y: 6 };
    fmt.Println(len(points))?;
    fmt.Println(points[1].x + points[2].y)?;

    var imported = Point[lib.N] {};
    imported[2] = Point { x: 12, y: 13 };
    fmt.Println(len(imported), imported[2].y)?;

    var raw = Byte[N] {};
    raw[0] = 7;
    raw[2] = 9;
    fmt.Println(len(raw), raw[0], raw[2])?;

    var fixed = Point[2] {};
    fixed[1] = Point { x: 8, y: 9 };
    fmt.Println(fixed[1].x)?;

    var stack = Stack[i32] {};
    stack.Push(11);
    fmt.Println(len(stack.data))?;
};
