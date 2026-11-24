package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bongdo/ink/internal/ast"
	"github.com/bongdo/ink/internal/parser"
)

func parseOne(t *testing.T, src string) *ast.File {
	t.Helper()
	f, errs := parser.Parse("main.ink", []byte(src))
	for _, e := range errs {
		t.Fatalf("%d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
	}
	return f
}

func checkSrc(t *testing.T, files map[string]string) *Program {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed := make(map[string][]*ast.File)
	for rel, src := range files {
		dir := root
		if rel != "." {
			dir = filepath.Join(root, filepath.FromSlash(rel))
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		path := filepath.Join(dir, "main.ink")
		f, errs := parser.Parse(path, []byte(src))
		if len(errs) > 0 {
			t.Fatalf("%s: parse: %v", rel, errs)
		}
		parsed[rel] = append(parsed[rel], f)
	}
	return Check(parsed, Options{Root: root, Module: "example.com/test"})
}

func noErrors(t *testing.T, p *Program) {
	t.Helper()
	var b strings.Builder
	for _, e := range p.Errors {
		if e.Warning {
			continue
		}
		b.WriteString(e.String())
		b.WriteByte('\n')
	}
	if b.Len() > 0 {
		t.Fatalf("unexpected sema errors:\n%s", b.String())
	}
}

func TestHello(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;

fn main() void! {
    fmt.Println("hello") or fail;
};
`})
	noErrors(t, p)
}

func TestUseForms(t *testing.T) {
	p := checkSrc(t, map[string]string{
		".": `use fmt as f;
use net/http as http;
use ./lib as lib;

fn main() void! {
    f.Println("hi") or fail;
    var h = http.Header {};
    _ = h;
    _ = lib.Square(2);
};
`,
		"lib": `fn Square(x: i32) i32 {
    return x * x;
};
`,
	})
	noErrors(t, p)
}

func TestStructMethod(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    Name: str,
    age: u32,

    fn Greet() str {
        return "hi, " + Name;
    };

    fn Rename(name: str) void {
        Name = name;
    };
};

fn main() void {
    var u = User { Name = "a", age = 3 };
    var s = u.Greet();
    u.Rename(s);
};
`})
	noErrors(t, p)
}

func TestStructLiteralNeedsNamedFields(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct User {
    Name: str,
};

fn main() void {
    var u = User { "a" };
    _ = u;
};
`})
	wantError(t, p, "struct literal entry needs field: value")
}

func TestEnumMatch(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `enum Shape {
    Circle { radius: f64 },
    Rect { w: f64, h: f64 },
    Empty,
};

fn area(s: Shape) f64 {
    return match (s) {
    case Circle { radius } => 3.14 * radius * radius;
    case Rect { w, h } => w * h;
    case Empty => 0.0;
    };
};

fn main() void {
    var s = Shape.Circle { radius = 2.0 };
    var a = area(s);
    match (s) {
    case Circle { radius } => a = radius;
    case Rect { w, h } => a = w * h;
    case Empty => a = 0.0;
    };
};
`})
	noErrors(t, p)
}

func TestOrAndDefer(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use os;
use fmt;

fn read(path: str) str! {
    var f = os.Open(path) or fail;
    defer f.Close();
    var buf = u8[16] {};
    var n = f.Read(buf[:]) or fail;
    _ = n;
    fmt.Println(path) or fail;
    return path;
};

fn main() void! {
    var s = read("x") on err {
        fmt.Println("failed", err) or fail("output");
        return err;
    };
    fmt.Println(s) or fail;
};
`})
	noErrors(t, p)
}

