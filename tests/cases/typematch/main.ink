use fmt;
use os;
use errors;

struct Point {
    x: i32,
};

fn describe(v: any) str {
    return match (v) {
        case i32 as n => "i32";
        case Point as p => "point";
        case *Point as p => "point-ptr";
        case bool => "bool";
        else => "other";
    };
};

fn kind(e: error) str {
    return match (e) {
        case *os.PathError as p => "path:" + p.Path;
        else => "other";
    };
};

fn main() void! {
    fmt.Println(describe(i32(42)))
        or fail;
    fmt.Println(describe(Point { x: 1 }))
        or fail;
    fmt.Println(describe(&Point { x: 2 }))
        or fail;
    fmt.Println(describe("s"))
        or fail;
    fmt.Println(describe(true))
        or fail;
    fmt.Println(kind(errors.New("boom")))
        or fail;
    fmt.Println(kind(&os.PathError { Op: "open", Path: "x", Err: errors.New("no") }))
        or fail;
};
