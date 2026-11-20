use fmt;
use ./lib;

type Point struct {
    x: i32,
    y: i32,
};

type Byte = u8;

type Stack[Item] struct {
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
    fmt.Println(len(points))
        or fail;
    fmt.Println(points[1].x + points[2].y)
        or fail;

    var imported = Point[lib.N] {};
    imported[2] = Point { x: 12, y: 13 };
    fmt.Println(len(imported), imported[2].y)
        or fail;

    var bytes = Byte[N] {};
    bytes[0] = 7;
    bytes[2] = 9;
    fmt.Println(len(bytes), bytes[0], bytes[2])
        or fail;

    var fixed = Point[2] {};
    fixed[1] = Point { x: 8, y: 9 };
    fmt.Println(fixed[1].x)
        or fail;

    var stack = Stack[i32] {};
    stack.Push(11);
    fmt.Println(len(stack.data))
        or fail;
};
