package sema

import (
	"testing"

	"github.com/bongdo/ink/internal/ast"
)

func findCall(p *Program, name string) *ast.CallExpr {
	var found *ast.CallExpr
	for _, pkg := range p.Packages {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				if found != nil {
					return false
				}
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					if fun.Name == name {
						found = call
					}
				case *ast.SelectorExpr:
					if fun.Name == name {
						found = call
					}
				}
				return true
			})
		}
	}
	return found
}

func TestExplicitCallTypeArgs(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn Map[In, Out](xs: In[], f: fn(In) Out) Out[] {
    var out = Out[] {};
    for (var x in xs) {
        out = append(out, f(x));
    };
    return out;
};

fn main() void {
    var words = Map[i32, str](i32[] { 1, 2 }, fn(x: i32) str => str(x));
    _ = words;
};
`})
	noErrors(t, p)
	call := findCall(p, "Map")
	if call == nil || call.Info == nil {
		t.Fatal("Map call not resolved")
	}
	info := call.Info
	if len(info.TypeArgs) != 2 || len(info.TypeParams) != 2 {
		t.Fatalf("type args = %v, params = %v", info.TypeArgs, info.TypeParams)
	}
	if arg, ok := info.TypeArgs[0].(*ast.NamedType); !ok || arg.Name != "i32" {
		t.Fatalf("type arg 0 = %#v", info.TypeArgs[0])
	}
	slice, ok := info.ParamTypes[0].(*ast.SliceType)
	if !ok {
		t.Fatalf("param 0 = %#v", info.ParamTypes[0])
	}
	if elem, ok := slice.Elem.(*ast.NamedType); !ok || elem.Name != "i32" {
		t.Fatalf("param 0 elem = %#v", slice.Elem)
	}
	fn, ok := info.ParamTypes[1].(*ast.FuncType)
	if !ok || len(fn.Params) != 1 || len(fn.Results) != 1 {
		t.Fatalf("param 1 = %#v", info.ParamTypes[1])
	}
	if result, ok := fn.Results[0].(*ast.NamedType); !ok || result.Name != "str" {
		t.Fatalf("callback result = %#v", fn.Results[0])
	}
	out, ok := info.ResultTypes[0].(*ast.SliceType)
	if !ok {
		t.Fatalf("result = %#v", info.ResultTypes[0])
	}
	if elem, ok := out.Elem.(*ast.NamedType); !ok || elem.Name != "str" {
		t.Fatalf("result elem = %#v", out.Elem)
	}
	if callType, ok := p.ExprTypes[call].(*ast.SliceType); !ok {
		t.Fatalf("call type = %#v", p.ExprTypes[call])
	} else if elem, ok := callType.Elem.(*ast.NamedType); !ok || elem.Name != "str" {
		t.Fatalf("call elem = %#v", callType.Elem)
	}
}

func TestExplicitCallTypeArgsArity(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn Map[In, Out](xs: In[], f: fn(In) Out) Out[] {
    var out = Out[] {};
    return out;
};

fn main() void {
    var words = Map[i32](i32[] { 1 }, fn(x: i32) str => str(x));
    _ = words;
};
`})
	if !hasError(p, "wants 2 type arguments, got 1") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestExplicitCallTypeArgsDriveConformance(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Writer interface {
    Write() void;
};

type Buf struct {
    age: i32,
};

fn Id[T](x: T) T {
    return x;
};

fn main() void {
    var w: Writer = Id[Buf](Buf { age = 1 });
    _ = w;
};
`})
	if !hasError(p, "Buf does not implement Writer") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestExplicitGoCallTypeArgs(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use cmp;

fn main() void {
    _ = cmp.Compare[int](1, 2);
};
`})
	noErrors(t, p)
	call := findCall(p, "Compare")
	if call == nil || call.Info == nil || len(call.Info.TypeArgs) != 1 {
		t.Fatalf("call info = %#v", call)
	}
}

func TestExplicitGoCallTypeArgsArity(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use cmp;

fn main() void {
    _ = cmp.Compare[int, str](1, 2);
};
`})
	if !hasError(p, "wants 1 type argument") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestExplicitCallTypeArgsNotGeneric(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn Plain(x: i32) i32 {
    return x;
};

fn main() void {
    _ = Plain[i32](1);
};
`})
	if !hasError(p, "function is not generic") {
		t.Fatalf("errors = %v", p.Errors)
	}
}
