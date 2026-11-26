package format

import (
	"strings"
	"testing"

	"github.com/bongdo/ink/internal/parser"
)

const sample = `use fmt;
use ./lib as lib;

// user type
struct User {
    Name: str,
    age: u32,

    fn Greet() str {
        return "hi, " + Name;
    };
};

enum Shape {
    Circle { radius: f64 },
    Empty,
};

interface Writer {
    Write(p: u8[]) int!;
};

alias Id = u64;

const Max = 10;

fn area(s: Shape) f64 {
    return match (s) {
    case Circle { radius } => 3.14 * radius * radius;
    case Empty => 0.0;
    };
};

fn read(path: str) u8[]! {
    var f = lib.Open(path)?;
    defer f.Close();
    var buf = u8[16] {};
    var n = f.Read(buf[:])?;
    return buf[:n];
};

fn main() void! {
    var u = User { Name = "a", age = 1 };
    var s = u.Greet();
    if (len(s) > 0) {
        fmt.Println(s)?;
    } else {
        fmt.Println("empty")?;
    };
    for (var i = 0; i < 10; i += 1) {
        if (i % 2 == 0) { continue; };
    };
    par {
        fmt.Println("a")?;
        fmt.Println("b")?;
    };
    {
        fmt.Println("c")?;
    };
    var xs = u8[] {1, 2};
    var m = map[str, i32] { "a" = 1 };
    for (var k, v in m) {
        fmt.Println(k, v)?;
    };
    var f = fn(x: i32) i32 => x + 1;
    var g = fn(x: i32) i32 { return x + 1; };
    var n = len(xs);
    var p = nil[*User];
    _ = p;
    _ = f;
    _ = g;
    _ = n;
    return;
};

test "works" {
    assert(max(1, 2) == 2);
};
`

func TestRoundTrip(t *testing.T) {
	f, errs := parser.Parse("sample.ink", []byte(sample))
	if len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	out := File(f)
	f2, errs := parser.Parse("sample.ink", out)
	if len(errs) > 0 {
		t.Fatalf("parse formatted: %v\n---\n%s", errs, out)
	}
	out2 := File(f2)
	if string(out) != string(out2) {
		t.Fatalf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, out2)
	}
}

func TestLineCommentsPreserved(t *testing.T) {
	src := `// head
use fmt;

fn main() void {
    // body
    var x = 1; // tail
    _ = x;
};
`
	f, errs := parser.Parse("comments.ink", []byte(src))
	if len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	out := string(File(f))
	for _, want := range []string{"// head", "// body", "// tail"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	f2, errs := parser.Parse("comments.ink", []byte(out))
	if len(errs) > 0 {
		t.Fatalf("parse formatted: %v\n---\n%s", errs, out)
	}
	if out2 := string(File(f2)); out2 != out {
		t.Fatalf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, out2)
	}
}

func TestTrailingCommentsStay(t *testing.T) {
	src := `use fmt;

struct User {
    Name: str, // name
};

fn main() void {
    var x = 1; // tail
    if (x > 0) {
        fmt.Println(x)?; // print
    }; // block
};
`
	want := `use fmt;

struct User {
    Name: str, // name
};

fn main() void {
    var x = 1; // tail
    if (x > 0) {
        fmt.Println(x)?; // print
    }; // block
};
`
	if got := formatSrc(t, src); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestTrailingCommentsUseAndMatch(t *testing.T) {
	src := `use fmt; // note

struct User {
    Name: str,
}; // point

fn main() void {
    var x = match (1) {
    case 1 => fmt.Println("one");
    else => fmt.Println("other"); // tail
    };
    _ = x;
}; // done
`
	want := `use fmt; // note

struct User {
    Name: str,
}; // point

fn main() void {
    var x = match (1) {
        case 1 => fmt.Println("one");
        else => fmt.Println("other"); // tail
    };
    _ = x;
}; // done
`
	out := formatSrc(t, src)
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	f, errs := parser.Parse("comments.ink", []byte(out))
	if len(errs) > 0 {
		t.Fatalf("parse formatted: %v\n%s", errs, out)
	}
	if out2 := string(File(f)); out2 != out {
		t.Fatalf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, out2)
	}
}

