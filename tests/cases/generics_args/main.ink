use cmp;
use fmt;
use slices;

type User struct {
    age: i32,
};

type Strs = str[];

fn Echo[T](x: T) T {
    return x;
};

fn main() void or error {
    var u = Echo[User](User { age = 7 });
    fmt.Println(u.age) or return;
    fmt.Println(cmp.Compare[int](1, 2)) or return;
    var xs = slices.Concat[Strs, str]();
    fmt.Println(len(xs)) or return;
};