func TestGeneric(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Stack[Item] {
    data: Item[],

    fn Push(x: Item) void {
        data = append(data, x);
    };

    fn Len() int {
        return len(data);
    };
};

fn Map[In, Out](xs: In[], f: fn(In) Out) Out[] {
    var out = Out[] {};
    for (var x in xs) {
        out = append(out, f(x));
    };
    return out;
};

fn main() void {
    var s = Stack[i32] {};
    s.Push(1);
    var n = s.Len();
    var ys = Map[i32, i32](i32[] {1, 2}, fn(x: i32) i32 => x * 2);
};
`})
	noErrors(t, p)
}

func TestControl(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var total = 0;
    for (var i = 0; i < 10; i += 1) {
        if (i == 3) { continue; };
        if (i == 8) { break; };
        total += i;
    };
    var xs = i32[] {1, 2, 3};
    for (var i, x in xs) {
        total += i + x;
    };
    var m = map[str, i32] { "a" = 1 };
    for (var k, v in m) {
        total += v + len(k);
    };
};
`})
	noErrors(t, p)
}

func TestPar(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn load(x: i32) i32! {
    return x;
};

fn main() void! {
    var a = 0;
    var b = 0;
    par {
        a = load(1) or fail;
        b = load(2) or fail;
    };
    var xs = i32[] {1, 2, 3};
    var out = i32[3] {};
    par for (var i = 0; i < 3; i += 1) limit 2 {
        out[i] = xs[i];
    };
    a = load(3) or fail;
};
`})
	noErrors(t, p)
}

func TestLocalPackage(t *testing.T) {
	p := checkSrc(t, map[string]string{
		".": `use ./lib as lib;

fn main() void {
    var n = lib.Square(3);
};
`,
		"lib": `fn Square(x: i32) i32 {
    return x * x;
};
`,
	})
	noErrors(t, p)
}

func TestParWriteRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var total = 0;
    par for (var i = 0; i < 4; i += 1) {
        total += i;
    };
};
`})
	if len(p.Errors) == 0 {
		t.Fatal("expected a go write error")
	}
}

func TestCheckStopsWhenCancelled(t *testing.T) {
	parsed := map[string][]*ast.File{".": {parseOne(t, "fn main() void {\n    var x = 1;\n    _ = x;\n};\n")}}
	root := t.TempDir()
	p := Check(parsed, Options{Root: root, Module: "example.com/test", Stop: func() bool { return true }})
	if len(p.Errors) != 0 {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestSiblingMethodCallHint(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Counter {
    N: i32,

    fn Bump() void {
        Add(1);
    };

    fn Add(n: i32) void {
        N += n;
    };
};

fn main() void {
    var c = Counter { N: 0 };
    c.Bump();
};
`})
	if !hasError(p, "self.Add") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestErrReturnWarning(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use strconv;

fn pick(s: str) i64! {
    var n = strconv.ParseInt(s, 10, 64)
        on err {
            return -1;
        };
    return n;
};

fn main() void! {
    _ = pick("1");
    return;
};
`})
	found := false
	for _, e := range p.Errors {
		if e.Warning && strings.Contains(e.Msg, "turns the failure into success") {
			found = true
		}
	}
	if !found {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestRemovedTypeAliases(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"var a: usize = 0;", "unknown type usize"},
		{"var a: isize = 0;", "unknown type isize"},
	} {
		p := checkSrc(t, map[string]string{".": "fn main() void {\n    " + tc.src + "\n    _ = a;\n};\n"})
		if !hasError(p, tc.want) {
			t.Fatalf("%s: errors = %v", tc.src, p.Errors)
		}
	}
}

func TestLimitIsNotReserved(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var limit = 4;
    var xs = i32[] { 1, 2 };
    par for (var i = 0; i < len(xs); i += 1) limit 1 {
        xs[i] = i32(limit);
    };
};
`})
	noErrors(t, p)
}

func TestOrHandlerReraiseRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn mk() i32! { return 1; };

fn g() i32! {
    var v = mk()
        on err {
            fail(err);
        };
    return v;
};

fn main() void {
    _ = g() on err { _ = err; };
};
`})
	if !hasError(p, "just or fail") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestFreeMethodWarning(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct App {
    N: i32,
};

fn bump(a: *App) void {
    a.N += 1;
};

fn main() void {
    var a = App { N: 0 };
    bump(&a);
};
`})
	found := false
	for _, e := range p.Errors {
		if e.Warning && strings.Contains(e.Msg, "state and behaviour belong") {
			found = true
		}
	}
	if !found {
		t.Fatalf("errors = %v", p.Errors)
	}
}