func TestOnErrSingleStatementInline(t *testing.T) {
	src := `fn f() void! {
    check(false, "x");
};

fn main() void! {
    spawn f()
        on err { print("x:", err); };
    var v = f()
        on err { return; };
    _ = v;
};
`
	want := `fn f() void! {
    check(false, "x");
};

fn main() void! {
    spawn f()
        on err { print("x:", err); };
    var v = f()
        on err { return; };
    _ = v;
};
`
	if out := formatSrc(t, src); out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestServeStatement(t *testing.T) {
	src := `fn main() void {
    var c = Counter { n: 0 };
    serve c;
    print(c.Value());
};
`
	if out := formatSrc(t, src); out != src {
		t.Fatalf("got:\n%s\nwant:\n%s", out, src)
	}
}

func TestBlockCommentDiagnostic(t *testing.T) {
	_, errs := parser.Parse("block.ink", []byte("fn main() void {\n    /* nope */\n};\n"))
	if len(errs) != 1 || errs[0].Msg != "block comments are not supported; use //" {
		t.Fatalf("errors = %v", errs)
	}
}

func formatSrc(t *testing.T, src string) string {
	t.Helper()
	f, errs := parser.Parse("test.ink", []byte(src))
	if len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	return string(File(f))
}

func TestExplicitVoid(t *testing.T) {
	src := `fn f() {
    return;
};

fn main() void {
    var g = fn() {};
    var h = fn(x: i32) i32 => x;
    _ = g;
    _ = h;
};
`
	out := formatSrc(t, src)
	if !strings.Contains(out, "fn f() void {") {
		t.Fatalf("declaration missing void:\n%s", out)
	}
	if strings.Contains(out, "void = {") {
		t.Fatalf("declaration kept '=':\n%s", out)
	}
	for _, want := range []string{"fn() {}", "fn(x: i32) i32 => x"} {
		if !strings.Contains(out, want) {
			t.Fatalf("function literal changed, missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "fn() void") {
		t.Fatalf("void added to function literal:\n%s", out)
	}
}

func TestInterfaceMethodWithoutFn(t *testing.T) {
	src := `interface Writer {
    Write(p: u8[]) int!;
    Close() void;
    Flush();
};
`
	out := formatSrc(t, src)
	for _, want := range []string{"Write(p: u8[]) int!;", "Close() void;", "Flush() void;"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "fn Write") || strings.Contains(out, "fn Close") {
		t.Fatalf("interface method printed with fn:\n%s", out)
	}
}

func TestMultilineCompositeLit(t *testing.T) {
	src := `fn main() void {
var users = User[] {
User { Name = "Alice", age = 30 },
User { Name = "Bob", age = 25 },
};
var one = User { Name = "C", age = 1 };
var nested = Outer {
inner = Inner {
a = 1,
},
b = 2,
};
};
`
	want := `fn main() void {
    var users = User[] {
        User { Name: "Alice", age: 30 },
        User { Name: "Bob", age: 25 },
    };
    var one = User { Name: "C", age: 1 };
    var nested = Outer {
        inner: Inner {
            a: 1,
        },
        b: 2,
    };
};
`
	out := formatSrc(t, src)
	if out != want {
		t.Fatalf("multiline composite not preserved:\n--- got ---\n%s\n--- want ---\n%s", out, want)
	}
	if again := formatSrc(t, out); again != out {
		t.Fatalf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, again)
	}
}

func TestNewSyntax(t *testing.T) {
	src := `fn pair() i32, i32! {
    return 1, 2;
};

alias Handler = fn(i32) (str, bool);

fn main() void {
    var x = nil;
    match (msg) {
    case tea.KeyPressMsg as k => {};
    else => {};
    };
};

`
	f, errs := parser.Parse("new.ink", []byte(src))
	if len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	out := string(File(f))
	for _, want := range []string{"fn pair() (i32, i32)!", "fn(i32) (str, bool)", "var x = nil;", "case tea.KeyPressMsg as k", "else =>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestCanonicalDeclSyntax(t *testing.T) {
	src := `struct User {
    Name: str,
};

enum Shape {
    Circle,
};

interface Writer {
    Write(p: u8[]) int!;
};

alias UserId = u64;

fn Greet() str {
    return "hi";
};

fn main() void! {
    var g = fn(x: i32) i32 => x;
    _ = g;
    return;
};
`
	out := formatSrc(t, src)
	if out != src {
		t.Fatalf("canonical input changed:\n--- got ---\n%s\n--- want ---\n%s", out, src)
	}
	if again := formatSrc(t, out); again != out {
		t.Fatalf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, again)
	}
}

func TestElseIfSingleSemicolon(t *testing.T) {
	src := `fn f(n: i32) void {
    if (n == 0) {
        return;
    } else if (n == 1) {
        return;
    } else {
        return;
    };
};
`
	out := formatSrc(t, src)
	if strings.Contains(out, "};;") {
		t.Fatalf("duplicate semicolon after else-if:\n%s", out)
	}
	if again := formatSrc(t, out); again != out {
		t.Fatalf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, again)
	}
}

func TestTypedVarDecl(t *testing.T) {
	src := `struct Point {
    X: i32,
};

var p: *Point = nil;
var xs: i32[] = nil;
var m: map[str, i32] = nil;
var h: fn(i32) i32 = nil;
var e: error = nil;

fn main() void {
    var n: i32 = 1;
    for (var i: i32 = 0; i < n; i += 1) {};
    _ = p;
    _ = xs;
    _ = m;
    _ = h;
    _ = e;
};
`
	out := formatSrc(t, src)
	for _, want := range []string{
		"var p: *Point = nil;",
		"var xs: i32[] = nil;",
		"var m: map[str, i32] = nil;",
		"var h: fn(i32) i32 = nil;",
		"var e: error = nil;",
		"var n: i32 = 1;",
		"for (var i: i32 = 0; i < n; i += 1)",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, ": void") {
		t.Fatalf("printed void type:\n%s", out)
	}
	if again := formatSrc(t, out); again != out {
		t.Fatalf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, again)
	}
}

