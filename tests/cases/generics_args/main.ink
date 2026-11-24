use cmp;
use fmt;
use slices;

struct User {
    age: i32,
};

alias Strs = str[];

fn Echo[T](x: T) T {
    return x;
};

fn main() void! {
    var u = Echo[User](User { age: 7 });
    fmt.Println(u.age)
        or fail;
    fmt.Println(cmp.Compare[int](1, 2))
        or fail;
    var xs = slices.Concat[Strs, str]();
    fmt.Println(len(xs))
        or fail;
};
