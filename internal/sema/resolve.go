package sema

import (
	"go/types"
	"sort"
	"strings"

	"github.com/bongdo/ink/internal/ast"
	"github.com/bongdo/ink/internal/token"
)

var builtinTypes = map[string]string{
	"void":     "",
	"bool":     "bool",
	"i8":       "int8",
	"i16":      "int16",
	"i32":      "int32",
	"i64":      "int64",
	"u8":       "uint8",
	"u16":      "uint16",
	"u32":      "uint32",
	"u64":      "uint64",
	"int":      "int",
	"uint":     "uint",
	"usize":    "uint",
	"isize":    "int",
	"f32":      "float32",
	"f64":      "float64",
	"str":      "string",
	"rune":     "rune",
	"byte":     "byte",
	"any":      "any",
	"error":    "error",
	"Duration": "time.Duration",
	"Type":     "reflect.Type",
}

func IsBuiltinType(name string) bool {
	_, ok := builtinTypes[name]
	return ok
}

func (p *Program) resolveTypeDecl(fc *fileCtx, d *ast.TypeDecl) {
	tparams := typeParamSet(d.TypeParams)
	if len(d.TypeParams) > 0 {
		seen := make(map[string]bool)
		for _, tp := range d.TypeParams {
			if !checkDeclName(p, fc.file, tp.Name, tp.Pos()) {
				continue
			}
			if seen[tp.Name] {
				p.errorf(fc.file.Path, tp.Pos(), "type parameter %s redeclared", tp.Name)
			}
			seen[tp.Name] = true
		}
	}
	switch d.Kind {
	case ast.TypeAlias:
		d.Target = p.resolveType(fc, d.Target, tparams)
		if d.Target != nil && isVoidType(d.Target) {
			p.errorf(fc.file.Path, d.Pos(), "alias %s cannot be void", d.Name)
		}
	case ast.TypeStruct:
		for _, f := range d.Fields {
			f.Type = p.resolveType(fc, f.Type, tparams)
			if f.Type != nil && isVoidType(f.Type) {
				p.errorf(fc.file.Path, f.Pos(), "field %s cannot be void", f.Name)
			}
		}
		for _, m := range d.Methods {
			if len(m.TypeParams) > 0 {
				p.errorf(fc.file.Path, m.Pos(), "methods cannot declare type parameters")
			}
			p.resolveSignature(fc, m, tparams)
		}
		p.resolveContracts(fc, d)
	case ast.TypeEnum:
		if len(d.TypeParams) > 0 {
			p.errorf(fc.file.Path, d.Pos(), "generic enums are not supported")
		}
		for _, v := range d.Variants {
			for _, f := range v.Fields {
				f.Type = p.resolveType(fc, f.Type, tparams)
				if f.Type != nil && isVoidType(f.Type) {
					p.errorf(fc.file.Path, f.Pos(), "field %s cannot be void", f.Name)
				}
			}
		}
	case ast.TypeInterface:
		for _, m := range d.IFace {
			p.resolveIfaceMethod(fc, m, tparams)
		}
	}
}

func typeParamSet(ps []*ast.TypeParam) map[string]bool {
	m := make(map[string]bool)
	for _, p := range ps {
		m[p.Name] = true
	}
	return m
}

func mergeParams(a, b map[string]bool) map[string]bool {
	if len(a) == 0 {
		return b
	}
	m := make(map[string]bool, len(a)+len(b))
	for k := range a {
		m[k] = true
	}
	for k := range b {
		m[k] = true
	}
	return m
}

func (p *Program) resolveSignature(fc *fileCtx, fn *ast.FuncDecl, outer map[string]bool) {
	tparams := mergeParams(outer, typeParamSet(fn.TypeParams))
	seen := make(map[string]bool)
	for _, param := range fn.Params {
		if param.Name != "_" && !checkLocalName(p, fc.file, param.Name, param.Pos()) {
			continue
		}
		if param.Name != "_" && seen[param.Name] {
			p.errorf(fc.file.Path, param.Pos(), "parameter %s redeclared", param.Name)
		}
		seen[param.Name] = true
		param.Type = p.resolveType(fc, param.Type, tparams)
		if param.Type != nil && isVoidType(param.Type) {
			p.errorf(fc.file.Path, param.Pos(), "parameter %s cannot be void", param.Name)
		}
	}
	fn.Results = p.normalizeResults(fc, fn.Results, tparams)
	p.funcs[fn] = &funcInfo{decl: fn, results: fn.Results, err: fn.Err, file: fc}
}

