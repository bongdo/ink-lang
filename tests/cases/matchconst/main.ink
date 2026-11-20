use fmt;
use os;

const Small = 10;

fn classify(n: i32) str {
    return match (n) {
        case 0 => "zero";
        case Small => "small";
        else => "big";
    };
};

fn main() void! {
    fmt.Println(classify(0))
        or fail;
    fmt.Println(classify(10))
        or fail;
    fmt.Println(classify(99))
        or fail;
    fmt.Println(match (os.O_RDONLY) {
        case os.O_WRONLY => "write";
        else => "read";
    })
        or fail;
};
