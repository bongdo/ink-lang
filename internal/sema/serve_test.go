package sema

import "testing"

const serveUser = `struct User {
    Name: str,
    age: u32,

    fn Greet() str {
        return "hi, " + Name;
    };

    fn Rename(name: str) void {
        Name = name;
    };
};
`

func TestServeNeedsStruct(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var n = 0;
    serve n;
};
`})
	wantError(t, p, "serve needs a struct value")
}

func TestServeFieldsAreInternal(t *testing.T) {
	p := checkSrc(t, map[string]string{".": serveUser + `
fn main() void {
    var u = User { Name: "Alice", age: 30 };
    serve u;
    print(u.Name);
};
`})
	wantError(t, p, "fields of a served object are internal")
}

func TestServeFieldsInsideMethod(t *testing.T) {
	p := checkSrc(t, map[string]string{".": serveUser + `
fn main() void {
    var u = User { Name: "Alice", age: 30 };
    serve u;
    print(u.Greet());
};
`})
	noErrors(t, p)
}

func TestServeFieldsInFreeFunction(t *testing.T) {
	p := checkSrc(t, map[string]string{".": serveUser + `
fn show(u: *User) void {
    print(u.Name);
};

fn main() void {
    var u = User { Name: "Alice", age: 30 };
    serve u;
    show(&u);
};
`})
	noErrors(t, p)
}

func TestServeInParRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": serveUser + `
fn main() void {
    var u = User { Name: "Alice", age: 30 };
    par {
        serve u;
    };
};
`})
	wantError(t, p, "serve must run before sharing")
}

func TestServeGenericRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Box[T] {
    Value: T,
};

fn main() void {
    var b = Box[i32] { Value: 1 };
    serve b;
};
`})
	wantError(t, p, "serve does not support generic types")
}

func TestServeTwiceAllowed(t *testing.T) {
	p := checkSrc(t, map[string]string{".": serveUser + `
fn main() void {
    var u = User { Name: "Alice", age: 30 };
    serve u;
    serve u;
};
`})
	noErrors(t, p)
}

func TestServeMethodWithServedParamRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": serveUser + `
struct Other {
    fn Touch(u: *User) void {
        u.Rename("x");
    };
};

fn main() void {
    var u = User { Name: "Alice", age: 30 };
    serve u;
    var o = Other {};
    serve o;
};
`})
	wantError(t, p, "a served method cannot take another served object")
}
