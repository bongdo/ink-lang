package sema

import (
	"testing"

	"github.com/bongdo/ink/internal/ast"
)

func varType(t *testing.T, p *Program, name string) ast.Type {
	t.Helper()
	for _, d := range p.Main.Files[0].Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Name != "main" {
			continue
		}
		for _, s := range fn.Body.Stmts {
			ds, ok := s.(*ast.DeclStmt)
			if !ok || len(ds.Decl.Names) != 1 || ds.Decl.Names[0] != name {
				continue
			}
			lit, ok := ds.Decl.Values[0].(*ast.CompositeLit)
			if !ok {
				t.Fatalf("%s value = %#v", name, ds.Decl.Values[0])
			}
			return lit.Type
		}
	}
	t.Fatalf("var %s not found", name)
	return nil
}

func arrayVarType(t *testing.T, p *Program, name string) *ast.ArrayType {
	t.Helper()
	at, ok := varType(t, p, name).(*ast.ArrayType)
	if !ok {
		t.Fatalf("%s type = %#v", name, varType(t, p, name))
	}
	return at
}

func TestArrayNamedConstStruct(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Foo struct {
    x: i32,
};

const N = 3;

fn main() void {
    var a = Foo[N] {};
    _ = a;
};
`})
	noErrors(t, p)
	at := arrayVarType(t, p, "a")
	length, ok := at.Len.(*ast.Ident)
	if !ok || length.Name != "N" || length.Ref == nil || length.Ref.Kind != ast.RefConst {
		t.Fatalf("len = %#v", at.Len)
	}
	elem, ok := at.Elem.(*ast.NamedType)
	if !ok || elem.Name != "Foo" || elem.Decl == nil {
		t.Fatalf("elem = %#v", at.Elem)
	}
}

func TestArrayNamedConstAlias(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Byte = u8;

const N = 3;

fn main() void {
    var a = Byte[N] {};
    _ = a;
};
`})
	noErrors(t, p)
	at := arrayVarType(t, p, "a")
	elem, ok := at.Elem.(*ast.NamedType)
	if !ok || elem.Name != "Byte" || elem.Decl == nil || elem.Decl.Kind != ast.TypeAlias {
		t.Fatalf("elem = %#v", at.Elem)
	}
	target, ok := elem.Decl.Target.(*ast.NamedType)
	if !ok || target.GoName != "uint8" {
		t.Fatalf("alias target = %#v", elem.Decl.Target)
	}
}

func TestArrayLiteralLength(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Foo struct {
    x: i32,
};

fn main() void {
    var a = Foo[3] {};
    _ = a;
};
`})
	noErrors(t, p)
	at := arrayVarType(t, p, "a")
	length, ok := at.Len.(*ast.BasicLit)
	if !ok || length.Value != "3" {
		t.Fatalf("len = %#v", at.Len)
	}
}

func TestArrayGenericTypeDiagnostic(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Stack[Item] struct {
    data: Item[],
};

fn main() void {
    var a = Stack[3] {};
    _ = a;
};
`})
	wantError(t, p, "generic type Stack requires type arguments")
}

func TestArrayLengthNotConstant(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Foo struct {
    x: i32,
};

type Bar struct {
    y: i32,
};

fn main() void {
    var a = Foo[Bar] {};
    _ = a;
};
`})
	wantError(t, p, "Bar is not a constant")
}

func TestGenericInstantiationStillWorks(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Stack[Item] struct {
    data: Item[],
};

fn main() void {
    var a = Stack[i32] {};
    _ = a;
};
`})
	noErrors(t, p)
	gen, ok := varType(t, p, "a").(*ast.GenericType)
	if !ok {
		t.Fatalf("type = %#v", varType(t, p, "a"))
	}
	if base, ok := gen.Base.(*ast.NamedType); !ok || base.Name != "Stack" {
		t.Fatalf("base = %#v", gen.Base)
	}
}

func TestArrayImportedConstLength(t *testing.T) {
	p := checkSrc(t, map[string]string{
		".": `use ./lib as lib;

type Foo struct {
    x: i32,
};

fn main() void {
    var a = Foo[lib.N] {};
    _ = a;
};
`,
		"lib": `const N = 3;
`,
	})
	noErrors(t, p)
	at := arrayVarType(t, p, "a")
	sel, ok := at.Len.(*ast.SelectorExpr)
	if !ok || sel.Name != "N" {
		t.Fatalf("len = %#v", at.Len)
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "lib" || pkg.Ref == nil || pkg.Ref.Kind != ast.RefPackage {
		t.Fatalf("len qualifier = %#v", sel.X)
	}
	if sel.Ref == nil || sel.Ref.Kind != ast.RefConst || sel.Ref.Decl == nil {
		t.Fatalf("len ref = %#v", sel.Ref)
	}
}

func TestArrayImportedVarNotConstant(t *testing.T) {
	p := checkSrc(t, map[string]string{
		".": `use ./lib as lib;

type Foo struct {
    x: i32,
};

fn main() void {
    var a = Foo[lib.N] {};
    _ = a;
};
`,
		"lib": `var N = 3;
`,
	})
	wantError(t, p, "lib.N is not a constant")
}

func TestDuplicateEnumVariantPackage(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type A enum {
    X,
    Y,
};

type B enum {
    Y,
    Z,
};

fn main() void {};
`})
	wantError(t, p, "enum variant Y is already declared in A")
}

func TestDuplicateEnumVariantOtherPackage(t *testing.T) {
	p := checkSrc(t, map[string]string{
		".": `type A enum {
    X,
};

fn main() void {};
`,
		"lib": `type B enum {
    X,
};
`,
	})
	noErrors(t, p)
}
