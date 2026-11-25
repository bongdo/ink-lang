use fmt;

struct User {
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
    fmt.Println(u.Greet())?;
    fmt.Println(u.age)?;
    var p = &u;
    p.Name = "Bob";
    fmt.Println(u.Greet())?;
};
