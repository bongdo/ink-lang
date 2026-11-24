use fmt;
use sort;
use strconv;
use strings;

struct User {
    name: str,
    age: u32,
};

fn main() void! {
    var users = User[] {
        User { name: "b", age: 30 },
        User { name: "a", age: 20 },
    };
    sort.Slice(users, fn(i: int, j: int) bool => users[i].age < users[j].age);
    for (var u in users) {
        fmt.Println(strings.ToUpper(u.name), u.age)
            or fail;
    };
    var n = strconv.Itoa(42);
    fmt.Println(n)
        or fail;
};