func (p *Program) resolveIfaceMethod(fc *fileCtx, m *ast.IFaceMethod, outer map[string]bool) {
	seen := make(map[string]bool)
	for _, param := range m.Params {
		if param.Name != "_" && !checkLocalName(p, fc.file, param.Name, param.Pos()) {
			continue
		}
		if param.Name != "_" && seen[param.Name] {
			p.errorf(fc.file.Path, param.Pos(), "parameter %s redeclared", param.Name)
		}
		seen[param.Name] = true
		param.Type = p.resolveType(fc, param.Type, outer)
		p.rejectVoid(fc, param.Type, param.Pos())
	}
	m.Results = p.normalizeResults(fc, m.Results, outer)
}

func (p *Program) normalizeResults(fc *fileCtx, ts []ast.Type, tparams map[string]bool) []ast.Type {
	var out []ast.Type
	for _, t := range ts {
		if r := p.normalizeResult(fc, t, tparams); r != nil {
			out = append(out, r)
		}
	}
	return out
}

func (p *Program) normalizeResult(fc *fileCtx, t ast.Type, tparams map[string]bool) ast.Type {
	if t == nil {
		return nil
	}
	t = p.resolveType(fc, t, tparams)
	if t == nil {
		return nil
	}
	if isVoidType(t) {
		return nil
	}
	return t
}

func (p *Program) resolveType(fc *fileCtx, t ast.Type, tparams map[string]bool) ast.Type {
	if t == nil {
		return nil
	}
	switch t := t.(type) {
	case *ast.GoType:
		return t
	case *ast.NamedType:
		return p.resolveNamed(fc, t, tparams)
	case *ast.SliceType:
		t.Elem = p.resolveType(fc, t.Elem, tparams)
		p.rejectVoid(fc, t.Elem, t.Pos())
		return t
	case *ast.ArrayType:
		t.Elem = p.resolveType(fc, t.Elem, tparams)
		p.rejectVoid(fc, t.Elem, t.Pos())
		if n, ok := t.Elem.(*ast.NamedType); ok && n.Decl != nil && len(n.Decl.TypeParams) > 0 {
			p.errorf(fc.file.Path, t.Pos(), "generic type %s requires type arguments", n.Name)
		}
		t.Len = p.resolveConstExpr(fc, t.Len)
		return t
	case *ast.MapType:
		t.Key = p.resolveType(fc, t.Key, tparams)
		t.Value = p.resolveType(fc, t.Value, tparams)
		p.rejectVoid(fc, t.Key, t.Pos())
		p.rejectVoid(fc, t.Value, t.Pos())
		return t
	case *ast.PointerType:
		t.Elem = p.resolveType(fc, t.Elem, tparams)
		p.rejectVoid(fc, t.Elem, t.Pos())
		return t
	case *ast.FuncType:
		for i, param := range t.Params {
			t.Params[i] = p.resolveType(fc, param, tparams)
			p.rejectVoid(fc, t.Params[i], t.Pos())
		}
		t.Results = p.normalizeResults(fc, t.Results, tparams)
		return t
	case *ast.InterfaceType:
		for _, m := range t.Methods {
			p.resolveIfaceMethod(fc, m, tparams)
		}
		return t
	case *ast.StructType:
		for _, f := range t.Fields {
			f.Type = p.resolveType(fc, f.Type, tparams)
			p.rejectVoid(fc, f.Type, f.Pos())
		}
		return t
	case *ast.GenericType:
		return p.resolveGeneric(fc, t, tparams)
	}
	return t
}

