package gen

import (
	"go/types"
	"strings"

	"github.com/bongdo/ink/internal/ast"
)

const backgroundCtx = "_s_context.Background()"

var builtinGo = map[string]string{
	"void":  "",
	"bool":  "bool",
	"i8":    "int8",
	"i16":   "int16",
	"i32":   "int32",
	"i64":   "int64",
	"u8":    "uint8",
	"u16":   "uint16",
	"u32":   "uint32",
	"u64":   "uint64",
	"int":   "int",
	"uint":  "uint",
	"usize": "uint",
	"isize": "int",
	"f32":   "float32",
	"f64":   "float64",
	"str":   "string",
	"rune":  "rune",
	"byte":  "byte",
	"any":   "any",
	"error": "error",
}

func (g *generator) typeStr(f *fileGen, t ast.Type) string {
	if t == nil {
		return "any"
	}
	switch t := t.(type) {
	case *ast.NamedType:
		return g.namedTypeStr(f, t)
	case *ast.SliceType:
		return "[]" + g.typeStr(f, t.Elem)
	case *ast.ArrayType:
		return "[" + g.arrayLenStr(f, t.Len) + "]" + g.typeStr(f, t.Elem)
	case *ast.MapType:
		return "map[" + g.typeStr(f, t.Key) + "]" + g.typeStr(f, t.Value)
	case *ast.PointerType:
		return "*" + g.typeStr(f, t.Elem)
	case *ast.GenericType:
		args := make([]string, 0, len(t.Args))
		for _, a := range t.Args {
			args = append(args, g.typeStr(f, a))
		}
		return g.typeStr(f, t.Base) + "[" + strings.Join(args, ", ") + "]"
	case *ast.FuncType:
		return g.funcTypeStr(f, t)
	case *ast.InterfaceType:
		return g.interfaceTypeStr(f, t.Methods)
	case *ast.StructType:
		return g.structTypeStr(f, t.Fields)
	case *ast.GoType:
		return g.goTypeStr(f, t)
	}
	return "any"
}

func (g *generator) goTypeStr(f *fileGen, t *ast.GoType) string {
	gt := g.p.GoTypeOf(t)
	if gt == nil {
		f.noteGoText(t.Text)
		return t.Text
	}
	bad := false
	text := types.TypeString(gt, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		alias, ok := f.goPkg(pkg)
		if !ok {
			bad = true
			return pkg.Name()
		}
		return alias
	})
	if bad || !nameableGoType(gt) {
		g.errAt(t, "cannot name Go type %s", text)
		return "any"
	}
	return text
}

func nameableGoType(t types.Type) bool {
	switch t := t.(type) {
	case *types.Basic:
		return t.Kind() != types.UnsafePointer
	case *types.TypeParam:
		return false
	case *types.Named:
		if !t.Obj().Exported() {
			return false
		}
		args := t.TypeArgs()
		for i := 0; i < args.Len(); i++ {
			if !nameableGoType(args.At(i)) {
				return false
			}
		}
	case *types.Pointer:
		return nameableGoType(t.Elem())
	case *types.Slice:
		return nameableGoType(t.Elem())
	case *types.Array:
		return nameableGoType(t.Elem())
	case *types.Map:
		return nameableGoType(t.Key()) && nameableGoType(t.Elem())
	case *types.Chan:
		return nameableGoType(t.Elem())
	case *types.Signature:
		if t.TypeParams().Len() > 0 || t.RecvTypeParams().Len() > 0 {
			return false
		}
		for i := 0; i < t.Params().Len(); i++ {
			if !nameableGoType(t.Params().At(i).Type()) {
				return false
			}
		}
		for i := 0; i < t.Results().Len(); i++ {
			if !nameableGoType(t.Results().At(i).Type()) {
				return false
			}
		}
	}
	return true
}

func (g *generator) namedTypeStr(f *fileGen, t *ast.NamedType) string {
	if t.Variant != nil {
		qual := t.PkgAlias
		if qual != "" {
			f.usePkg(qual)
		}
		return qualified(qual, g.variantName(t.Variant))
	}
	if t.PkgAlias != "" {
		f.usePkg(t.PkgAlias)
		name := t.GoName
		if name == "" {
			name = t.Name
		}
		return t.PkgAlias + "." + name
	}
	if t.Package != "" {
		if f.pkg.Imports[t.Package] != nil {
			f.usePkg(t.Package)
			name := t.GoName
			if name == "" {
				name = t.Name
			}
			return t.Package + "." + name
		}
		if d := f.pkg.Types[t.Package]; d != nil && d.Kind == ast.TypeEnum {
			if v := variantByName(d, t.Name); v != nil {
				return g.variantName(v)
			}
		}
		if i := strings.LastIndexByte(t.Package, '.'); i >= 0 {
			alias := t.Package[:i]
			enumName := t.Package[i+1:]
			if imp := f.pkg.Imports[alias]; imp != nil && imp.Local != nil {
				if d := imp.Local.Types[enumName]; d != nil && d.Kind == ast.TypeEnum {
					if v := variantByName(d, t.Name); v != nil {
						f.usePkg(alias)
						return alias + "." + g.variantName(v)
					}
				}
			}
		}
	}
	if t.Package == "" {
		switch t.Name {
		case "Duration":
			return f.std("time") + ".Duration"
		case "Type":
			return f.std("reflect") + ".Type"
		}
		if v, ok := builtinGo[t.Name]; ok {
			if v == "" {
				return ""
			}
			return v
		}
	}
	name := t.GoName
	if name == "" {
		name = t.Name
	}
	return name
}

