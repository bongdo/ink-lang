use fmt;

type User struct {
    Name: str,
    age: u32,

    fn Greet() str {
        return "hi, " + Name;
    };

    fn Birthday() void {
        age += 1;
    };
};

fn main() void! {
    var u = User { Name: "Alice", age: 30 };
    u.Birthday();
    fmt.Println(u.Greet())
        or fail;
    fmt.Println(u.age)
        or fail;
    var p = &u;
    p.Name = "Bob";
    fmt.Println(u.Greet())
        or fail;
};