func (p *Program) resolveNamed(fc *fileCtx, t *ast.NamedType, tparams map[string]bool) *ast.NamedType {
	if tparams[t.Name] && t.Package == "" {
		return t
	}
	if t.Package == "" {
		if IsBuiltinType(t.Name) {
			t.GoName = builtinTypes[t.Name]
			return t
		}
		if d, ok := fc.pkg.Types[t.Name]; ok {
			t.Decl = d
			t.GoName = d.GoName
			return t
		}
		p.errorf(fc.file.Path, t.Pos(), "unknown type %s", t.Name)
		return t
	}
	if imp, ok := fc.imports[t.Package]; ok {
		fc.mark(t.Package)
		if imp.Local != nil {
			d, ok := imp.Local.Types[t.Name]
			if !ok {
				p.errorf(fc.file.Path, t.Pos(), "package %s has no type %s", t.Package, t.Name)
				return t
			}
			if !exported(t.Name) && imp.Local != fc.pkg {
				p.errorf(fc.file.Path, t.Pos(), "type %s is not exported by %s", t.Name, t.Package)
				return t
			}
			t.Decl = d
			t.GoName = d.GoName
			t.PkgAlias = t.Package
			return t
		}
		if imp.Go != nil && imp.Go.Types != nil {
			obj := imp.Go.Types.Scope().Lookup(t.Name)
			if obj == nil {
				p.errorf(fc.file.Path, t.Pos(), "package %s has no type %s", t.Package, t.Name)
				return t
			}
			if _, ok := obj.(*types.TypeName); !ok {
				p.errorf(fc.file.Path, t.Pos(), "%s.%s is not a type", t.Package, t.Name)
				return t
			}
			if !obj.Exported() {
				p.errorf(fc.file.Path, t.Pos(), "type %s is not exported by %s", t.Name, t.Package)
				return t
			}
			t.GoName = t.Name
			t.PkgAlias = t.Package
			p.goNamed[t] = obj.Type()
			return t
		}
		p.errorf(fc.file.Path, t.Pos(), "unknown package %s", t.Package)
		return t
	}
	if d, ok := fc.pkg.Types[t.Package]; ok && d.Kind == ast.TypeEnum {
		v := findVariant(d, t.Name)
		if v == nil {
			p.errorf(fc.file.Path, t.Pos(), "enum %s has no variant %s", t.Package, t.Name)
			return t
		}
		t.Decl = d
		t.Variant = v
		t.GoName = v.GoName
		return t
	}
	if i := strings.LastIndexByte(t.Package, '.'); i >= 0 {
		alias := t.Package[:i]
		enumName := t.Package[i+1:]
		if imp, ok := fc.imports[alias]; ok && imp.Local != nil {
			fc.mark(alias)
			d, ok := imp.Local.Types[enumName]
			if !ok || d.Kind != ast.TypeEnum {
				p.errorf(fc.file.Path, t.Pos(), "package %s has no enum %s", alias, enumName)
				return t
			}
			if !exported(enumName) && imp.Local != fc.pkg {
				p.errorf(fc.file.Path, t.Pos(), "enum %s is not exported by %s", enumName, alias)
				return t
			}
			v := findVariant(d, t.Name)
			if v == nil {
				p.errorf(fc.file.Path, t.Pos(), "enum %s has no variant %s", enumName, t.Name)
				return t
			}
			t.Decl = d
			t.Variant = v
			t.GoName = v.GoName
			t.PkgAlias = alias
			return t
		}
	}
	p.errorf(fc.file.Path, t.Pos(), "unknown type %s.%s", t.Package, t.Name)
	return t
}

func (fc *fileCtx) mark(alias string) {
	if u := fc.uses[alias]; u != nil {
		u.Used = true
	}
}

func findVariant(d *ast.TypeDecl, name string) *ast.Variant {
	for _, v := range d.Variants {
		if v.Name == name {
			return v
		}
	}
	return nil
}

func (p *Program) resolveGeneric(fc *fileCtx, t *ast.GenericType, tparams map[string]bool) ast.Type {
	base := p.resolveType(fc, t.Base, tparams)
	t.Base = base
	if named, ok := base.(*ast.NamedType); ok {
		if named.Decl != nil && len(named.Decl.TypeParams) > 0 {
			if len(t.Args) != len(named.Decl.TypeParams) {
				p.errorf(fc.file.Path, t.Pos(), "type %s wants %d type arguments, got %d", named.Name, len(named.Decl.TypeParams), len(t.Args))
			}
			for i, a := range t.Args {
				t.Args[i] = p.resolveType(fc, a, tparams)
				p.rejectVoid(fc, t.Args[i], t.Pos())
			}
			return t
		}
		if named.Decl == nil && named.PkgAlias != "" {
			return t
		}
		if len(t.Args) == 1 {
			if lit := constExprOf(t.Args[0]); lit != nil {
				e := p.resolveConstExpr(fc, lit)
				return &ast.ArrayType{Span: t.Span, Elem: base, Len: e}
			}
		}
	}
	p.errorf(fc.file.Path, t.Pos(), "type is not generic")
	return t
}

