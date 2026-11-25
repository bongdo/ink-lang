package gen

import (
	"strings"
	"testing"
)

func TestContractDefaultWireTags(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use encoding/json;
use fmt;

struct User {
    ID: i64,
    Email: str,
    Pass: str,

    wire {
        ID: "id",
        Email: "email",
    };
};

fn main() void! {
    var u = User { ID: 1, Email: "a@b", Pass: "secret" };
    var b = json.Marshal(u)?;
    fmt.Println(str(b))?;
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, "`json:\"id\"`")
	contains(t, src, "`json:\"email\"`")
	contains(t, src, "`json:\"-\"`")
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "{\"id\":1,\"email\":\"a@b\"}\n")
}

func TestContractNamedView(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use encoding/json;
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
    var b = json.Marshal(u.admin())?;
    fmt.Println(str(b))?;
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, "func (self *User) admin() map[string]any")
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "{\"email\":\"a@b\",\"id\":1,\"role\":\"admin\"}\n")
}

func TestContractDefaultDecode(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use encoding/json;
use fmt;

struct User {
    ID: i64,
    Email: str,
    Pass: str,

    wire {
        ID: "id",
        Email: "email",
    };
};

fn main() void! {
    var u = User {};
    json.Unmarshal(bytes("{\"id\":7,\"email\":\"e\",\"Pass\":\"p\"}"), &u)?;
    fmt.Println(u.ID, u.Email, len(u.Pass))?;
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "7 e 0\n")
}

func TestContractNoBlocksNoTags(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use encoding/json;
use fmt;

struct Plain {
    A: i32,
    B: str,
};

fn main() void! {
    var v = Plain { A: 1, B: "b" };
    var b = json.Marshal(v)?;
    fmt.Println(str(b))?;
};
`,
	})
	src := p.source(t, "zz_ink.go")
	if strings.Count(src, "json:") != 0 {
		t.Fatalf("plain struct got json tags:\n%s", src)
	}
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "{\"A\":1,\"B\":\"b\"}\n")
}
