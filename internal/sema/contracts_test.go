package sema

import "testing"

const contractOK = `struct User {
    ID: i64,
    Email: str,
    Pass: str,

    wire {
        ID: "id",
        Email: "email",
    };

    admin {
        ID: "id",
        Email: "email",
    };
};

fn main() void {
    var u = User {};
    _ = u.admin();
};
`

func TestContractsOK(t *testing.T) {
	noErrors(t, checkSrc(t, map[string]string{".": contractOK}))
}

func TestContractMissingField(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    ID: i64,

    wire {
        Missing: "m",
    };
};

fn main() void {};
`})
	wantError(t, p, `contract: field "Missing" is not in User`)
}

func TestContractUnexportedField(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    id: i64,

    wire {
        id: "id",
    };
};

fn main() void {};
`})
	wantError(t, p, "is not exported")
}

func TestContractDuplicateField(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    ID: i64,

    wire {
        ID: "id",
        ID: "other",
    };
};

fn main() void {};
`})
	wantError(t, p, "is listed twice")
}

func TestContractDuplicateName(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    ID: i64,
    Other: i64,

    wire {
        ID: "same",
        Other: "same",
    };
};

fn main() void {};
`})
	wantError(t, p, `name "same" is used twice`)
}

func TestContractDuplicateBlock(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    ID: i64,

    wire {
        ID: "id",
    };

    wire {
        ID: "id2",
    };
};

fn main() void {};
`})
	wantError(t, p, "already declared")
}

func TestContractEmpty(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    ID: i64,

    wire {
    };
};

fn main() void {};
`})
	wantError(t, p, `block "wire" is empty`)
}

func TestContractGeneric(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Box[T] {
    Value: T,

    wire {
        Value: "value",
    };
};

fn main() void {};
`})
	wantError(t, p, "not supported on generic types")
}

func TestContractUnusedView(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    ID: i64,

    admin {
        ID: "id",
    };
};

fn main() void {};
`})
	wantError(t, p, `named view "admin" is unused`)
}

func TestContractBadKey(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    ID: i64,

    wire {
        ID: "a,b",
    };
};

fn main() void {};
`})
	wantError(t, p, "cannot be used as a JSON name")
}

func TestContractUppercaseView(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    ID: i64,

    Admin {
        ID: "id",
    };
};

fn main() void {};
`})
	wantError(t, p, "must start with a lowercase letter")
}

func TestContractViewCollidesWithField(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    admin: i64,

    admin {
        admin: "id",
    };
};

fn main() void {
    var u = User {};
    _ = u.admin();
};
`})
	wantError(t, p, `view "admin" collides with a field`)
}

func TestContractViewCollidesWithMethod(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    ID: i64,

    fn admin() void {};

    admin {
        ID: "id",
    };
};

fn main() void {
    var u = User {};
    _ = u.admin();
};
`})
	wantError(t, p, `view "admin" collides with a method`)
}

func TestContractUnusedIsPerType(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Other {
    admin: i64,
};

struct User {
    ID: i64,

    admin {
        ID: "id",
    };
};

fn main() void {
    var o = Other { admin: 3 };
    _ = o.admin;
};
`})
	wantError(t, p, `named view "admin" is unused`)
}

func TestContractInvalidViewOneError(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    ID: i64,

    Admin {
        ID: "id",
    };
};

fn main() void {};
`})
	if len(p.Errors) != 1 {
		t.Fatalf("want 1 error, got %d: %v", len(p.Errors), p.Errors)
	}
	wantError(t, p, "must start with a lowercase letter")
}
