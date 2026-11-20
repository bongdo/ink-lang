use fmt;
use ./shape;

fn area(s: shape.Shape) f64 {
    return match (s) {
        case Circle { Radius } => 3.0 * Radius * Radius;
        case Empty => 0.0;
    };
};

fn main() void! {
    var s = shape.Shape.Circle { Radius: 2.0 };
    fmt.Println(area(s))
        or fail;
    var e = shape.Shape.Empty;
    fmt.Println(area(e))
        or fail;
};