func constExprOf(t ast.Type) ast.Expr {
	if n, ok := t.(*ast.NamedType); ok {
		if n.Package == "" {
			return &ast.Ident{Span: n.Span, Name: n.Name}
		}
		return &ast.SelectorExpr{
			Span: n.Span,
			X:    &ast.Ident{Span: n.Span, Name: n.Package},
			Name: n.Name,
		}
	}
	return nil
}

func (p *Program) resolveConstExpr(fc *fileCtx, e ast.Expr) ast.Expr {
	if e == nil {
		return nil
	}
	switch e := e.(type) {
	case *ast.Ident:
		d, ok := fc.pkg.Vars[e.Name]
		if !ok || !d.Const {
			p.errorf(fc.file.Path, e.Pos(), "%s is not a constant", e.Name)
			return e
		}
		e.Ref = &ast.Ref{Kind: ast.RefConst, Name: e.Name, Decl: d, Path: fc.file.Path}
		return e
	case *ast.SelectorExpr:
		id, ok := e.X.(*ast.Ident)
		if !ok {
			p.errorf(fc.file.Path, e.Pos(), "array length must be a constant expression")
			return e
		}
		imp := fc.imports[id.Name]
		if imp == nil {
			p.errorf(fc.file.Path, e.Pos(), "unknown package %s", id.Name)
			return e
		}
		if imp.Local == nil {
			p.errorf(fc.file.Path, e.Pos(), "constant %s.%s must be declared in a local package", id.Name, e.Name)
			return e
		}
		d, ok := imp.Local.Vars[e.Name]
		if !ok {
			p.errorf(fc.file.Path, e.Pos(), "package %s has no constant %s", id.Name, e.Name)
			return e
		}
		if !d.Const {
			p.errorf(fc.file.Path, e.Pos(), "%s.%s is not a constant", id.Name, e.Name)
			return e
		}
		if !exported(e.Name) && imp.Local != fc.pkg {
			p.errorf(fc.file.Path, e.Pos(), "constant %s is not exported by %s", e.Name, id.Name)
			return e
		}
		fc.mark(id.Name)
		id.Ref = &ast.Ref{Kind: ast.RefPackage, Name: id.Name, Use: fc.uses[id.Name]}
		e.Ref = &ast.Ref{Kind: ast.RefConst, Name: e.Name, Decl: d, Path: fc.file.Path}
		return e
	case *ast.BasicLit:
		return e
	case *ast.UnaryExpr:
		e.X = p.resolveConstExpr(fc, e.X)
		return e
	case *ast.BinaryExpr:
		e.X = p.resolveConstExpr(fc, e.X)
		e.Y = p.resolveConstExpr(fc, e.Y)
		return e
	case *ast.ParenExpr:
		e.X = p.resolveConstExpr(fc, e.X)
		return e
	case *ast.CallExpr:
		for i, a := range e.Args {
			e.Args[i] = p.resolveConstExpr(fc, a)
		}
		return e
	}
	p.errorf(fc.file.Path, e.Pos(), "array length must be a constant expression")
	return e
}

func isVoidType(t ast.Type) bool {
	n, ok := t.(*ast.NamedType)
	return ok && n.Package == "" && n.Name == "void"
}

func (p *Program) rejectVoid(fc *fileCtx, t ast.Type, pos token.Pos) {
	if isVoidType(t) {
		p.errorf(fc.file.Path, pos, "void is only allowed as a function result")
	}
}

func exported(name string) bool {
	for _, r := range name {
		return r >= 'A' && r <= 'Z'
	}
	return false
}

func (p *Program) finalChecks() {
	for _, pkg := range p.Packages {
		for _, f := range pkg.Files {
			for _, u := range f.Uses {
				if !u.Used && u.Alias != "_" {
					p.errorf(f.Path, u.Pos(), "import %s is not used", u.Alias)
				}
			}
		}
	}
}

func (p *Program) checkMain() {
	if p.Main == nil {
		return
	}
	main := p.Main.Funcs["main"]
	if main == nil {
		p.errorf(p.Main.Files[0].Path, p.Main.Files[0].Pos(), "package main has no fn main")
		return
	}
	if len(main.Params) != 0 {
		p.errorf(p.Main.Files[0].Path, main.Pos(), "main takes no parameters")
	}
	if len(main.Results) > 0 {
		p.errorf(p.Main.Files[0].Path, main.Pos(), "main returns void or void!")
	}
}

