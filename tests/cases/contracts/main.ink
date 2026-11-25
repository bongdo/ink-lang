use encoding/json;
use fmt;

struct User {
    ID: i64,
    Email: str,
    Role: str,
    Pass: str,

    wire {
        ID: "id",
        Email: "email",
    };

    admin {
        ID: "id",
        Email: "email",
        Role: "role",
    };
};

fn main() void! {
    var u = User { ID: 1, Email: "a@b", Role: "admin", Pass: "secret" };
    var b = json.Marshal(u)?;
    fmt.Println(str(b))?;
    var a = json.Marshal(u.admin())?;
    fmt.Println(str(a))?;
};