func TestEmptyForLoop(t *testing.T) {
	src := `fn main() void {
    for (; ; ) {};
    par for (;;) limit 2 {};
};
`
	out := formatSrc(t, src)
	for _, want := range []string{"for () {}", "par for () limit 2 {}"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "for (;") {
		t.Fatalf("empty loop kept separators:\n%s", out)
	}
	if again := formatSrc(t, out); again != out {
		t.Fatalf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, again)
	}
}

func TestConstAndAliasGroups(t *testing.T) {
	src := `use fmt;

const A = 1;

const B = 2;
const C = 3;

var v = 1;

alias X = u8;

alias Y = u16;
alias Z = u32;

struct S {
    a: i32,
};
`
	want := `use fmt;

const A = 1;
const B = 2;
const C = 3;

var v = 1;

alias X = u8;
alias Y = u16;
alias Z = u32;

struct S {
    a: i32,
};
`
	out := formatSrc(t, src)
	if out != want {
		t.Fatalf("groups not collapsed:\n--- got ---\n%s\n--- want ---\n%s", out, want)
	}
	if again := formatSrc(t, out); again != out {
		t.Fatalf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, again)
	}
}

func TestVersionedImportKeepsAlias(t *testing.T) {
	for _, want := range []string{
		"use charm.land/bubbletea/v2 as tea;\n",
		"use charm.land/bubbletea/v2 as bubbletea;\n",
		"use charm.land/bubbletea/v2 as v2;\n",
	} {
		if out := formatSrc(t, strings.TrimSuffix(want, "\n")); out != want {
			t.Fatalf("got %q, want %q", out, want)
		}
	}
}

func TestBlankImportKept(t *testing.T) {
	for _, want := range []string{
		"use net/http/pprof as _;\n",
		"use fmt as _;\n",
	} {
		if out := formatSrc(t, strings.TrimSuffix(want, "\n")); out != want {
			t.Fatalf("got %q, want %q", out, want)
		}
	}
}