func (p *Program) checkEnumVariants(pkg *Package) {
	seen := make(map[string]*ast.TypeDecl)
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			td, ok := d.(*ast.TypeDecl)
			if !ok || td.Kind != ast.TypeEnum {
				continue
			}
			for _, v := range td.Variants {
				if prev := seen[v.Name]; prev != nil && prev != td {
					p.errorf(f.Path, v.Pos(), "enum variant %s is already declared in %s", v.Name, prev.Name)
					continue
				}
				seen[v.Name] = td
			}
		}
	}
}

type packageVar struct {
	fc       *fileCtx
	decl     *ast.VarDecl
	declType ast.Type
}

func (p *Program) checkPackageVars() {
	for _, pkg := range p.Packages {
		p.checkEnumVariants(pkg)
		p.global[pkg] = make(map[string]ast.Type)
		for name := range pkg.Vars {
			p.global[pkg][name] = nil
		}
		var vars []packageVar
		for _, f := range pkg.Files {
			fc := p.fileCtx(pkg, f)
			for _, d := range f.Decls {
				v, ok := d.(*ast.VarDecl)
				if !ok {
					continue
				}
				c := &checker{p: p, fc: fc}
				var declType ast.Type
				if v.Type != nil {
					if v.Const {
						p.errorf(f.Path, v.Pos(), "const cannot have a type")
					}
					declType = c.resolveType(v.Type)
					v.Type = declType
					p.rejectVoid(fc, declType, v.Pos())
				}
				item := packageVar{fc: fc, decl: v, declType: declType}
				vars = append(vars, item)
				p.resolvePackageVar(item)
			}
		}
		for {
			changed := false
			for _, item := range vars {
				if !p.packageVarUnresolved(pkg, item) {
					continue
				}
				if p.resolvePackageVarRetry(item) {
					changed = true
				}
			}
			if !changed {
				break
			}
		}
	}
	p.dedupeLitCalls()
}

func (p *Program) dedupeLitCalls() {
	seen := make(map[*ast.FuncLit]bool, len(p.litCalls))
	out := p.litCalls[:0]
	for _, lc := range p.litCalls {
		if lc.lit == nil || seen[lc.lit] {
			continue
		}
		seen[lc.lit] = true
		out = append(out, lc)
	}
	p.litCalls = out
}

func (p *Program) resolvePackageVar(item packageVar) {
	c := &checker{p: p, fc: item.fc}
	v := item.decl
	for _, val := range v.Values {
		if _, ok := val.(*ast.OrExpr); ok {
			p.errorf(item.fc.file.Path, val.Pos(), "or is not allowed in a package-level initializer")
		}
		c.applyNil(val, item.declType)
		c.checkMatchValue(val)
		c.resolveExpr(val, true)
		c.checkVoidValue(val)
		c.checkAssign(item.declType, val)
	}
	var ts []ast.Type
	for _, val := range v.Values {
		ts = append(ts, c.exprType(val))
	}
	for i, name := range v.Names {
		if name == "_" {
			continue
		}
		t := item.declType
		if t == nil && len(ts) == len(v.Names) {
			t = ts[i]
		}
		p.global[item.fc.pkg][name] = t
	}
}

func (p *Program) resolvePackageVarRetry(item packageVar) bool {
	pkg := item.fc.pkg
	before := make(map[string]ast.Type, len(item.decl.Names))
	for _, name := range item.decl.Names {
		if name != "_" {
			before[name] = p.global[pkg][name]
		}
	}
	errs, edges, lits := len(p.Errors), len(p.edges), len(p.litCalls)
	p.resolvePackageVar(item)
	added := append([]Error(nil), p.Errors[errs:]...)
	p.Errors = p.Errors[:errs]
	for _, e := range added {
		if !p.hasError(e) {
			p.Errors = append(p.Errors, e)
		}
	}
	changed := false
	for name, old := range before {
		if old == nil && p.global[pkg][name] != nil {
			changed = true
		}
	}
	if !changed {
		p.edges = p.edges[:edges]
		p.litCalls = p.litCalls[:lits]
	}
	return changed
}

func (p *Program) hasError(e Error) bool {
	for _, have := range p.Errors {
		if have.Path == e.Path && have.Pos == e.Pos && have.Msg == e.Msg {
			return true
		}
	}
	return false
}

func (p *Program) packageVarUnresolved(pkg *Package, item packageVar) bool {
	if len(item.decl.Values) == 0 || item.declType != nil {
		return false
	}
	for _, name := range item.decl.Names {
		if name != "_" && p.global[pkg][name] == nil {
			return true
		}
	}
	return false
}

