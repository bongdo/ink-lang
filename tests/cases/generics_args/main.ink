use fmt;

type User struct {
    age: i32,
};

fn Echo[T](x: T) T {
    return x;
};

fn main() void or error {
    var u = Echo[User](User { age = 7 });
    fmt.Println(u.age) or return;
};