func (g *generator) variantName(v *ast.Variant) string {
	if v == nil {
		return ""
	}
	if v.GoName != "" {
		return v.GoName
	}
	if v.Enum != nil {
		return g.enumName(v.Enum) + v.Name
	}
	return v.Name
}

func (g *generator) enumName(d *ast.TypeDecl) string {
	if d == nil {
		return ""
	}
	if d.GoName != "" {
		return d.GoName
	}
	return d.Name
}

func qualified(qual, name string) string {
	if qual == "" {
		return name
	}
	return qual + "." + name
}

func (g *generator) arrayLenStr(f *fileGen, e ast.Expr) string {
	w := newWriter(f, backgroundCtx)
	return w.expr(e)
}

func (g *generator) funcTypeStr(f *fileGen, t *ast.FuncType) string {
	params := g.typeListStr(f, t.Params)
	if t.NeedsCtx {
		if params == "" {
			params = f.std("context") + ".Context"
		} else {
			params = f.std("context") + ".Context, " + params
		}
	}
	return "func(" + params + ")" + g.resultStr(f, t.Results, t.Err)
}

func (g *generator) resultStr(f *fileGen, results []ast.Type, err bool) string {
	if len(results) == 0 {
		if err {
			return " error"
		}
		return ""
	}
	parts := make([]string, 0, len(results))
	for _, t := range results {
		parts = append(parts, g.typeStr(f, t))
	}
	if len(parts) == 1 && !err {
		return " " + parts[0]
	}
	if err {
		parts = append(parts, "error")
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func (g *generator) typeListStr(f *fileGen, ts []ast.Type) string {
	parts := make([]string, 0, len(ts))
	for _, t := range ts {
		parts = append(parts, g.typeStr(f, t))
	}
	return strings.Join(parts, ", ")
}

func (g *generator) paramsStr(f *fileGen, params []*ast.Param, types []ast.Type, needsCtx bool) string {
	var parts []string
	if needsCtx {
		parts = append(parts, "_s_ctx "+f.std("context")+".Context")
	}
	for i, p := range params {
		t := p.Type
		if t == nil && i < len(types) {
			t = types[i]
		}
		name := p.Name
		if name == "" {
			name = "_"
		}
		parts = append(parts, name+" "+g.typeStr(f, t))
	}
	return strings.Join(parts, ", ")
}

func (g *generator) typeParamsDecl(ps []*ast.TypeParam) string {
	if len(ps) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		parts = append(parts, p.Name+" any")
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (g *generator) typeParamsUse(ps []*ast.TypeParam) string {
	if len(ps) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ps))
	for _, p := range ps {
		parts = append(parts, p.Name)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func (g *generator) interfaceTypeStr(f *fileGen, methods []*ast.IFaceMethod) string {
	var parts []string
	for _, m := range methods {
		parts = append(parts, g.ifaceMethodStr(f, m))
	}
	if len(parts) == 0 {
		return "interface{}"
	}
	return "interface{ " + strings.Join(parts, "; ") + " }"
}

func (g *generator) ifaceMethodStr(f *fileGen, m *ast.IFaceMethod) string {
	params := g.paramsStr(f, m.Params, nil, m.NeedsCtx)
	return m.Name + "(" + params + ")" + g.resultStr(f, m.Results, m.Err)
}

func (g *generator) structTypeStr(f *fileGen, fields []*ast.Field) string {
	var parts []string
	for _, fld := range fields {
		parts = append(parts, fld.Name+" "+g.typeStr(f, fld.Type))
	}
	if len(parts) == 0 {
		return "struct{}"
	}
	return "struct{ " + strings.Join(parts, "; ") + " }"
}

func (f *fileGen) noteGoText(text string) {
	for alias := range f.pkg.Imports {
		if textUses(text, alias) {
			f.usePkg(alias)
		}
	}
}

func textUses(text, name string) bool {
	needle := name + "."
	for i := 0; ; {
		j := strings.Index(text[i:], needle)
		if j < 0 {
			return false
		}
		k := i + j
		if k == 0 || !identChar(text[k-1]) {
			return true
		}
		i = k + 1
	}
}

func identChar(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

func isInkBuiltinType(name string) bool {
	if name == "Duration" || name == "Type" {
		return true
	}
	_, ok := builtinGo[name]
	return ok
}
