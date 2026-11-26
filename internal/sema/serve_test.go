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

func TestServeTwiceRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": serveUser + `
fn main() void {
    var u = User { Name: "Alice", age: 30 };
    serve u;
    serve u;
};
`})
	wantError(t, p, "u is already served")
}

func TestServeReturnReferenceRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Store {
    items: i32[],

    fn Items() i32[] {
        return items;
    };

    fn Me() *Store {
        return self;
    };
};

fn main() void {
    var s = Store { items: i32[] { 1 } };
    serve s;
};
`})
	wantError(t, p, "returning a reference to served state escapes the lock")
}

func TestServeAliasReferenceRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Store {
    items: i32[],

    fn Alias() i32[] {
        var x = items;
        return x;
    };

    fn Element() *i32 {
        return &items[0];
    };

    fn Slice() i32[] {
        return items[0:len(items)];
    };

    fn Leak(out: i32[]) void {
        out = items;
    };
};

fn main() void {
    var s = Store { items: i32[] { 1 } };
    serve s;
};
`})
	wantError(t, p, "do not alias served state")
	wantError(t, p, "returning a reference to served state")
}

func TestServeAppendBackAllowed(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Store {
    items: i32[],

    fn Add(v: i32) void {
        items = append(items, v);
    };

    fn Count() i32 {
        return len(items);
    };
};

fn main() void {
    var s = Store { items: i32[] {} };
    serve s;
    s.Add(1);
    print(s.Count());
};
`})
	noErrors(t, p)
}

func TestServePointerThroughHolderRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Counter {
    n: i32,

    fn Inc() void {
        n += 1;
    };
};

struct Holder {
    ptr: *Counter,
};

fn main() void {
    var c = Counter { n: 0 };
    serve c;
    var h = Holder { ptr: &c };
    print(h.ptr.n);
};
`})
	wantError(t, p, "fields of a served object are internal")
}

func TestServeWrapperReturnRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Wrap {
    items: i32[],
};

struct Store {
    items: i32[],

    fn Wrap() Wrap {
        return Wrap { items: items };
    };

    fn Grow() i32[] {
        return append(items, 1);
    };
};

fn main() void {
    var s = Store { items: i32[] { 1 } };
    serve s;
};
`})
	wantError(t, p, "returning a reference to served state")
}

func TestServeNestedReferenceRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Item {
    tags: str[],
};

struct Store {
    items: Item[],

    fn Tags() str[] {
        return items[0].tags;
    };

    fn CopyOut() Item[] {
        var out = Item[] {};
        out = append(out, items...);
        return out;
    };

    fn LoopOut() Item[] {
        var out = Item[] {};
        for (var it in items) {
            out = append(out, it);
        };;
        return out;
    };

    fn AnyOut() any {
        return any(items);
    };
};

fn main() void {
    var s = Store { items: Item[] { Item { tags: str[] { "a" } } } };
    serve s;
};
`})
	wantError(t, p, "returning a reference to served state")
	wantError(t, p, "do not alias served state")
}

func TestServeValueCopyRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Counter {
    n: i32,

    fn Value() i32 {
        return n;
    };
};

fn show(c: Counter) i32 {
    return c.Value();
};

fn main() void {
    var c = Counter { n: 1 };
    serve c;
    print(show(c));
};
`})
	wantError(t, p, "a served type is not copyable")
}

func TestServeAfterParRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": serveUser + `
fn main() void {
    var u = User { Name: "Alice", age: 30 };
    par {
        print("x");
    };
    serve u;
};
`})
	wantError(t, p, "serve must run before par or spawn")
}

func TestServeFieldRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Inner {
    n: i32,
};

struct Outer {
    inner: Inner,
};

fn main() void {
    var o = Outer { inner: Inner { n: 0 } };
    serve o.inner;
};
`})
	wantError(t, p, "serve needs a whole object")
}

func TestServeGlobalOutsideMainRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": serveUser + `
var u = User { Name: "Alice", age: 30 };

fn setup() void {
    serve u;
};

fn main() void {
    setup();
    print(u.Greet());
};
`})
	wantError(t, p, "serve a package variable in main or a test")
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