func (p *Program) checkVarCyclesAll() {
	for _, pkg := range p.Packages {
		var vars []packageVar
		for _, f := range pkg.Files {
			fc := p.fileCtx(pkg, f)
			for _, d := range f.Decls {
				if v, ok := d.(*ast.VarDecl); ok {
					vars = append(vars, packageVar{fc: fc, decl: v})
				}
			}
		}
		p.checkVarCycles(pkg, vars)
	}
}

func (p *Program) checkVarCycles(pkg *Package, vars []packageVar) {
	if len(vars) == 0 {
		return
	}
	members := make(map[*ast.VarDecl]bool, len(vars))
	for _, item := range vars {
		members[item.decl] = true
	}
	reported := make(map[string]bool)
	for _, item := range vars {
		seen := make(map[*ast.VarDecl]bool)
		var path []*ast.VarDecl
		cycle := findVarCycle(item.decl, members, seen, &path)
		if cycle == nil {
			continue
		}
		names := make([]string, 0, len(cycle))
		keyNames := make([]string, 0, len(cycle))
		for _, d := range cycle {
			names = append(names, varDeclName(d))
			keyNames = append(keyNames, varDeclName(d))
		}
		sort.Strings(keyNames)
		key := strings.Join(keyNames, "|")
		if reported[key] {
			continue
		}
		reported[key] = true
		p.errorf(item.fc.file.Path, item.decl.Pos(), "initialization cycle: %s", strings.Join(names, " -> "))
	}
}

func findVarCycle(start *ast.VarDecl, members map[*ast.VarDecl]bool, seen map[*ast.VarDecl]bool, path *[]*ast.VarDecl) []*ast.VarDecl {
	return walkVarCycle(start, start, members, seen, path)
}

func walkVarCycle(cur, start *ast.VarDecl, members map[*ast.VarDecl]bool, seen map[*ast.VarDecl]bool, path *[]*ast.VarDecl) []*ast.VarDecl {
	if cur == nil || seen[cur] {
		return nil
	}
	seen[cur] = true
	*path = append(*path, cur)
	for _, next := range varRefs(cur) {
		if next == start {
			return append([]*ast.VarDecl(nil), *path...)
		}
		if members[next] {
			if cycle := walkVarCycle(next, start, members, seen, path); cycle != nil {
				return cycle
			}
		}
	}
	*path = (*path)[:len(*path)-1]
	return nil
}

func varRefs(v *ast.VarDecl) []*ast.VarDecl {
	var refs []*ast.VarDecl
	seen := make(map[*ast.VarDecl]bool)
	funcs := make(map[*ast.FuncDecl]bool)
	var scan func(ast.Node)
	scan = func(root ast.Node) {
		if root == nil {
			return
		}
		invoked := invokedFuncLits(root)
		ast.Inspect(root, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				return invoked[n]
			case *ast.CallExpr:
				id, ok := unparen(n.Fun).(*ast.Ident)
				if !ok || id.Ref == nil || id.Ref.Func == nil || funcs[id.Ref.Func] {
					return true
				}
				funcs[id.Ref.Func] = true
				scan(id.Ref.Func.Body)
			case *ast.Ident:
				if n.Ref == nil || n.Ref.Kind != ast.RefGlobal {
					return true
				}
				if d, ok := n.Ref.Decl.(*ast.VarDecl); ok && !seen[d] {
					seen[d] = true
					refs = append(refs, d)
				}
			}
			return true
		})
	}
	for _, val := range v.Values {
		scan(val)
	}
	return refs
}

func invokedFuncLits(n ast.Node) map[*ast.FuncLit]bool {
	invoked := make(map[*ast.FuncLit]bool)
	ast.Inspect(n, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if lit := calleeFuncLit(call.Fun); lit != nil {
				invoked[lit] = true
			}
		}
		return true
	})
	return invoked
}

func calleeFuncLit(e ast.Expr) *ast.FuncLit {
	for {
		paren, ok := e.(*ast.ParenExpr)
		if !ok {
			break
		}
		e = paren.X
	}
	lit, _ := e.(*ast.FuncLit)
	return lit
}

func varDeclName(v *ast.VarDecl) string {
	for _, name := range v.Names {
		if name != "_" {
			return name
		}
	}
	return "var"
}
