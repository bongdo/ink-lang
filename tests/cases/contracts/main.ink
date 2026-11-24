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
    var b = json.Marshal(u)
        or fail;
    fmt.Println(str(b))
        or fail;
    var a = json.Marshal(u.admin())
        or fail;
    fmt.Println(str(a))
        or fail;
};
