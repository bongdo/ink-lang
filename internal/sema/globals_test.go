package sema

import (
	"testing"

	"github.com/bongdo/ink/internal/ast"
)

func TestForwardGlobalInference(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `var a = b;
var b = c;
var c = i32(7);

fn main() void {
    _ = a;
    _ = b;
    _ = c;
};
`})
	noErrors(t, p)
	pkg := p.Packages["."]
	for _, name := range []string{"a", "b", "c"} {
		typ := p.global[pkg][name]
		if typ == nil {
			t.Fatalf("%s has no type", name)
		}
		n, ok := typ.(*ast.NamedType)
		if !ok || n.Name != "i32" {
			t.Fatalf("%s type = %#v", name, typ)
		}
	}
}

func TestGlobalInitializationCycle(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `var a = b;
var b = a;

fn main() void {
    _ = a;
};
`})
	if !hasError(p, "initialization cycle") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestTypedGlobalInitializationCycle(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `var a: i32 = b;
var b = a + 1;

fn main() void {
    _ = a;
    _ = b;
};
`})
	if !hasError(p, "initialization cycle: a -> b") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestGlobalCycleIgnoresFunctionBodies(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `var f = fn() i32 {
    return a;
};

var a = 1;

fn main() void {
    _ = f;
    _ = a;
};
`})
	noErrors(t, p)
}

func TestGlobalIIFECycle(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `var a: i32 = fn() i32 {
    return b;
}();

var b = a + 1;

fn main() void {
    _ = a;
    _ = b;
};
`})
	if !hasError(p, "initialization cycle: a -> b") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestGlobalSelfCycle(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `var a = a;

fn main() void {
    _ = a;
};
`})
	if !hasError(p, "initialization cycle: a") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestStringIndexIsByte(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var word = "é";
    var b: byte = word[0];
    _ = b;
};
`})
	noErrors(t, p)
	var index *ast.IndexExpr
	for _, f := range p.Packages["."].Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if e, ok := n.(*ast.IndexExpr); ok {
				index = e
			}
			return true
		})
	}
	if index == nil {
		t.Fatal("index expression not found")
	}
	typ, ok := p.ExprTypes[index].(*ast.NamedType)
	if !ok || typ.Name != "byte" {
		t.Fatalf("index type = %#v", p.ExprTypes[index])
	}
}

func TestGlobalCycleThroughFunctions(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `var a = f();
var b = g();

fn f() i32 {
    return b;
};

fn g() i32 {
    return a;
};

fn main() void {
    _ = a;
    _ = b;
};
`})
	if !hasError(p, "initialization cycle") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestGlobalCycleThroughNestedCall(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `var a = f();

fn f() i32 {
    return g();
};

fn g() i32 {
    return a;
};

fn main() void {
    _ = a;
};
`})
	if !hasError(p, "initialization cycle") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestGlobalFunctionReadIsNotACycle(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `var a = f();
var b = 1;

fn f() i32 {
    return b;
};

fn main() void {
    _ = a;
    _ = b;
};
`})
	noErrors(t, p)
}
