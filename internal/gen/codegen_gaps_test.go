package gen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bongdo/ink/internal/ast"
	"github.com/bongdo/ink/internal/parser"
	"github.com/bongdo/ink/internal/sema"
)

func functionBody(t *testing.T, src, name string) string {
	t.Helper()
	head := "func " + name + "("
	i := strings.Index(src, head)
	if i < 0 {
		t.Fatalf("function %s not found in generated source:\n%s", name, src)
	}
	j := strings.Index(src[i:], "\n}\n")
	if j < 0 {
		t.Fatalf("function %s has no closing brace:\n%s", name, src[i:])
	}
	return src[i : i+j]
}

func TestUnreachablePanicFollowsControlFlow(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;

type E enum {
    A,
    B,
};

fn viaReturn() i32 {
    return 1;
};

fn viaPanic() i32 {
    panic("boom");
};

fn viaIfElse(n: i32) i32 {
    if (n > 0) {
        return 2;
    } else {
        return 3;
    };
};

fn viaFor() i32 {
    for () {
        return 4;
    };
};

fn viaTypeMatch(v: any) i32 {
    match (v) {
        case i32 => return 5;
        else => return 6;
    };
};

fn viaEnumMatch(e: E) i32 {
    match (e) {
        case A => return 7;
        case B => return 8;
    };
};

fn viaWithin() i32 {
    within 1s {
        return 9;
    };
};

fn main() void {
    var fall = fn(n: i32) i32 {
        if (n > 0) {
            return 1;
        };
    };
    fmt.Println(viaReturn()) or panic();
    fmt.Println(viaIfElse(1)) or panic();
    fmt.Println(viaFor()) or panic();
    fmt.Println(viaTypeMatch(i32(1))) or panic();
    fmt.Println(viaEnumMatch(E.A)) or panic();
    fmt.Println(viaWithin()) or panic();
    fmt.Println(fall(0)) or panic();
};
`,
	})
	src := p.source(t, "zz_ink.go")
	for _, name := range []string{"viaReturn", "viaIfElse", "viaFor", "viaTypeMatch", "viaEnumMatch", "viaWithin"} {
		if body := functionBody(t, src, name); strings.HasSuffix(body, "\n\t"+`panic("unreachable")`) {
			t.Errorf("%s always returns but keeps a trailing panic:\n%s", name, body)
		}
	}
	if body := functionBody(t, src, "viaEnumMatch"); !strings.Contains(body, "default:") {
		t.Errorf("enum match lost its default guard:\n%s", body)
	}
	if body := functionBody(t, src, "_s_main"); !strings.Contains(body, `panic("unreachable")`) {
		t.Errorf("function literal lost its panic:\n%s", body)
	}
	if out, err := goCommand(t, p.dir, "vet", "."); err != nil {
		t.Fatalf("go vet: %v\n%s", err, out)
	}
	stdout, stderr, err := p.run(t)
	if err == nil {
		t.Fatalf("expected the falling-through literal to panic, stdout:\n%s", stdout)
	}
	if !strings.Contains(stderr, "panic: unreachable") {
		t.Fatalf("expected panic: unreachable, stderr:\n%s", stderr)
	}
	expect(t, stdout, "1\n2\n4\n5\n7\n9\n")
}

func TestStdImportAliasReused(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;
use context;

fn main() void! {
    var ctx, cancel = context.WithCancel();
    defer cancel();
    if (ctx != nil[context.Context]) {
        fmt.Println("hello") or fail;
    };
    within 1s {
        fmt.Println("within") or fail;
    };
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, "context.WithCancel(")
	contains(t, src, "context.WithTimeout(")
	contains(t, src, "fmt.Fprintln(")
	if n := strings.Count(src, `"fmt"`); n != 1 {
		t.Fatalf("fmt imported %d times:\n%s", n, src)
	}
	if n := strings.Count(src, `"context"`); n != 1 {
		t.Fatalf("context imported %d times:\n%s", n, src)
	}
	if strings.Contains(src, "_s_fmt.") || strings.Contains(src, "_s_context.") {
		t.Fatalf("generated code still uses a private alias:\n%s", src)
	}
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "hello\nwithin\n")
}

func TestStdImportAliasFollowsUser(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt as f;

fn main() void! {
    f.Println("hi") or fail;
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, `f "fmt"`)
	contains(t, src, "f.Fprintln(_s_os.Stderr, _s_err)")
	if n := strings.Count(src, `"fmt"`); n != 1 {
		t.Fatalf("fmt imported %d times:\n%s", n, src)
	}
	if strings.Contains(src, "_s_fmt") {
		t.Fatalf("generated code still uses _s_fmt:\n%s", src)
	}
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "hi\n")
}

func TestGoTypeImportsRequiredPackage(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;
use crypto/hmac;
use crypto/sha256;

fn main() void {
    var key = bytes("k");
    var mac = hmac.New(fn() => sha256.New(), key);
    fmt.Println(len(mac.Sum(nil))) or panic();
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, `_s_go_hash "hash"`)
	contains(t, src, "func() _s_go_hash.Hash")
	if strings.Contains(src, "func() hash.Hash") {
		t.Fatalf("generated code names an unimported package:\n%s", src)
	}
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "32\n")
}

func TestGoTypeUnnameableIsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/u\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `use slices;

fn main() void {
    var xs = i32[] { 3, 1, 2 };
    slices.SortFunc(xs, fn(a, b) => a - b);
};
`
	path := filepath.Join(dir, "main.ink")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	f, perrs := parser.Parse(path, []byte(src))
	for _, e := range perrs {
		t.Fatalf("parse: %d:%d: %s", e.Pos.Line, e.Pos.Col, e.Msg)
	}
	parsed := map[string][]*ast.File{".": {f}}
	prog := sema.Check(parsed, sema.Options{Root: dir, Module: "example.com/u"})
	for _, e := range prog.Errors {
		t.Fatalf("sema: %s", e.String())
	}
	_, errs := Generate(prog, parsed, Options{Root: dir, Module: "example.com/u"})
	if len(errs) == 0 {
		t.Fatal("expected a cannot name Go type error")
	}
	if !strings.Contains(errs[0].Error(), "cannot name Go type E") {
		t.Fatalf("unexpected error: %v", errs)
	}
}
