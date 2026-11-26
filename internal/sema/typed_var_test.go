package sema

import (
	"testing"

	"github.com/bongdo/ink-lang/internal/ast"
)

func TestTypedVarNil(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Point {
    X: i32,
};

var gp: *Point = nil;
var gs: i32[] = nil;
var gm: map[str, i32] = nil;
var gh: fn(i32) i32 = nil;
var ge: error = nil;

fn main() void {
    var p: *Point = nil;
    var s: i32[] = nil;
    var m: map[str, i32] = nil;
    var h: fn(i32) i32 = nil;
    var e: error = nil;
    _ = p;
    _ = s;
    _ = m;
    _ = h;
    _ = e;
};
`})
	noErrors(t, p)
	for _, name := range []string{"gp", "gs", "gm", "gh", "ge"} {
		if p.global[p.Main][name] == nil {
			t.Fatalf("package var %s has no type", name)
		}
	}
	if _, ok := p.global[p.Main]["gp"].(*ast.PointerType); !ok {
		t.Fatalf("gp type = %#v", p.global[p.Main]["gp"])
	}
	if n, ok := p.global[p.Main]["ge"].(*ast.NamedType); !ok || n.Name != "error" {
		t.Fatalf("ge type = %#v", p.global[p.Main]["ge"])
	}
}

func TestTypedVarMultipleNames(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn pair() i32, i32 {
    return 1, 2;
};

var ga, gb: i32 = pair();

fn main() void {
    var a, b: i32 = pair();
    var c, d: i32 = 3, 4;
    _ = a;
    _ = b;
    _ = c;
    _ = d;
};
`})
	noErrors(t, p)
	for _, name := range []string{"ga", "gb"} {
		if n, ok := p.global[p.Main][name].(*ast.NamedType); !ok || n.Name != "i32" {
			t.Fatalf("%s type = %#v", name, p.global[p.Main][name])
		}
	}
}

func TestTypedConstRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `const C: i32 = 1;

fn main() void {};
`})
	wantError(t, p, "const cannot have a type")
}

func TestTypedVoidRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn note() void {};

fn main() void {
    var x: void = note();
    _ = x;
};
`})
	wantError(t, p, "void is only allowed as a function result")
}
