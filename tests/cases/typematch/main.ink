use fmt;
use os;

fn boom() void! {
    check(false, "boom");
};

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
    fmt.Println(describe(i32(42)))?;
    fmt.Println(describe(Point { x: 1 }))?;
    fmt.Println(describe(&Point { x: 2 }))?;
    fmt.Println(describe("s"))?;
    fmt.Println(describe(true))?;
    var e: error = nil;
    boom()
        on err { e = err; };
    fmt.Println(kind(e))?;
    fmt.Println(kind(&os.PathError { Op: "open", Path: "x", Err: e }))?;
};
