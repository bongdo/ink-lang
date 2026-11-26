package sema

import (
	"go/types"
	"sort"
	"strconv"

	"github.com/bongdo/ink/internal/ast"
	"github.com/bongdo/ink/internal/token"
)

type symbol struct {
	name       string
	kind       ast.RefKind
	typ        ast.Type
	goType     types.Type
	decl       ast.Node
	field      *ast.Field
	fn         *ast.FuncDecl
	typD       *ast.TypeDecl
	use        *ast.Use
	arena      bool
	arenaDepth int
	nilMap     bool
	served     bool
	used       bool
}

type scope struct {
	parent *scope
	names  map[string]*symbol
}

type parFrame struct {
	iter      map[string]bool
	plain     bool
	loopDecls map[ast.Node]bool
}

type checker struct {
	p               *Program
	fc              *fileCtx
	fn              *funcInfo
	scopes          []*scope
	loopDepth       int
	parFrames       []parFrame
	parLoopBase     []int
	inPar           bool
	inListener      int
	inInlineHandler int
	inTaskHandler   int
	inTaskCapture   map[*symbol]bool
	listeners       []bool
	parListeners    []bool
	parArenas       []int
	inArena         int
	inServeTarget   int
	inAssignLhs     int
	blockDepth      int
	started         bool
	prevStmt        ast.Stmt
	blockIndex      int
	tparams         map[string]bool
}

func (c *checker) push() {
	c.scopes = append(c.scopes, &scope{names: make(map[string]*symbol), parent: c.top()})
}

func (c *checker) pop() {
	c.scopes = c.scopes[:len(c.scopes)-1]
}

func (c *checker) top() *scope {
	if len(c.scopes) == 0 {
		return nil
	}
	return c.scopes[len(c.scopes)-1]
}

func (c *checker) declare(sym *symbol) {
	if sym.name == "_" {
		return
	}
	if builtinReserved(sym.name) {
		pos := token.Pos{}
		if sym.decl != nil {
			pos = sym.decl.Pos()
		}
		c.p.errorf(c.fc.file.Path, pos, "%s is a builtin name; rename it", sym.name)
	}
	sym.arenaDepth = c.inArena
	if sc := c.top(); sc != nil {
		sc.names[sym.name] = sym
	}
}

func (c *checker) lookupLocal(name string) *symbol {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if s := c.scopes[i].names[name]; s != nil {
			return s
		}
	}
	return nil
}

func (c *checker) lookup(name string) *symbol {
	if s := c.lookupLocal(name); s != nil {
		return s
	}
	if d := c.fc.pkg.Types[name]; d != nil {
		return &symbol{name: name, kind: ast.RefType, typ: &ast.NamedType{Span: d.Span, Name: name, Decl: d, GoName: d.GoName}, decl: d, typD: d}
	}
	if d := c.fc.pkg.Funcs[name]; d != nil {
		return &symbol{name: name, kind: ast.RefFunc, typ: c.funcType(d), decl: d, fn: d}
	}
	if d := c.fc.pkg.Vars[name]; d != nil {
		kind := ast.RefGlobal
		if d.Const {
			kind = ast.RefConst
		}
		return &symbol{name: name, kind: kind, typ: c.p.global[c.fc.pkg][name], decl: d, served: d.Served}
	}
	if imp := c.fc.imports[name]; imp != nil {
		return &symbol{name: name, kind: ast.RefPackage, use: c.fc.uses[name], typ: nil}
	}
	if IsBuiltinType(name) {
		return &symbol{name: name, kind: ast.RefType, typ: &ast.NamedType{Name: name, GoName: builtinTypes[name]}}
	}
	if isBuiltinFunc(name) {
		return &symbol{name: name, kind: ast.RefBuiltin, typ: nil}
	}
	return nil
}

func (c *checker) funcType(fn *ast.FuncDecl) ast.Type {
	ft := &ast.FuncType{Span: fn.Span, Results: fn.Results, Err: fn.Err}
	for _, p := range fn.Params {
		ft.Params = append(ft.Params, p.Type)
	}
	return ft
}

func isMapType(t ast.Type) bool {
	t = unwrapAliasType(t)
	switch t := t.(type) {
	case *ast.MapType:
		return true
	case *ast.NamedType:
		return t.Decl != nil && t.Decl.Kind == ast.TypeAlias && isMapType(t.Decl.Target)
	}
	return false
}

func isNilLit(e ast.Expr) bool {
	_, ok := unparen(e).(*ast.NilLit)
	return ok
}

func builtinReserved(name string) bool {
	if _, ok := builtinTypes[name]; ok {
		return true
	}
	return isBuiltinFunc(name)
}

func isBuiltinFunc(name string) bool {
	switch name {
	case "len", "cap", "append", "copy", "delete", "has", "panic", "assert", "assert_eq", "min", "max", "runes", "bytes", "make", "new", "asset", "check", "print":
		return true
	}
	return false
}

func (p *Program) checkFreeMethods() {
	for _, pkg := range p.Packages {
		count := make(map[*ast.TypeDecl]int)
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				v, ok := d.(*ast.VarDecl)
				if !ok {
					continue
				}
				for _, val := range v.Values {
					if lit, ok := val.(*ast.FuncLit); ok {
						countFreeReceiver(p, pkg, lit, count)
					}
				}
			}
		}
		names := make([]string, 0, len(pkg.Funcs))
		for name := range pkg.Funcs {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fn := pkg.Funcs[name]
			if fn == nil || fn.Owner != nil || len(fn.Params) == 0 || fn.Params[0].Type == nil {
				continue
			}
			ptr, ok := fn.Params[0].Type.(*ast.PointerType)
			if !ok {
				continue
			}
			named, ok := ptr.Elem.(*ast.NamedType)
			if !ok || named.Decl == nil || named.Decl.Kind != ast.TypeStruct || p.declPkg[named.Decl] != pkg {
				continue
			}
			count[named.Decl]++
		}
		decls := make([]*ast.TypeDecl, 0, len(count))
		for decl := range count {
			decls = append(decls, decl)
		}
		sort.Slice(decls, func(i, j int) bool { return decls[i].Pos().Offset < decls[j].Pos().Offset })
		for _, decl := range decls {
			p.warnf(p.fileOfDecl(pkg, decl), decl.Pos(), "type %s is passed as *%s to %d free functions; state and behaviour belong in the type body", decl.Name, decl.Name, count[decl])
		}
	}
}

func countFreeReceiver(p *Program, pkg *Package, lit *ast.FuncLit, count map[*ast.TypeDecl]int) {
	if lit == nil || len(lit.Params) == 0 || lit.Params[0].Type == nil {
		return
	}
	ptr, ok := lit.Params[0].Type.(*ast.PointerType)
	if !ok {
		return
	}
	named, ok := ptr.Elem.(*ast.NamedType)
	if !ok || named.Decl == nil || named.Decl.Kind != ast.TypeStruct || p.declPkg[named.Decl] != pkg {
		return
	}
	count[named.Decl]++
}

func (p *Program) fileOfDecl(pkg *Package, decl *ast.TypeDecl) string {
	for _, f := range pkg.Files {
		for _, d := range f.Decls {
			if d == ast.Decl(decl) {
				return f.Path
			}
		}
	}
	return pkg.Files[0].Path
}

func (p *Program) checkFuncs() {
	for _, pkg := range p.Packages {
		for _, f := range pkg.Files {
			fc := p.fileCtx(pkg, f)
			for _, d := range f.Decls {
				if p.stopped() {
					return
				}
				switch d := d.(type) {
				case *ast.FuncDecl:
					c := &checker{p: p, fc: fc, fn: p.funcs[d]}
					c.checkFunction(d, nil)
				case *ast.TypeDecl:
					for _, m := range d.Methods {
						c := &checker{p: p, fc: fc, fn: p.funcs[m]}
						c.checkFunction(m, nil)
					}
				}
			}
		}
	}
}

func (p *Program) checkServeFuncs() {
	for _, pkg := range p.Packages {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				switch d := d.(type) {
				case *ast.FuncDecl:
					count := 0
					for _, param := range d.Params {
						if servedParamDecl(param.Type) != nil {
							count++
						}
					}
					if count > 1 {
						p.errorf(f.Path, d.Pos(), "a free function may serve one object; split %s", d.Name)
					}
					if count == 1 && len(d.TypeParams) > 0 {
						p.errorf(f.Path, d.Pos(), "serve does not support generic functions")
					}
					if count == 1 {
						for _, param := range d.Params {
							if sd := servedParamDecl(param.Type); sd != nil && p.declPkg[sd] != pkg {
								p.errorf(f.Path, param.Pos(), "a served object cannot cross packages; keep the function in the type's package")
								break
							}
						}
					}
					for _, param := range d.Params {
						if sd := servedStructDecl(param.Type); sd != nil && sd.Served {
							if _, ok := param.Type.(*ast.PointerType); !ok {
								p.errorf(f.Path, param.Pos(), "a served type is not copyable; take %s", "*"+sd.Name)
							}
						}
					}
				case *ast.TypeDecl:
					if !d.Served {
						continue
					}
					for _, m := range d.Methods {
						if m.NeedsCtx {
							p.errorf(f.Path, m.Pos(), "a served method cannot take a context; keep it outside the object")
						}
						if len(m.TypeParams) > 0 {
							p.errorf(f.Path, m.Pos(), "serve does not support generic methods")
						}
						for _, param := range m.Params {
							if servedParamDecl(param.Type) != nil {
								p.errorf(f.Path, param.Pos(), "a served method cannot take another served object; pass values or call it outside")
								break
							}
						}
					}
				}
			}
		}
	}
}

func (c *checker) checkFunction(fn *ast.FuncDecl, outer *checker) {
	if outer != nil {
		c.scopes = outer.scopes
	}
	c.push()
	c.tparams = mergeParams(c.tparams, typeParamSet(fn.TypeParams))
	if fn.Owner != nil {
		c.tparams = mergeParams(c.tparams, typeParamSet(fn.Owner.TypeParams))
		for _, tp := range fn.Owner.TypeParams {
			c.declare(&symbol{name: tp.Name, kind: ast.RefType, typ: &ast.NamedType{Span: tp.Span, Name: tp.Name}})
		}
	}
	for _, tp := range fn.TypeParams {
		c.declare(&symbol{name: tp.Name, kind: ast.RefType, typ: &ast.NamedType{Span: tp.Span, Name: tp.Name}})
	}
	if fn.Owner != nil {
		c.push()
		c.declare(&symbol{name: "self", kind: ast.RefSelf, typ: &ast.PointerType{Span: fn.Owner.Span, Elem: &ast.NamedType{Span: fn.Owner.Span, Name: fn.Owner.Name, Decl: fn.Owner, GoName: fn.Owner.GoName}}, decl: fn})
		for _, fld := range fn.Owner.Fields {
			c.declare(&symbol{name: fld.Name, kind: ast.RefField, typ: fld.Type, decl: fld, field: fld})
		}
	}
	for _, param := range fn.Params {
		c.declare(&symbol{name: param.Name, kind: ast.RefParam, typ: param.Type, goType: c.goTypeOf(param.Type), decl: param})
	}
	c.checkBlock(fn.Body)
	if c.fn != nil && len(c.fn.results) > 0 && !terminatesBlock(fn.Body) {
		pos := fn.EndPos()
		if fn.Body != nil {
			pos = fn.Body.End
		}
		c.p.errorf(c.fc.file.Path, pos, "missing return in function returning %s", resultName(c.fn.results))
	}
	if outer != nil {
		c.scopes = nil
	}
}

func resultName(results []ast.Type) string {
	if len(results) > 1 {
		return "multiple values"
	}
	return inkTypeName(results[0])
}

func inkTypeName(t ast.Type) string {
	n, ok := t.(*ast.NamedType)
	if !ok || n.Decl != nil || n.Package != "" {
		return typeDesc(t)
	}
	if n.PkgAlias != "" {
		return n.PkgAlias + "." + n.Name
	}
	return n.Name
}

func arenaRef(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.PointerType, *ast.SliceType, *ast.MapType, *ast.FuncType:
		return true
	case *ast.ArrayType:
		return arenaRef(t.Elem)
	case *ast.NamedType:
		if t.Package == "" && (t.Name == "any" || t.Name == "error") {
			return true
		}
		if t.Decl != nil {
			switch t.Decl.Kind {
			case ast.TypeInterface, ast.TypeEnum:
				return true
			case ast.TypeStruct:
				for _, f := range t.Decl.Fields {
					if arenaRef(f.Type) {
						return true
					}
				}
			case ast.TypeAlias:
				if t.Decl.Target != nil {
					return arenaRef(t.Decl.Target)
				}
			}
		}
	}
	return false
}

func hasPointers(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.PointerType, *ast.SliceType, *ast.MapType, *ast.FuncType:
		return true
	case *ast.ArrayType:
		return hasPointers(t.Elem)
	case *ast.NamedType:
		if t.Decl != nil {
			switch t.Decl.Kind {
			case ast.TypeInterface, ast.TypeEnum:
				return true
			case ast.TypeStruct:
				for _, f := range t.Decl.Fields {
					if hasPointers(f.Type) {
						return true
					}
				}
				return false
			case ast.TypeAlias:
				if t.Decl.Target != nil {
					return hasPointers(t.Decl.Target)
				}
				return false
			}
		}
		if t.Package != "" {
			return true
		}
		switch t.Name {
		case "bool", "i8", "i16", "i32", "i64", "u8", "u16", "u32", "u64",
			"int", "uint", "f32", "f64", "rune", "byte", "Duration":
			return false
		}
		return true
	}
	return true
}

func (c *checker) markFullArenaWrites(body *ast.Block) {
	if body == nil {
		return
	}
	for i := 0; i+1 < len(body.Stmts); i++ {
		decl, ok := body.Stmts[i].(*ast.DeclStmt)
		if !ok || decl.Decl == nil || len(decl.Decl.Names) != 1 || len(decl.Decl.Values) != 1 {
			continue
		}
		call, ok := unparen(decl.Decl.Values[0]).(*ast.CallExpr)
		if !ok || call.Info == nil || call.Info.Builtin != "make" || len(call.TypeArgs) != 1 || len(call.Args) != 1 {
			continue
		}
		if hasPointers(call.TypeArgs[0]) {
			continue
		}
		loop, ok := body.Stmts[i+1].(*ast.ForStmt)
		if !ok || !c.fullWriteLoop(loop, decl.Decl.Names[0], call.Args[0]) {
			continue
		}
		call.Info.FullWrite = true
	}
}

func (c *checker) fullWriteLoop(f *ast.ForStmt, name string, size ast.Expr) bool {
	if f.Range != nil || f.Body == nil || len(f.Body.Stmts) != 1 {
		return false
	}
	idx := loopIndex(f.Init)
	if idx == "" {
		return false
	}
	cond, ok := unparen(f.Cond).(*ast.BinaryExpr)
	if !ok || cond.Op != token.Lt {
		return false
	}
	left, ok := unparen(cond.X).(*ast.Ident)
	if !ok || left.Name != idx || !c.sameBound(name, size, cond.Y) {
		return false
	}
	if !incOne(f.Post, idx) {
		return false
	}
	as, ok := f.Body.Stmts[0].(*ast.AssignStmt)
	if !ok || as.Op != token.Assign || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return false
	}
	ix, ok := unparen(as.Lhs[0]).(*ast.IndexExpr)
	if !ok {
		return false
	}
	base, ok := unparen(ix.X).(*ast.Ident)
	if !ok || base.Name != name {
		return false
	}
	at, ok := unparen(ix.Index).(*ast.Ident)
	if !ok || at.Name != idx {
		return false
	}
	return !mentions(as.Rhs[0], name) && !fallible(as.Rhs[0])
}

func loopIndex(s ast.Stmt) string {
	switch s := s.(type) {
	case *ast.DeclStmt:
		if s.Decl == nil || len(s.Decl.Names) != 1 || len(s.Decl.Values) != 1 || !zeroLit(s.Decl.Values[0]) {
			return ""
		}
		return s.Decl.Names[0]
	case *ast.AssignStmt:
		if s.Op != token.Assign || len(s.Lhs) != 1 || len(s.Rhs) != 1 || !zeroLit(s.Rhs[0]) {
			return ""
		}
		if id, ok := unparen(s.Lhs[0]).(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

func zeroLit(e ast.Expr) bool {
	n, ok := intLiteral(e)
	return ok && n == 0
}

func incOne(s ast.Stmt, idx string) bool {
	as, ok := s.(*ast.AssignStmt)
	if !ok || as.Op != token.PlusAssign || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return false
	}
	id, idOK := unparen(as.Lhs[0]).(*ast.Ident)
	n, nOK := intLiteral(as.Rhs[0])
	return idOK && nOK && id.Name == idx && n == 1
}

func (c *checker) sameBound(name string, size, bound ast.Expr) bool {
	if call, ok := unparen(bound).(*ast.CallExpr); ok {
		if id, ok := unparen(call.Fun).(*ast.Ident); ok && id.Name == "len" && len(call.Args) == 1 {
			if arg, ok := unparen(call.Args[0]).(*ast.Ident); ok && arg.Name == name {
				return true
			}
		}
	}
	if n, ok := intLiteral(size); ok {
		m, ok := intLiteral(bound)
		return ok && n == m
	}
	si, ok := unparen(size).(*ast.Ident)
	if !ok {
		return false
	}
	bi, ok := unparen(bound).(*ast.Ident)
	if !ok || bi.Name != si.Name {
		return false
	}
	sym := c.lookup(si.Name)
	return sym != nil && sym.kind == ast.RefConst
}

func mentions(e ast.Expr, name string) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return !found
	})
	return found
}

func fallible(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.TryExpr, *ast.OnErrExpr:
			found = true
		}
		return !found
	})
	return found
}

func (c *checker) isArenaExpr(e ast.Expr) bool {
	e = unparen(e)
	switch e := e.(type) {
	case *ast.OnErrExpr:
		return c.isArenaExpr(e.X)
	case *ast.Ident:
		if sym := c.lookupLocal(e.Name); sym != nil {
			return sym.arena
		}
		return false
	case *ast.CallExpr:
		if e.Info != nil {
			if (e.Info.Builtin == "new" || e.Info.Builtin == "make") && c.inArena > 0 {
				return true
			}
			if e.Info.Builtin == "append" {
				for _, a := range e.Args {
					if c.isArenaExpr(a) {
						return true
					}
				}
				return false
			}
		}
		if sel, ok := e.Fun.(*ast.SelectorExpr); ok && c.isArenaExpr(sel.X) && arenaRef(c.exprType(e)) {
			return true
		}
		if arenaRef(c.exprType(e)) {
			for _, a := range e.Args {
				if c.isArenaExpr(a) {
					return true
				}
			}
		}
		return false
	case *ast.SelectorExpr:
		return c.isArenaExpr(e.X) && arenaRef(c.exprType(e))
	case *ast.IndexExpr:
		return c.isArenaExpr(e.X) && arenaRef(c.exprType(e))
	case *ast.SliceExpr:
		return c.isArenaExpr(e.X) && arenaRef(c.exprType(e))
	case *ast.FuncLit:
		return c.inArena > 0
	case *ast.UnaryExpr:
		if e.Op == token.Amp {
			return c.arenaLocation(e.X)
		}
	case *ast.CompositeLit:
		for _, en := range e.Entries {
			if c.isArenaExpr(en.Value) || (en.KeyExpr != nil && c.isArenaExpr(en.KeyExpr)) {
				return true
			}
		}
	}
	return false
}

func (c *checker) arenaLocation(e ast.Expr) bool {
	switch e := unparen(e).(type) {
	case *ast.Ident:
		if sym := c.lookupLocal(e.Name); sym != nil {
			return sym.arena
		}
		return false
	case *ast.SelectorExpr:
		return c.isArenaExpr(e.X) || c.arenaLocation(e.X)
	case *ast.IndexExpr:
		return c.isArenaExpr(e.X) || c.arenaLocation(e.X)
	case *ast.SliceExpr:
		return c.isArenaExpr(e.X) || c.arenaLocation(e.X)
	case *ast.UnaryExpr:
		if e.Op == token.Star {
			return c.isArenaExpr(e.X) || c.arenaLocation(e.X)
		}
	}
	return false
}

func (c *checker) arenaTargetOutside(e ast.Expr) bool {
	switch e := unparen(e).(type) {
	case *ast.Ident:
		if e.Name == "_" {
			return false
		}
		sym := c.lookup(e.Name)
		return sym == nil || sym.arenaDepth < c.inArena
	case *ast.SelectorExpr:
		return c.arenaTargetOutside(e.X)
	case *ast.IndexExpr:
		return c.arenaTargetOutside(e.X)
	}
	return false
}

func (c *checker) markArenaTarget(e ast.Expr) {
	for {
		switch t := unparen(e).(type) {
		case *ast.Ident:
			if sym := c.lookupLocal(t.Name); sym != nil {
				sym.arena = true
			}
			return
		case *ast.SelectorExpr:
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.SliceExpr:
			e = t.X
		default:
			return
		}
	}
}

func (c *checker) checkArenaCall(call *ast.CallExpr) {
	if call == nil {
		return
	}
	if call.Info != nil {
		switch call.Info.Builtin {
		case "len", "cap", "min", "max":
			return
		case "append":
			return
		case "copy":
			c.checkArenaCopy(call)
			return
		}
	}
	if sel, ok := unparen(call.Fun).(*ast.SelectorExpr); ok && c.isArenaExpr(sel.X) {
		c.p.errorf(c.fc.file.Path, sel.X.Pos(), "method call on an arena value; read its fields instead")
		return
	}
	for _, a := range call.Args {
		if c.isArenaExpr(a) {
			c.p.errorf(c.fc.file.Path, a.Pos(), "arena value cannot leave the arena block; read a field or an index instead")
			return
		}
	}
}

func (c *checker) checkArenaCopy(call *ast.CallExpr) {
	for _, a := range call.Args {
		if !c.isArenaExpr(a) {
			continue
		}
		t := c.exprType(a)
		if st, ok := t.(*ast.SliceType); ok {
			t = st.Elem
		}
		if !arenaRef(t) {
			continue
		}
		c.p.errorf(c.fc.file.Path, a.Pos(), "arena value cannot leave the arena block; read a field or an index instead")
		return
	}
}

func terminatesBlock(b *ast.Block) bool {
	if b == nil || len(b.Stmts) == 0 {
		return false
	}
	return terminatesStmt(b.Stmts[len(b.Stmts)-1])
}

func terminatesStmt(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ArenaStmt:
		return terminatesBlock(s.Body)
	case *ast.Block:
		return terminatesBlock(s)
	case *ast.IfStmt:
		return s.Else != nil && terminatesStmt(s.Then) && terminatesStmt(s.Else)
	case *ast.ExprStmt:
		if _, ok := unparen(s.X).(*ast.TryExpr); ok {
			return true
		}
		if m, ok := unparen(s.X).(*ast.MatchExpr); ok {
			return terminatesMatch(m)
		}
		if call, ok := unparen(s.X).(*ast.CallExpr); ok && call.Info != nil && call.Info.Builtin == "check" {
			return true
		}
		return isPanicExpr(s.X)
	case *ast.ForStmt:
		if s.Par || s.Range != nil || s.Cond != nil {
			return false
		}
		return !blockBreaks(s.Body)
	case *ast.OnErrStmt:
		return terminatesBlock(s.Body)
	}
	return false
}

func terminatesExpr(e ast.Expr) bool {
	switch e := unparen(e).(type) {
	case *ast.CallExpr:
		return isPanicExpr(e)
	case *ast.MatchExpr:
		return e.IsExpr && terminatesMatch(e)
	}
	return false
}

func isPanicExpr(e ast.Expr) bool {
	call, ok := unparen(e).(*ast.CallExpr)
	return ok && call.Info != nil && call.Info.Kind == ast.CallBuiltin && call.Info.Builtin == "panic"
}

func terminatesMatch(m *ast.MatchExpr) bool {
	if m == nil || len(m.Cases) == 0 {
		return false
	}
	for _, c := range m.Cases {
		if !terminatesCase(c) {
			return false
		}
	}
	return true
}

func terminatesCase(c *ast.Case) bool {
	if c == nil {
		return false
	}
	if c.Block != nil {
		return terminatesBlock(c.Block)
	}
	if len(c.Body) > 0 {
		return terminatesStmt(c.Body[len(c.Body)-1])
	}
	if c.Expr != nil {
		return terminatesExpr(c.Expr)
	}
	return false
}

func blockBreaks(b *ast.Block) bool {
	if b == nil {
		return false
	}
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ForStmt:
			return false
		case *ast.BreakStmt:
			found = true
			return false
		}
		return !found
	})
	return found
}

func (c *checker) constLimit(e ast.Expr) (int64, bool) {
	switch e := unparen(e).(type) {
	case *ast.BasicLit:
		if e.Kind != token.Int {
			return 0, false
		}
		n, err := strconv.ParseInt(e.Value, 0, 64)
		return n, err == nil
	case *ast.Ident:
		sym := c.lookup(e.Name)
		if sym == nil {
			return 0, false
		}
		decl, ok := sym.decl.(*ast.VarDecl)
		if !ok || !decl.Const || len(decl.Values) != 1 {
			return 0, false
		}
		return c.constLimit(decl.Values[0])
	case *ast.UnaryExpr:
		if e.Op == token.Minus {
			n, ok := c.constLimit(e.X)
			return -n, ok
		}
		if e.Op == token.Plus {
			return c.constLimit(e.X)
		}
	}
	return 0, false
}

func (c *checker) hasListener() bool {
	return len(c.listeners) > 0 && c.listeners[len(c.listeners)-1]
}

func (c *checker) parAllowsPropagation() bool {
	return len(c.parListeners) > 0 && c.parListeners[len(c.parListeners)-1]
}

func (c *checker) checkBlock(b *ast.Block) {
	if b == nil {
		return
	}
	c.blockDepth++
	defer func() { c.blockDepth-- }()
	has := c.hasListener()
	if len(b.Stmts) > 0 {
		if _, ok := b.Stmts[0].(*ast.OnErrStmt); ok {
			has = true
		}
	}
	c.listeners = append(c.listeners, has)
	c.push()
	var prev ast.Stmt
	if c.blockDepth == 1 {
		c.prevStmt = nil
	}
	for i, s := range b.Stmts {
		if _, ok := s.(*ast.OnErrStmt); ok && i != 0 {
			c.p.errorf(c.fc.file.Path, s.Pos(), "on err must be the first statement of its block")
		}
		if c.blockDepth == 1 {
			c.prevStmt = prev
			c.blockIndex = i
		}
		c.checkStmt(s)
		prev = s
	}
	c.pop()
	c.listeners = c.listeners[:len(c.listeners)-1]
}

func (p *Program) checkTests() {
	for _, pkg := range p.Packages {
		seen := make(map[string]int)
		for _, f := range pkg.Files {
			fc := p.fileCtx(pkg, f)
			for _, d := range f.Decls {
				t, ok := d.(*ast.TestDecl)
				if !ok {
					continue
				}
				if t.Name == "" {
					p.errorf(f.Path, t.Pos(), "test name is empty")
				}
				id := testGoID(t.Name)
				if n := seen[id]; n > 0 {
					seen[id] = n + 1
					id = id + "_" + strconv.Itoa(n)
				} else {
					seen[id] = 1
				}
				t.GoID = id
				fi := &funcInfo{results: nil, err: true, file: fc, test: t}
				c := &checker{p: p, fc: fc, fn: fi}
				c.checkBlock(t.Body)
			}
		}
	}
}

func testGoID(name string) string {
	var b []rune
	up := true
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			if up && r >= 'a' && r <= 'z' {
				r = r - 'a' + 'A'
			}
			b = append(b, r)
			up = false
		} else {
			up = true
		}
	}
	if len(b) == 0 {
		return "Unnamed"
	}
	if b[0] >= '0' && b[0] <= '9' {
		b = append([]rune("T"), b...)
	}
	return string(b)
}

func (c *checker) checkStmt(s ast.Stmt) {
	if s == nil {
		return
	}
	switch s := s.(type) {
	case *ast.Block:
		c.checkBlock(s)
	case *ast.EmptyStmt:
	case *ast.DeclStmt:
		c.checkVarDecl(s.Decl)
	case *ast.AssignStmt:
		var lhsTypes []ast.Type
		c.inAssignLhs++
		for _, e := range s.Lhs {
			lhsTypes = append(lhsTypes, c.resolveExpr(e, false))
			if sel, ok := unparen(e).(*ast.SelectorExpr); ok && c.mapStructElement(sel.X) {
				c.p.errorf(c.fc.file.Path, e.Pos(), "cannot assign to a field of a map element; copy out, mutate, then write back")
			}
			if c.nilMapWrite(e) {
				c.p.warnf(c.fc.file.Path, e.Pos(), "write to a nil map; initialize it with map[K,V] {}")
			}
			c.clearNilMap(e)
		}
		c.inAssignLhs--
		for i, e := range s.Rhs {
			c.checkMatchValue(e)
			if len(s.Rhs) == len(lhsTypes) {
				c.applyNil(e, lhsTypes[i])
			}
			c.resolveExpr(e, true)
			c.checkVoidValue(e)
			if len(s.Rhs) == len(lhsTypes) {
				c.checkAssign(lhsTypes[i], e)
			}
			if c.servedValueCopy(e) {
				c.p.errorf(c.fc.file.Path, e.Pos(), "a served type is not copyable; use a pointer")
			}
			if len(s.Lhs) == 1 && len(s.Rhs) == 1 {
				blank := false
				if id, ok := s.Lhs[0].(*ast.Ident); ok && id.Name == "_" {
					blank = true
				}
				if call, ok := e.(*ast.CallExpr); ok && call.Info != nil && call.Info.Results > 1 && !blank {
					c.p.errorf(c.fc.file.Path, call.Pos(), "multi-result call needs matching names")
				}
			}
		}
		if len(s.Rhs) == len(s.Lhs) {
			for i, rhs := range s.Rhs {
				if c.servedBody() && c.escapesServe(rhs) && !appendBack(s.Lhs[i], rhs) {
					c.p.errorf(c.fc.file.Path, rhs.Pos(), "do not alias served state inside its method; copy the values")
				}
				if c.servedAliasExpr(rhs) {
					c.markServedTarget(s.Lhs[i])
				}
			}
		}
		if len(s.Rhs) == 1 {
			if call, ok := orCall(s.Rhs[0]); ok && call.Info.Results != len(s.Lhs) {
				c.p.errorf(c.fc.file.Path, call.Pos(), "wrong number of results: got %d, want %d", call.Info.Results, len(s.Lhs))
			}
		}
		if len(s.Rhs) > 1 {
			for _, e := range s.Rhs {
				if hasOr(e) {
					c.p.errorf(c.fc.file.Path, e.Pos(), "or needs a single right side")
				}
			}
		}
		if c.inArena > 0 && len(s.Rhs) == len(s.Lhs) {
			for i, lhs := range s.Lhs {
				if c.isArenaExpr(s.Rhs[i]) {
					if c.arenaTargetOutside(lhs) {
						c.p.errorf(c.fc.file.Path, lhs.Pos(), "arena value escapes the arena block")
					} else {
						c.markArenaTarget(lhs)
					}
				}
			}
		}
	case *ast.ExprStmt:
		if m, ok := unparen(s.X).(*ast.MatchExpr); !ok || m.IsExpr {
			c.checkMatchValue(s.X)
		}
		c.resolveExpr(s.X, true)
		if call, ok := unparen(s.X).(*ast.CallExpr); ok && call.Info != nil && call.Info.HasError && !isPanicExpr(call) {
			if call.Info.Builtin == "check" {
				if c.inTaskHandler > 0 {
					c.p.errorf(c.fc.file.Path, s.X.Pos(), "check cannot leave a spawn task; handle the error here")
				} else if c.fn == nil || (!c.fn.err && !(c.inPar && c.parAllowsPropagation())) {
					c.p.errorf(c.fc.file.Path, s.X.Pos(), "check in a function that cannot fail; add ! to the result or handle it with on err")
				}
			} else {
				c.p.errorf(c.fc.file.Path, s.X.Pos(), "fallible result is dropped; use ?, on err, or _ = ... to discard")
			}
		}
	case *ast.IfStmt:
		c.resolveExpr(s.Cond, false)
		c.checkErrGuard(s)
		c.checkBlock(s.Then)
		if s.Else != nil {
			c.push()
			c.checkStmt(s.Else)
			c.pop()
		}
	case *ast.ForStmt:
		c.checkFor(s)
	case *ast.ReturnStmt:
		if c.inInlineHandler > 0 && c.fn != nil && c.fn.err {
			c.p.warnf(c.fc.file.Path, s.Pos(), "return in on err turns the failure into success: the caller gets a value and a nil error; use ? on the call to propagate")
		}
		if c.inListener > 0 {
			c.p.errorf(c.fc.file.Path, s.Pos(), "return cannot leave an on err listener; log, panic or handle")
		}
		if c.inTaskHandler > 0 {
			c.p.errorf(c.fc.file.Path, s.Pos(), "return cannot leave a spawn task; log or panic")
		}
		if c.fn == nil {
			c.p.errorf(c.fc.file.Path, s.Pos(), "return outside a function")
		}
		if c.fn != nil {
			for i, e := range s.Results {
				var want ast.Type
				if i < len(c.fn.results) {
					want = c.fn.results[i]
				} else if c.fn.err && i == len(c.fn.results) {
					want = &ast.NamedType{Name: "error", GoName: "error"}
				}
				c.applyNil(e, want)
			}
		}
		for i, e := range s.Results {
			c.checkMatchValue(e)
			c.resolveExpr(e, true)
			c.checkVoidValue(e)
			if c.servedBody() && c.escapesServe(e) {
				c.p.errorf(c.fc.file.Path, e.Pos(), "returning a reference to served state escapes the lock; copy the value")
			}
			if c.servedValueCopy(e) {
				c.p.errorf(c.fc.file.Path, e.Pos(), "a served type is not copyable; return a pointer")
			}
			var want ast.Type
			if c.fn != nil {
				if i < len(c.fn.results) {
					want = c.fn.results[i]
				} else if c.fn.err && i == len(c.fn.results) {
					want = &ast.NamedType{Name: "error", GoName: "error"}
				}
			}
			c.checkAssign(want, e)
		}
		if len(s.Results) > 1 {
			for _, e := range s.Results {
				if hasOr(e) {
					c.p.errorf(c.fc.file.Path, e.Pos(), "or needs a single return value")
				}
			}
		}
		if c.fn != nil {
			n := len(c.fn.results)
			var orc *ast.CallExpr
			if len(s.Results) == 1 {
				orc, _ = orCall(s.Results[0])
			}
			switch {
			case orc != nil && orc.Info.Results != n:
				c.p.errorf(c.fc.file.Path, s.Pos(), "wrong number of return values: got %d, want %d", orc.Info.Results, n)
			case orc != nil:
			case len(s.Results) != n && !(c.fn.err && len(s.Results) == n+1):
				if n == 0 && !c.fn.err && len(s.Results) > 0 {
					c.p.errorf(c.fc.file.Path, s.Pos(), "return value in a void function")
				} else if len(s.Results) == 1 {
					if call, ok := s.Results[0].(*ast.CallExpr); ok && call.Info != nil && call.Info.Results == n && (n > 0 || call.Info.HasError) {
					} else {
						c.p.errorf(c.fc.file.Path, s.Pos(), "wrong number of return values: got %d, want %d", len(s.Results), n)
					}
				} else {
					c.p.errorf(c.fc.file.Path, s.Pos(), "wrong number of return values: got %d, want %d", len(s.Results), n)
				}
			}
		}
		if c.inArena > 0 {
			for _, e := range s.Results {
				if c.isArenaExpr(e) {
					c.p.errorf(c.fc.file.Path, e.Pos(), "arena value escapes the arena block")
					break
				}
			}
		}
		if c.inPar {
			c.p.errorf(c.fc.file.Path, s.Pos(), "return cannot leave a par block; use ?")
		}
	case *ast.BreakStmt:
		if c.loopDepth == 0 {
			c.p.errorf(c.fc.file.Path, s.Pos(), "break outside a loop")
		}
		if c.crossesPar() {
			c.p.errorf(c.fc.file.Path, s.Pos(), "break cannot leave a par block")
		}
	case *ast.ContinueStmt:
		if c.loopDepth == 0 {
			c.p.errorf(c.fc.file.Path, s.Pos(), "continue outside a loop")
		}
		if c.crossesPar() {
			c.p.errorf(c.fc.file.Path, s.Pos(), "continue cannot leave a par block")
		}
	case *ast.DeferStmt:
		if s.Call == nil {
			c.p.errorf(c.fc.file.Path, s.Pos(), "defer expects a call")
		} else {
			c.resolveExpr(s.Call, false)
		}
	case *ast.ServeStmt:
		c.checkServe(s)
	case *ast.ArenaStmt:
		c.inArena++
		c.checkBlock(s.Body)
		c.inArena--
		c.markFullArenaWrites(s.Body)
	case *ast.ParStmt:
		c.checkPar(s.Body, nil)
	case *ast.GoCallStmt:
		c.checkGoCall(s)
	case *ast.OnErrStmt:
		c.checkOnErr(s)
	}
}

func (p *Program) markServedGlobals() {
	for _, pkg := range p.Packages {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				s, ok := n.(*ast.ServeStmt)
				if !ok {
					return true
				}
				if id, ok := unparen(s.X).(*ast.Ident); ok {
					if d := pkg.Vars[id.Name]; d != nil {
						d.Served = true
						if t := p.global[pkg][id.Name]; t != nil {
							if sd := servedStructDecl(t); sd != nil && len(sd.TypeParams) == 0 {
								sd.Served = true
							}
						}
					}
				}
				return true
			})
		}
	}
}

func (p *Program) servedTypeDeclIn(t ast.Type, pkg *Package) *ast.TypeDecl {
	for t != nil {
		switch x := t.(type) {
		case *ast.PointerType:
			t = x.Elem
		case *ast.GenericType:
			t = x.Base
		case *ast.NamedType:
			if x.Decl != nil {
				if x.Decl.Kind == ast.TypeAlias {
					t = x.Decl.Target
					continue
				}
				if x.Decl.Kind == ast.TypeStruct {
					return x.Decl
				}
				return nil
			}
			if d := pkg.Types[x.Name]; d != nil && d.Kind == ast.TypeStruct {
				return d
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}

func (p *Program) markServedTypes() {
	for _, pkg := range p.Packages {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				types := make(map[string]ast.Type)
				for name, t := range p.global[pkg] {
					types[name] = t
				}
				for _, param := range fn.Params {
					types[param.Name] = param.Type
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					switch n := n.(type) {
					case *ast.VarDecl:
						for i, name := range n.Names {
							var t ast.Type
							if n.Type != nil {
								t = n.Type
							} else if i < len(n.Values) {
								t = p.exprTypeOf(n.Values[i], types, pkg)
							}
							if t != nil {
								types[name] = t
							}
						}
					}
					return true
				})
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					s, ok := n.(*ast.ServeStmt)
					if !ok {
						return true
					}
					if id, ok := unparen(s.X).(*ast.Ident); ok {
						if t := types[id.Name]; t != nil {
							if sd := p.servedTypeDeclIn(t, pkg); sd != nil && len(sd.TypeParams) == 0 {
								sd.Served = true
							}
						}
					}
					return true
				})
			}
		}
	}
}

func (p *Program) exprTypeOf(e ast.Expr, vars map[string]ast.Type, pkg *Package) ast.Type {
	switch x := unparen(e).(type) {
	case *ast.CompositeLit:
		return x.Type
	case *ast.UnaryExpr:
		if x.Op == token.Amp {
			return p.exprTypeOf(x.X, vars, pkg)
		}
	case *ast.Ident:
		return vars[x.Name]
	case *ast.CallExpr:
		if id, ok := x.Fun.(*ast.Ident); ok {
			if fd := pkg.Funcs[id.Name]; fd != nil && len(fd.Results) == 1 {
				return fd.Results[0]
			}
		}
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			if d := p.servedTypeDeclIn(p.exprTypeOf(sel.X, vars, pkg), pkg); d != nil {
				for _, m := range d.Methods {
					if m.Name == sel.Name && len(m.Results) == 1 {
						return m.Results[0]
					}
				}
			}
		}
	}
	return nil
}

func (c *checker) checkServe(s *ast.ServeStmt) {
	if s.X == nil {
		return
	}
	c.inServeTarget++
	c.resolveExpr(s.X, false)
	c.inServeTarget--
	id, ok := unparen(s.X).(*ast.Ident)
	if !ok {
		c.p.errorf(c.fc.file.Path, s.X.Pos(), "serve needs a whole object: a local or package variable, not a field")
		return
	}
	sym := c.lookup(id.Name)
	if sym == nil {
		return
	}
	if sym.kind == ast.RefParam || sym.kind == ast.RefSelf {
		c.p.errorf(c.fc.file.Path, s.X.Pos(), "serve an object at creation, not a parameter or self")
		return
	}
	if sym.kind == ast.RefLocal && sym.used {
		c.p.errorf(c.fc.file.Path, s.X.Pos(), "serve must run before the object is used")
		return
	}
	inMain := c.fn != nil && c.fn.decl != nil && c.fn.decl.Name == "main"
	inTest := c.fn != nil && c.fn.test != nil
	if sym.kind == ast.RefGlobal && !inMain && !inTest {
		c.p.errorf(c.fc.file.Path, s.X.Pos(), "serve a package variable in main or a test, before other goroutines start")
		return
	}
	decl := servedStructDecl(c.exprType(s.X))
	if decl == nil {
		c.p.errorf(c.fc.file.Path, s.X.Pos(), "serve needs a struct value")
		return
	}
	if c.p.declPkg[decl] != c.fc.pkg {
		c.p.errorf(c.fc.file.Path, s.X.Pos(), "serve only works on a type from the same package")
		return
	}
	if len(decl.TypeParams) > 0 {
		c.p.errorf(c.fc.file.Path, s.X.Pos(), "serve does not support generic types")
		return
	}
	if c.inPar || c.inTaskHandler > 0 {
		c.p.errorf(c.fc.file.Path, s.Pos(), "serve must run before sharing; not inside par or spawn")
		return
	}
	if c.blockDepth > 1 {
		c.p.errorf(c.fc.file.Path, s.Pos(), "serve must run at the top level of a function, before sharing")
		return
	}
	if c.started {
		c.p.errorf(c.fc.file.Path, s.Pos(), "serve must run before par or spawn in the same function")
		return
	}
	if prev, ok := c.prevStmt.(*ast.ServeStmt); ok {
		if pid, ok := unparen(prev.X).(*ast.Ident); ok && pid.Name == id.Name {
			c.p.errorf(c.fc.file.Path, s.Pos(), "%s is already served", id.Name)
			return
		}
	}
	first := sym.kind == ast.RefGlobal && c.blockIndex == 0
	afterServe := false
	afterListener := false
	if sym.kind == ast.RefGlobal {
		switch c.prevStmt.(type) {
		case *ast.ServeStmt:
			afterServe = true
		case *ast.OnErrStmt:
			afterListener = c.blockIndex == 1
		}
	}
	if !first && !afterServe && !afterListener && !prevDefines(c.prevStmt, id.Name) {
		c.p.errorf(c.fc.file.Path, s.Pos(), "serve must directly follow the declaration or assignment of %s", id.Name)
		return
	}
	decl.Served = true
	id.Ref.Served = true
	sym.served = true
	if sym.kind == ast.RefGlobal {
		if d, ok := sym.decl.(*ast.VarDecl); ok {
			d.Served = true
		}
	}
}

func appendBack(lhs, rhs ast.Expr) bool {
	call, ok := unparen(rhs).(*ast.CallExpr)
	if !ok || call.Info == nil || call.Info.Builtin != "append" || len(call.Args) == 0 {
		return false
	}
	return sameTarget(lhs, call.Args[0])
}

func sameTarget(a, b ast.Expr) bool {
	switch x := unparen(a).(type) {
	case *ast.Ident:
		y, ok := unparen(b).(*ast.Ident)
		return ok && x.Name == y.Name
	case *ast.SelectorExpr:
		y, ok := unparen(b).(*ast.SelectorExpr)
		return ok && x.Name == y.Name && sameTarget(x.X, y.X)
	}
	return false
}

func prevDefines(s ast.Stmt, name string) bool {
	switch s := s.(type) {
	case *ast.DeclStmt:
		if s.Decl == nil {
			return false
		}
		for _, n := range s.Decl.Names {
			if n == name {
				return true
			}
		}
	case *ast.AssignStmt:
		for _, lhs := range s.Lhs {
			if id, ok := unparen(lhs).(*ast.Ident); ok && id.Name == name {
				return true
			}
		}
	}
	return false
}

func (c *checker) checkServedField(s *ast.SelectorExpr) {
	if c.inServeTarget > 0 || !c.servedBase(s.X) || c.servedFieldAllowed(s.X) {
		return
	}
	c.p.errorf(c.fc.file.Path, s.Pos(), "fields of a served object are internal; use a method or a free function with *T")
}

func (c *checker) servedBase(e ast.Expr) bool {
	switch x := unparen(e).(type) {
	case *ast.Ident:
		if x.Ref != nil && x.Ref.Served {
			return true
		}
		if x.Ref != nil && x.Ref.Kind == ast.RefParam {
			if sym := c.lookup(x.Name); sym != nil && servedParamDecl(sym.typ) != nil {
				return true
			}
		}
		if x.Ref != nil && x.Ref.Kind == ast.RefSelf && c.fn != nil && c.fn.decl != nil &&
			c.fn.decl.Owner != nil && c.fn.decl.Owner.Served {
			return true
		}
	case *ast.SelectorExpr:
		if x.Ref != nil && x.Ref.Served {
			return true
		}
		if c.servedBase(x.X) {
			return true
		}
	case *ast.IndexExpr:
		if c.servedBase(x.X) {
			return true
		}
	case *ast.UnaryExpr:
		if x.Op == token.Star && c.servedBase(x.X) {
			return true
		}
	}
	d := servedStructDecl(c.exprType(e))
	return d != nil && d.Served
}

func (c *checker) servedFieldAllowed(e ast.Expr) bool {
	if c.fn == nil {
		return false
	}
	id, ok := unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	if id.Ref != nil && id.Ref.Kind == ast.RefSelf {
		return c.fn.decl != nil && c.fn.decl.Owner != nil && c.fn.decl.Owner.Served
	}
	if c.fn.decl == nil || c.fn.decl.Owner != nil {
		return false
	}
	sym := c.lookup(id.Name)
	return sym != nil && sym.kind == ast.RefParam && servedParamDecl(sym.typ) != nil
}

func servedParamDecl(t ast.Type) *ast.TypeDecl {
	if _, ok := t.(*ast.PointerType); !ok {
		return nil
	}
	return servedPtrDecl(t)
}

func servedPtrDecl(t ast.Type) *ast.TypeDecl {
	for t != nil {
		switch x := t.(type) {
		case *ast.PointerType:
			t = x.Elem
		case *ast.GenericType:
			t = x.Base
		case *ast.NamedType:
			if x.Decl == nil {
				return nil
			}
			if x.Decl.Kind == ast.TypeAlias {
				t = x.Decl.Target
				continue
			}
			if x.Decl.Kind == ast.TypeStruct && x.Decl.Served {
				return x.Decl
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}

func servedStructDecl(t ast.Type) *ast.TypeDecl {
	for t != nil {
		switch x := t.(type) {
		case *ast.PointerType:
			t = x.Elem
		case *ast.GenericType:
			t = x.Base
		case *ast.NamedType:
			if x.Decl == nil {
				return nil
			}
			if x.Decl.Kind == ast.TypeAlias {
				t = x.Decl.Target
				continue
			}
			if x.Decl.Kind == ast.TypeStruct {
				return x.Decl
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}

func (c *checker) checkOnErr(s *ast.OnErrStmt) {
	if s.Body == nil || len(s.Body.Stmts) == 0 {
		c.p.errorf(c.fc.file.Path, s.Pos(), "on err must do something")
	}
	c.push()
	c.declare(&symbol{name: "err", kind: ast.RefLocal, typ: &ast.NamedType{Name: "error", GoName: "error"}, decl: s})
	c.inListener++
	c.checkBlock(s.Body)
	c.inListener--
	c.pop()
}

func (c *checker) checkGoCall(s *ast.GoCallStmt) {
	c.started = true
	if c.servedBody() {
		c.p.errorf(c.fc.file.Path, s.Pos(), "spawn inside a served method; the lock is already held")
	}
	var handler *ast.Block
	expr := s.Call
	if or, ok := unparen(s.Call).(*ast.OnErrExpr); ok {
		handler = or.Block
		expr = or.X
	} else if t, ok := unparen(s.Call).(*ast.TryExpr); ok {
		c.p.errorf(c.fc.file.Path, t.Pos(), "spawn takes on err; there is no caller to return to")
		c.resolveExpr(t.X, false)
		return
	}
	call, ok := unparen(expr).(*ast.CallExpr)
	if !ok {
		c.p.errorf(c.fc.file.Path, s.Pos(), "spawn expects a call")
		return
	}
	if sel, ok := unparen(call.Fun).(*ast.SelectorExpr); ok && c.inArena > 0 {
		if c.isArenaExpr(sel.X) || c.arenaLocation(sel.X) {
			c.p.errorf(c.fc.file.Path, sel.X.Pos(), "arena value cannot leave the arena block; read a field or an index instead")
		}
	}
	switch unparen(call.Fun).(type) {
	case *ast.Ident, *ast.SelectorExpr:
	default:
		c.p.errorf(c.fc.file.Path, call.Fun.Pos(), "spawn expects a function or method name; no closures")
		return
	}
	t := c.resolveExpr(call, false)
	if call.Info == nil {
		return
	}
	if call.Info.Results > 0 {
		c.p.errorf(c.fc.file.Path, call.Pos(), "spawn call must not return a value; use par { ... } to collect results")
		return
	}
	if !call.Info.HasError && t != nil && !isVoidType(t) {
		c.p.errorf(c.fc.file.Path, call.Pos(), "spawn call must not return a value; use par { ... } to collect results")
		return
	}
	if call.Info.HasError && handler == nil {
		c.p.errorf(c.fc.file.Path, call.Pos(), "spawn call can fail; handle it with on err")
		return
	}
	if handler != nil {
		if !call.Info.HasError {
			c.p.errorf(c.fc.file.Path, handler.Pos(), "on err on a call that cannot fail")
		}
		c.checkTaskHandler(handler)
	}
}

func (c *checker) checkTaskHandler(b *ast.Block) {
	if b == nil || len(b.Stmts) == 0 {
		c.p.errorf(c.fc.file.Path, b.Pos(), "on err must do something")
	}
	captured := make(map[*symbol]bool)
	for _, sc := range c.scopes {
		for _, sym := range sc.names {
			captured[sym] = true
		}
	}
	saved := c.inTaskCapture
	c.inTaskCapture = captured
	c.inTaskHandler++
	c.push()
	c.declare(&symbol{name: "err", kind: ast.RefLocal, typ: &ast.NamedType{Name: "error", GoName: "error"}, decl: b})
	c.checkBlock(b)
	c.pop()
	c.inTaskHandler--
	c.inTaskCapture = saved
}

func (c *checker) checkErrGuard(s *ast.IfStmt) {
	err := errGuard(s.Cond)
	if err == nil || s.Then == nil || len(s.Then.Stmts) != 1 {
		return
	}
	ret, ok := s.Then.Stmts[0].(*ast.ReturnStmt)
	if !ok || !c.isErrorAstType(c.exprType(err)) {
		return
	}
	for _, r := range ret.Results {
		if id, ok := unparen(r).(*ast.Ident); ok && sameIdent(err, id) {
			c.p.errorf(c.fc.file.Path, s.Pos(), "use ? instead of if err != nil { return err }")
			return
		}
	}
}

func errGuard(cond ast.Expr) *ast.Ident {
	bin, ok := unparen(cond).(*ast.BinaryExpr)
	if !ok || bin.Op != token.Ne {
		return nil
	}
	x, y := unparen(bin.X), unparen(bin.Y)
	if _, ok := x.(*ast.NilLit); ok {
		x, y = y, x
	}
	if _, ok := y.(*ast.NilLit); !ok {
		return nil
	}
	id, _ := x.(*ast.Ident)
	return id
}

func sameIdent(a, b *ast.Ident) bool {
	if a.Name != b.Name {
		return false
	}
	return a.Ref == nil || b.Ref == nil || a.Ref.Decl == b.Ref.Decl
}

func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

func (c *checker) isErrorAstType(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.NamedType:
		if t.Decl != nil && t.Decl.Kind == ast.TypeAlias {
			return c.isErrorAstType(t.Decl.Target)
		}
		return t.Package == "" && t.Name == "error"
	case *ast.GoType:
		return isErrorType(c.p.goOf[t])
	}
	return false
}

func (c *checker) checkFor(f *ast.ForStmt) {
	if f.Par {
		c.started = true
	}
	if f.Par && c.servedBody() {
		c.p.errorf(c.fc.file.Path, f.Pos(), "par for inside a served method; the lock is already held")
	}
	c.push()
	if f.Range != nil {
		t := c.resolveExpr(f.Range.X, false)
		iter := make(map[string]bool)
		for _, n := range f.Range.Names {
			iter[n] = true
		}
		elem, key, isMap, known := c.rangeTypes(t)
		f.Range.Known = known
		f.Range.IsMap = isMap
		f.Range.KeyType = key
		f.Range.ElemType = elem
		if f.Par && isMap {
			c.p.errorf(c.fc.file.Path, f.Range.Pos(), "map iteration is not allowed in par for; collect keys into a slice first")
		}
		switch len(f.Range.Names) {
		case 1:
			if isMap {
				c.declare(&symbol{name: f.Range.Names[0], kind: ast.RefLocal, typ: key, decl: f.Range})
			} else {
				c.declare(&symbol{name: f.Range.Names[0], kind: ast.RefLocal, typ: elem, decl: f.Range})
			}
		case 2:
			c.declare(&symbol{name: f.Range.Names[0], kind: ast.RefLocal, typ: key, decl: f.Range})
			c.declare(&symbol{name: f.Range.Names[1], kind: ast.RefLocal, typ: elem, decl: f.Range})
		}
		if f.Limit != nil {
			c.resolveExpr(f.Limit, false)
			if n, ok := c.constLimit(f.Limit); ok && n < 1 {
				c.p.errorf(c.fc.file.Path, f.Limit.Pos(), "limit must be positive")
			}
		}
		if f.Par {
			frame := parFrame{iter: iter, loopDecls: map[ast.Node]bool{f.Range: true}}
			c.parFrames = append(c.parFrames, frame)
			oldInPar := c.inPar
			c.inPar = true
			c.parArenas = append(c.parArenas, c.inArena)
			c.parListeners = append(c.parListeners, c.hasListener())
			if f.Body != nil && len(f.Body.Stmts) > 0 {
				if _, ok := f.Body.Stmts[0].(*ast.OnErrStmt); ok {
					c.p.errorf(c.fc.file.Path, f.Body.Stmts[0].Pos(), "on err must be in a block, not a par branch")
				}
			}
			c.loopDepth++
			c.parLoopBase = append(c.parLoopBase, c.loopDepth)
			c.checkBlock(f.Body)
			c.parListeners = c.parListeners[:len(c.parListeners)-1]
			c.parLoopBase = c.parLoopBase[:len(c.parLoopBase)-1]
			c.loopDepth--
			c.inPar = oldInPar
			c.parArenas = c.parArenas[:len(c.parArenas)-1]
			c.parFrames = c.parFrames[:len(c.parFrames)-1]
			c.checkParWrites(f.Body, frame)
		} else {
			c.loopDepth++
			c.checkBlock(f.Body)
			c.loopDepth--
		}
	} else {
		if f.Limit != nil {
			c.resolveExpr(f.Limit, false)
			if n, ok := c.constLimit(f.Limit); ok && n < 1 {
				c.p.errorf(c.fc.file.Path, f.Limit.Pos(), "limit must be positive")
			}
		}
		if f.Init == nil && f.Cond == nil && f.Post == nil && (c.inPar || f.Par) {
			c.p.errorf(c.fc.file.Path, f.Pos(), "for () is not allowed inside par; use a bounded loop")
		}
		if f.Init != nil {
			c.checkStmt(f.Init)
		}
		if f.Cond != nil {
			c.resolveExpr(f.Cond, false)
		}
		if f.Post != nil {
			if stmtHasOr(f.Post) {
				c.p.errorf(c.fc.file.Path, f.Post.Pos(), "or is not supported in a for post statement")
			}
			c.checkStmt(f.Post)
		}
		if f.Par && f.Init != nil {
			switch init := f.Init.(type) {
			case *ast.DeclStmt:
			case *ast.AssignStmt:
				if init.Op != token.Assign {
					c.p.errorf(c.fc.file.Path, init.Pos(), "par for init must assign simple names")
				}
				for _, l := range init.Lhs {
					if _, ok := l.(*ast.Ident); !ok {
						c.p.errorf(c.fc.file.Path, l.Pos(), "par for init must assign simple names")
					}
				}
			default:
				c.p.errorf(c.fc.file.Path, f.Init.Pos(), "par for init must declare or assign simple names")
			}
		}
		if f.Par {
			iter := make(map[string]bool)
			loopDecls := make(map[ast.Node]bool)
			switch init := f.Init.(type) {
			case *ast.DeclStmt:
				if init.Decl != nil {
					for _, n := range init.Decl.Names {
						iter[n] = true
					}
					loopDecls[init.Decl] = true
				}
			case *ast.AssignStmt:
				for _, l := range init.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						iter[id.Name] = true
					}
				}
			}
			frame := parFrame{iter: iter, loopDecls: loopDecls}
			c.parFrames = append(c.parFrames, frame)
			oldInPar := c.inPar
			c.inPar = true
			c.parArenas = append(c.parArenas, c.inArena)
			c.parListeners = append(c.parListeners, c.hasListener())
			if f.Body != nil && len(f.Body.Stmts) > 0 {
				if _, ok := f.Body.Stmts[0].(*ast.OnErrStmt); ok {
					c.p.errorf(c.fc.file.Path, f.Body.Stmts[0].Pos(), "on err must be in a block, not a par branch")
				}
			}
			c.loopDepth++
			c.parLoopBase = append(c.parLoopBase, c.loopDepth)
			c.checkBlock(f.Body)
			c.parListeners = c.parListeners[:len(c.parListeners)-1]
			c.parLoopBase = c.parLoopBase[:len(c.parLoopBase)-1]
			c.loopDepth--
			c.inPar = oldInPar
			c.parArenas = c.parArenas[:len(c.parArenas)-1]
			c.parFrames = c.parFrames[:len(c.parFrames)-1]
			c.checkParWrites(f.Body, frame)
		} else {
			c.loopDepth++
			c.checkBlock(f.Body)
			c.loopDepth--
		}
	}
	c.pop()
}

func (c *checker) rangeTypes(t ast.Type) (elem, key ast.Type, isMap, known bool) {
	switch t := t.(type) {
	case *ast.SliceType:
		return t.Elem, &ast.NamedType{Name: "int", GoName: "int"}, false, true
	case *ast.ArrayType:
		return t.Elem, &ast.NamedType{Name: "int", GoName: "int"}, false, true
	case *ast.MapType:
		return t.Value, t.Key, true, true
	case *ast.NamedType:
		if t.Decl != nil && t.Decl.Kind == ast.TypeAlias {
			return c.rangeTypes(t.Decl.Target)
		}
		if t.Decl == nil && t.Package == "" && t.Name == "str" {
			return &ast.NamedType{Name: "rune", GoName: "rune"}, &ast.NamedType{Name: "int", GoName: "int"}, false, true
		}
		if gt := c.goTypeOf(t); gt != nil {
			if m, ok := gt.Underlying().(*types.Map); ok {
				return c.mapGoType(m.Elem(), t.Pos()), c.mapGoType(m.Key(), t.Pos()), true, true
			}
			if s, ok := gt.Underlying().(*types.Slice); ok {
				return c.mapGoType(s.Elem(), t.Pos()), &ast.NamedType{Name: "int", GoName: "int"}, false, true
			}
		}
	case *ast.GoType:
		if gt := c.p.goOf[t]; gt != nil {
			switch u := gt.Underlying().(type) {
			case *types.Map:
				return c.mapGoType(u.Elem(), t.Pos()), c.mapGoType(u.Key(), t.Pos()), true, true
			case *types.Slice:
				return c.mapGoType(u.Elem(), t.Pos()), &ast.NamedType{Name: "int", GoName: "int"}, false, true
			case *types.Array:
				return c.mapGoType(u.Elem(), t.Pos()), &ast.NamedType{Name: "int", GoName: "int"}, false, true
			case *types.Basic:
				if u.Kind() == types.String {
					return &ast.NamedType{Name: "rune", GoName: "rune"}, &ast.NamedType{Name: "int", GoName: "int"}, false, true
				}
			}
		}
	}
	return nil, nil, false, false
}

func (c *checker) servedBody() bool {
	if c.fn == nil || c.fn.decl == nil {
		return false
	}
	if c.fn.decl.Owner != nil {
		return c.fn.decl.Owner.Served
	}
	for _, p := range c.fn.decl.Params {
		if servedParamDecl(p.Type) != nil {
			return true
		}
	}
	return false
}

func (c *checker) heldServedExpr(e ast.Expr) bool {
	switch x := unparen(e).(type) {
	case *ast.Ident:
		if x.Ref != nil && x.Ref.Kind == ast.RefSelf {
			return c.fn != nil && c.fn.decl != nil && c.fn.decl.Owner != nil && c.fn.decl.Owner.Served
		}
		if x.Ref != nil && x.Ref.Kind == ast.RefParam {
			sym := c.lookup(x.Name)
			return sym != nil && servedParamDecl(sym.typ) != nil
		}
	case *ast.UnaryExpr:
		return x.Op == token.Amp && c.heldServedExpr(x.X)
	}
	return false
}

func referenceType(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.PointerType, *ast.SliceType, *ast.MapType:
		return true
	case *ast.NamedType:
		if t.Decl != nil && t.Decl.Kind == ast.TypeAlias {
			return referenceType(t.Decl.Target)
		}
	case *ast.GenericType:
		return referenceType(t.Base)
	}
	return false
}

func containsReference(t ast.Type) bool {
	return containsReferenceSeen(t, make(map[*ast.TypeDecl]bool))
}

func containsReferenceSeen(t ast.Type, seen map[*ast.TypeDecl]bool) bool {
	switch t := t.(type) {
	case *ast.PointerType, *ast.SliceType, *ast.MapType, *ast.FuncType, *ast.InterfaceType:
		return true
	case *ast.ArrayType:
		return containsReferenceSeen(t.Elem, seen)
	case *ast.GenericType:
		if containsReferenceSeen(t.Base, seen) {
			return true
		}
		for _, a := range t.Args {
			if containsReferenceSeen(a, seen) {
				return true
			}
		}
	case *ast.NamedType:
		if t.Decl == nil {
			return false
		}
		if t.Decl.Kind == ast.TypeAlias {
			return containsReferenceSeen(t.Decl.Target, seen)
		}
		if t.Decl.Kind != ast.TypeStruct || seen[t.Decl] {
			return false
		}
		seen[t.Decl] = true
		for _, fld := range t.Decl.Fields {
			if containsReferenceSeen(fld.Type, seen) {
				return true
			}
		}
	}
	return false
}

func (c *checker) escapesServe(e ast.Expr) bool {
	if c.heldServedExpr(e) {
		return true
	}
	if lit, ok := unparen(e).(*ast.CompositeLit); ok {
		for _, entry := range lit.Entries {
			if c.escapesServe(entry.Value) {
				return true
			}
		}
		return false
	}
	if call, ok := unparen(e).(*ast.CallExpr); ok && call.Info != nil && call.Info.Builtin == "append" && len(call.Args) > 0 {
		if c.escapesServe(call.Args[0]) {
			return true
		}
	}
	if c.servedRootAny(e) && containsReference(c.exprType(e)) {
		return true
	}
	if !referenceType(c.exprType(e)) {
		return false
	}
	switch x := unparen(e).(type) {
	case *ast.Ident:
		if x.Ref != nil && x.Ref.Kind == ast.RefField && c.fn != nil && c.fn.decl != nil &&
			c.fn.decl.Owner != nil && c.fn.decl.Owner.Served {
			return true
		}
	case *ast.SelectorExpr:
		return c.heldServedExpr(x.X)
	case *ast.IndexExpr:
		return c.escapesServe(x.X)
	case *ast.SliceExpr:
		return c.escapesServe(x.X)
	case *ast.UnaryExpr:
		if x.Op == token.Amp {
			return c.servedRootAny(x.X)
		}
		return c.escapesServe(x.X)
	}
	return false
}

func (c *checker) servedRootAny(e ast.Expr) bool {
	if c.heldServedExpr(e) {
		return true
	}
	switch x := unparen(e).(type) {
	case *ast.Ident:
		return x.Ref != nil && x.Ref.Kind == ast.RefField && c.fn != nil && c.fn.decl != nil &&
			c.fn.decl.Owner != nil && c.fn.decl.Owner.Served
	case *ast.SelectorExpr:
		return c.heldServedExpr(x.X)
	case *ast.IndexExpr:
		return c.servedRootAny(x.X)
	case *ast.SliceExpr:
		return c.servedRootAny(x.X)
	case *ast.UnaryExpr:
		return c.servedRootAny(x.X)
	}
	return false
}

func (c *checker) servedValueCopy(e ast.Expr) bool {
	if _, ok := unparen(e).(*ast.CompositeLit); ok {
		return false
	}
	t := c.exprType(e)
	if t == nil || c.isPointerType(t) {
		return false
	}
	d := servedStructDecl(t)
	return d != nil && d.Served
}

func (c *checker) servedAliasExpr(e ast.Expr) bool {
	switch x := unparen(e).(type) {
	case *ast.Ident:
		return x.Ref != nil && x.Ref.Served
	case *ast.SelectorExpr:
		return x.Ref != nil && x.Ref.Served
	case *ast.UnaryExpr:
		return x.Op == token.Amp && c.servedAliasExpr(x.X)
	}
	return false
}

func (c *checker) markServedTarget(e ast.Expr) {
	switch x := unparen(e).(type) {
	case *ast.Ident:
		if x.Ref != nil {
			x.Ref.Served = true
		}
		if sym := c.lookupLocal(x.Name); sym != nil {
			sym.served = true
		}
	case *ast.SelectorExpr:
		if x.Ref != nil {
			x.Ref.Served = true
			if x.Ref.Field != nil {
				x.Ref.Field.Served = true
			}
		}
	}
}

func (c *checker) checkVarDecl(v *ast.VarDecl) {
	if v == nil {
		return
	}
	if v.Const {
		c.p.errorf(c.fc.file.Path, v.Pos(), "const is only allowed at package level; use var")
		if v.Type != nil {
			c.p.errorf(c.fc.file.Path, v.Pos(), "const cannot have a type")
		}
	}
	var declType ast.Type
	if v.Type != nil {
		declType = c.resolveType(v.Type)
		v.Type = declType
		c.p.rejectVoid(c.fc, declType, v.Pos())
	}
	var types []ast.Type
	for _, val := range v.Values {
		c.applyNil(val, declType)
		c.checkMatchValue(val)
		types = append(types, c.resolveExpr(val, true))
		c.checkVoidValue(val)
		c.checkAssign(declType, val)
		if c.servedValueCopy(val) {
			c.p.errorf(c.fc.file.Path, val.Pos(), "a served type is not copyable; use a pointer")
		}
	}
	if len(v.Values) == 1 {
		if call, ok := orCall(v.Values[0]); ok {
			if call.Info.Results == len(v.Names) {
				types = call.Info.ResultTypes[:call.Info.Results]
			} else {
				c.p.errorf(c.fc.file.Path, call.Pos(), "wrong number of results: got %d, want %d", call.Info.Results, len(v.Names))
			}
		} else if call, ok := v.Values[0].(*ast.CallExpr); ok && call.Info != nil && len(call.Info.ResultTypes) > 0 {
			switch {
			case len(call.Info.ResultTypes) == len(v.Names):
				types = call.Info.ResultTypes
			case len(v.Names) == 1 && call.Info.Results > 1 && v.Names[0] != "_":
				c.p.errorf(c.fc.file.Path, call.Pos(), "multi-result call needs matching names")
			case len(v.Names) > 1:
				c.p.errorf(c.fc.file.Path, call.Pos(), "wrong number of results: got %d, want %d", len(call.Info.ResultTypes), len(v.Names))
			}
		}
	}
	if len(v.Values) > 1 {
		for _, val := range v.Values {
			if hasOr(val) {
				c.p.errorf(c.fc.file.Path, val.Pos(), "or needs a single initializer")
			}
		}
	}
	alias := false
	for _, val := range v.Values {
		if c.servedBody() && c.escapesServe(val) {
			c.p.errorf(c.fc.file.Path, val.Pos(), "do not alias served state inside its method; copy the values")
		}
		if c.servedAliasExpr(val) {
			alias = true
		}
	}
	for i, name := range v.Names {
		if name == "_" {
			continue
		}
		t := declType
		if t == nil && len(types) == len(v.Names) {
			t = types[i]
		}
		sym := &symbol{name: name, kind: ast.RefLocal, typ: t, goType: c.goTypeOf(t), decl: v, served: alias}
		if c.inArena > 0 && i < len(v.Values) && c.isArenaExpr(v.Values[i]) {
			sym.arena = true
		}
		if isMapType(t) && i < len(v.Values) && isNilLit(v.Values[i]) {
			sym.nilMap = true
		}
		c.declare(sym)
	}
}

func (c *checker) nilMapWrite(e ast.Expr) bool {
	idx, ok := unparen(e).(*ast.IndexExpr)
	if !ok {
		return false
	}
	id, ok := unparen(idx.X).(*ast.Ident)
	if !ok || id.Ref == nil {
		return false
	}
	if sym := c.lookupLocal(id.Name); sym != nil {
		return sym.nilMap
	}
	d, ok := id.Ref.Decl.(*ast.VarDecl)
	return ok && d != nil && isMapType(d.Type) && len(d.Values) == 1 && isNilLit(d.Values[0])
}

func (c *checker) clearNilMap(e ast.Expr) {
	id, ok := unparen(e).(*ast.Ident)
	if !ok {
		return
	}
	if sym := c.lookupLocal(id.Name); sym != nil {
		sym.nilMap = false
	}
}

func (c *checker) voidValue(e ast.Expr) bool {
	switch e := unparen(e).(type) {
	case *ast.OnErrExpr:
		return c.voidValue(e.X)
	case *ast.TryExpr:
		return c.voidValue(e.X)
	case *ast.CallExpr:
		return e.Info != nil && e.Info.Results == 0 && !e.Info.HasError
	}
	t := c.exprType(e)
	return t != nil && isVoidType(t)
}

func (c *checker) checkVoidValue(e ast.Expr) {
	if c.voidValue(e) {
		c.p.errorf(c.fc.file.Path, e.Pos(), "void is not a value")
	}
}

func (c *checker) arenaInWorker() bool {
	if !c.inPar || len(c.parArenas) == 0 {
		return false
	}
	return c.inArena <= c.parArenas[len(c.parArenas)-1]
}

func (c *checker) checkPar(body *ast.Block, _ map[string]bool) {
	c.started = true
	if c.servedBody() {
		c.p.errorf(c.fc.file.Path, body.Pos(), "par inside a served method; the lock is already held")
	}
	frame := parFrame{plain: true}
	c.parFrames = append(c.parFrames, frame)
	c.parLoopBase = append(c.parLoopBase, c.loopDepth)
	oldInPar := c.inPar
	c.inPar = true
	c.parArenas = append(c.parArenas, c.inArena)
	c.parListeners = append(c.parListeners, c.hasListener())
	c.push()
	for _, s := range body.Stmts {
		if _, ok := s.(*ast.OnErrStmt); ok {
			c.p.errorf(c.fc.file.Path, s.Pos(), "on err must be in a block, not a par branch")
		}
		c.push()
		c.checkStmt(s)
		c.pop()
	}
	c.pop()
	c.inPar = oldInPar
	c.parArenas = c.parArenas[:len(c.parArenas)-1]
	c.parLoopBase = c.parLoopBase[:len(c.parLoopBase)-1]
	c.parFrames = c.parFrames[:len(c.parFrames)-1]
	c.checkParWrites(body, frame)
}

func (c *checker) crossesPar() bool {
	return c.inPar && len(c.parLoopBase) > 0 && c.loopDepth <= c.parLoopBase[len(c.parLoopBase)-1]
}

func (f *funcInfo) node() ast.Node {
	if f.decl != nil {
		return f.decl
	}
	if f.lit != nil {
		return f.lit
	}
	if f.test != nil {
		return f.test
	}
	return nil
}

func hasOr(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if found {
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if _, ok := n.(*ast.OnErrExpr); ok {
			found = true
			return false
		}
		return true
	})
	return found
}

func orCall(e ast.Expr) (*ast.CallExpr, bool) {
	var x ast.Expr
	switch v := e.(type) {
	case *ast.OnErrExpr:
		x = v.X
	case *ast.TryExpr:
		x = v.X
	default:
		return nil, false
	}
	call, ok := x.(*ast.CallExpr)
	if !ok || call.Info == nil || !call.Info.HasError {
		return nil, false
	}
	return call, true
}

func stmtHasOr(s ast.Stmt) bool {
	found := false
	ast.Inspect(s, func(n ast.Node) bool {
		if found {
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if _, ok := n.(*ast.OnErrExpr); ok {
			found = true
			return false
		}
		return true
	})
	return found
}

func (c *checker) collectInner(b *ast.Block) map[ast.Node]bool {
	inner := make(map[ast.Node]bool)
	ast.Inspect(b, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.VarDecl:
			inner[n] = true
		case *ast.RangeClause:
			inner[n] = true
		}
		return true
	})
	return inner
}

type parWrite struct {
	id     *ast.Ident
	pos    token.Pos
	viaPtr bool
	viaMap bool
}

type parStmtEffects struct {
	writes []parWrite
	reads  map[string]bool
}

type parWalker struct {
	c     *checker
	frame parFrame
	inner map[ast.Node]bool
	alias map[string][]*ast.Ident
	eff   *parStmtEffects
}

func (c *checker) checkParWrites(b *ast.Block, frame parFrame) {
	alias := c.collectParAliases(b)
	if c.fn != nil {
		var body *ast.Block
		if c.fn.decl != nil {
			body = c.fn.decl.Body
		} else if c.fn.lit != nil {
			body = c.fn.lit.Body
		} else if c.fn.test != nil {
			body = c.fn.test.Body
		}
		if body != nil {
			for name, roots := range c.collectParAliases(body) {
				alias[name] = append(alias[name], roots...)
			}
		}
	}
	var stmts []parStmtEffects
	if frame.plain {
		stmts = make([]parStmtEffects, len(b.Stmts))
		for i, s := range b.Stmts {
			inner := c.collectInner(&ast.Block{Stmts: []ast.Stmt{s}})
			w := &parWalker{
				c:     c,
				frame: frame,
				inner: inner,
				alias: alias,
				eff:   &parStmtEffects{reads: make(map[string]bool)},
			}
			w.stmt(s)
			stmts[i] = *w.eff
		}
	} else {
		inner := c.collectInner(b)
		for d := range frame.loopDecls {
			inner[d] = true
		}
		w := &parWalker{
			c:     c,
			frame: frame,
			inner: inner,
			alias: alias,
			eff:   &parStmtEffects{reads: make(map[string]bool)},
		}
		for _, s := range b.Stmts {
			w.stmt(s)
		}
		stmts = []parStmtEffects{*w.eff}
	}
	writers := make(map[string]int)
	readers := make(map[string]map[int]bool)
	for i := range stmts {
		seen := make(map[string]bool)
		for _, w := range stmts[i].writes {
			if seen[w.id.Name] {
				continue
			}
			seen[w.id.Name] = true
			writers[w.id.Name]++
		}
		for r := range stmts[i].reads {
			if readers[r] == nil {
				readers[r] = make(map[int]bool)
			}
			readers[r][i] = true
		}
	}
	for i := range stmts {
		seen := make(map[string]bool)
		for _, w := range stmts[i].writes {
			if seen[w.id.Name] {
				continue
			}
			seen[w.id.Name] = true
			c.checkParWrite(w, i, frame, writers, readers)
		}
	}
}

func (c *checker) checkParWrite(w parWrite, stmt int, frame parFrame, writers map[string]int, readers map[string]map[int]bool) {
	if w.id.Ref == nil {
		return
	}
	switch w.id.Ref.Kind {
	case ast.RefSelf, ast.RefField:
		c.p.errorf(c.fc.file.Path, w.pos, "writing to self inside par")
	case ast.RefGlobal:
		c.p.errorf(c.fc.file.Path, w.pos, "writing to global %s inside par", w.id.Name)
	case ast.RefParam, ast.RefLocal:
		t := c.exprType(w.id)
		if c.isMapType(t) {
			c.p.errorf(c.fc.file.Path, w.pos, "writing to map %s inside par", w.id.Name)
			return
		}
		if c.isPointerType(t) {
			c.p.errorf(c.fc.file.Path, w.pos, "writing to %s captured from outside par", w.id.Name)
			return
		}
		if w.viaMap {
			c.p.errorf(c.fc.file.Path, w.pos, "writing to map %s inside par", w.id.Name)
			return
		}
		if w.viaPtr {
			c.p.errorf(c.fc.file.Path, w.pos, "writing to %s captured from outside par", w.id.Name)
			return
		}
		if frame.plain && writers[w.id.Name] == 1 {
			if !parReadByOther(w.id.Name, stmt, readers) {
				return
			}
			c.p.errorf(c.fc.file.Path, w.pos, "read by another par statement: %s", w.id.Name)
			return
		}
		c.p.errorf(c.fc.file.Path, w.pos, "writing to %s captured from outside par", w.id.Name)
	}
}

func (c *checker) isMapType(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.MapType:
		return true
	case *ast.NamedType:
		if t.Decl != nil && t.Decl.Kind == ast.TypeAlias {
			return c.isMapType(t.Decl.Target)
		}
	case *ast.GoType:
		gt := c.p.goOf[t]
		if gt != nil {
			_, ok := gt.Underlying().(*types.Map)
			return ok
		}
	}
	return false
}

func (c *checker) isInterfaceType(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.InterfaceType:
		return true
	case *ast.NamedType:
		if t.Package == "" && (t.Name == "any" || t.Name == "error") {
			return true
		}
		if t.Decl != nil {
			switch t.Decl.Kind {
			case ast.TypeInterface:
				return true
			case ast.TypeAlias:
				return c.isInterfaceType(t.Decl.Target)
			}
		}
	case *ast.GoType:
		gt := c.p.goOf[t]
		if gt != nil {
			_, ok := gt.Underlying().(*types.Interface)
			return ok
		}
	}
	if gt := c.goTypeOf(t); gt != nil {
		_, ok := gt.Underlying().(*types.Interface)
		return ok
	}
	return false
}

func (c *checker) writeMap(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.IndexExpr:
		if c.isMapType(c.exprType(x.X)) {
			return true
		}
		return c.writeMap(x.X)
	case *ast.SelectorExpr:
		if c.isMapType(c.exprType(x)) {
			return true
		}
		return c.writeMap(x.X)
	case *ast.UnaryExpr:
		return c.writeMap(x.X)
	case *ast.ParenExpr:
		return c.writeMap(x.X)
	}
	return false
}

func (c *checker) writeThroughPointer(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.SelectorExpr:
		if c.isPointerType(c.exprType(x.X)) {
			return true
		}
		return c.writeThroughPointer(x.X)
	case *ast.IndexExpr:
		if c.isPointerType(c.exprType(x.X)) {
			return true
		}
		return c.writeThroughPointer(x.X)
	case *ast.UnaryExpr:
		if x.Op == token.Star && c.isPointerType(c.exprType(x.X)) {
			return true
		}
		return c.writeThroughPointer(x.X)
	case *ast.ParenExpr:
		return c.writeThroughPointer(x.X)
	}
	return false
}

func (c *checker) isPointerType(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.PointerType:
		return true
	case *ast.NamedType:
		if t.Decl != nil && t.Decl.Kind == ast.TypeAlias {
			return c.isPointerType(t.Decl.Target)
		}
	case *ast.GoType:
		gt := c.p.goOf[t]
		if gt != nil {
			_, ok := gt.Underlying().(*types.Pointer)
			return ok
		}
	}
	return false
}

func parReadByOther(name string, stmt int, readers map[string]map[int]bool) bool {
	for i := range readers[name] {
		if i != stmt {
			return true
		}
	}
	return false
}

func (w *parWalker) stmt(s ast.Stmt) {
	if s == nil {
		return
	}
	switch s := s.(type) {
	case *ast.Block:
		for _, x := range s.Stmts {
			w.stmt(x)
		}
	case *ast.DeclStmt:
		if s.Decl != nil {
			for _, v := range s.Decl.Values {
				w.expr(v)
			}
		}
	case *ast.AssignStmt:
		for _, lhs := range s.Lhs {
			w.expr(lhs)
			if !w.frame.plain && w.c.disjoint(lhs, w.frame.iter) && !w.globalRoot(lhs) {
				continue
			}
			w.write(lhs)
		}
		for _, r := range s.Rhs {
			w.expr(r)
		}
	case *ast.ExprStmt:
		w.expr(s.X)
	case *ast.ServeStmt:
		w.expr(s.X)
	case *ast.IfStmt:
		w.expr(s.Cond)
		w.stmt(s.Then)
		w.stmt(s.Else)
	case *ast.ForStmt:
		if s.Range != nil {
			w.expr(s.Range.X)
		} else {
			w.stmt(s.Init)
			w.expr(s.Cond)
			w.stmt(s.Post)
		}
		w.stmt(s.Body)
	case *ast.ParStmt:
		w.stmt(s.Body)
	case *ast.GoCallStmt:
		w.expr(s.Call)
	case *ast.OnErrStmt:
		w.stmt(s.Body)
	case *ast.WithinStmt:
		w.expr(s.Duration)
		w.stmt(s.Body)
	case *ast.ReturnStmt:
		for _, e := range s.Results {
			w.expr(e)
		}
	case *ast.ArenaStmt:
		w.stmt(s.Body)
	case *ast.DeferStmt:
		w.expr(s.Call)
	}
}

func (w *parWalker) expr(e ast.Expr) {
	if e == nil {
		return
	}
	switch e := e.(type) {
	case *ast.Ident:
		w.read(e)
	case *ast.FuncLit:
		return
	case *ast.CallExpr:
		w.call(e)
	case *ast.BinaryExpr:
		w.expr(e.X)
		w.expr(e.Y)
	case *ast.UnaryExpr:
		w.expr(e.X)
	case *ast.ParenExpr:
		w.expr(e.X)
	case *ast.IndexExpr:
		w.expr(e.X)
		w.expr(e.Index)
	case *ast.SliceExpr:
		w.expr(e.X)
		w.expr(e.Low)
		w.expr(e.High)
	case *ast.SelectorExpr:
		w.expr(e.X)
	case *ast.CompositeLit:
		for _, en := range e.Entries {
			w.expr(en.KeyExpr)
			w.expr(en.Value)
		}
	case *ast.MatchExpr:
		w.expr(e.Subject)
		for _, cs := range e.Cases {
			if cs.Block != nil {
				w.stmt(cs.Block)
			}
			if cs.Expr != nil {
				w.expr(cs.Expr)
			}
			for _, s := range cs.Body {
				w.stmt(s)
			}
		}
	case *ast.TryExpr:
		w.expr(e.X)
	case *ast.OnErrExpr:
		w.expr(e.X)
		if e.Block != nil {
			w.stmt(e.Block)
		}
	}
}

func (w *parWalker) call(e *ast.CallExpr) {
	if sel, ok := e.Fun.(*ast.SelectorExpr); ok {
		if !parPackageRef(sel.X) {
			w.method(sel)
		} else {
			w.expr(sel.X)
		}
	} else {
		w.expr(e.Fun)
	}
	for _, a := range e.Args {
		w.expr(a)
	}
	if id, ok := e.Fun.(*ast.Ident); ok {
		w.mutatingBuiltin(e, id)
	}
}

func parPackageRef(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Ref != nil && id.Ref.Kind == ast.RefPackage
}

func (w *parWalker) method(sel *ast.SelectorExpr) {
	w.expr(sel.X)
	if w.pureMethod(sel) || w.slotMethod(sel.X) || w.servedReceiver(sel.X) {
		return
	}
	w.writeMethod(sel.X)
}

func (w *parWalker) servedReceiver(e ast.Expr) bool {
	root := rootIdent(e)
	return root != nil && root.Ref != nil && root.Ref.Served
}

func (w *parWalker) writeMethod(e ast.Expr) {
	root := rootIdent(e)
	if root == nil {
		return
	}
	viaPtr := w.c.isPointerType(w.c.exprType(e)) || w.c.isInterfaceType(w.c.exprType(e))
	for _, t := range w.targets(root) {
		if w.isInner(t) {
			continue
		}
		w.eff.writes = append(w.eff.writes, parWrite{id: t, pos: e.Pos(), viaPtr: viaPtr})
	}
}

func (w *parWalker) isInner(id *ast.Ident) bool {
	return id != nil && id.Ref != nil && id.Ref.Decl != nil && w.inner[id.Ref.Decl]
}

func (w *parWalker) pureMethod(sel *ast.SelectorExpr) bool {
	return sel.Ref != nil && sel.Ref.Func != nil && pureParMethod(sel.Ref.Func)
}

func (w *parWalker) slotMethod(recv ast.Expr) bool {
	if w.frame.plain || !w.c.disjoint(recv, w.frame.iter) || w.globalRoot(recv) {
		return false
	}
	return w.c.structValue(w.c.exprType(recv))
}

func (w *parWalker) globalRoot(e ast.Expr) bool {
	root := rootIdent(e)
	return root != nil && root.Ref != nil && root.Ref.Kind == ast.RefGlobal
}

func (w *parWalker) mutatingBuiltin(e *ast.CallExpr, id *ast.Ident) {
	switch id.Name {
	case "delete", "copy":
	default:
		return
	}
	if len(e.Args) == 0 {
		return
	}
	if !w.frame.plain && w.c.disjoint(e.Args[0], w.frame.iter) && !w.globalRoot(e.Args[0]) {
		return
	}
	w.write(e.Args[0])
}

func (w *parWalker) read(id *ast.Ident) {
	if id.Name == "_" || id.Ref == nil || !parVarRef(id.Ref.Kind) {
		return
	}
	for _, t := range w.targets(id) {
		if w.isInner(t) || t.Ref == nil || !parVarRef(t.Ref.Kind) {
			continue
		}
		w.eff.reads[t.Name] = true
	}
}

func (w *parWalker) write(e ast.Expr) {
	root := rootIdent(e)
	if root == nil {
		return
	}
	viaPtr := w.c.writeThroughPointer(e)
	viaMap := w.c.writeMap(e)
	for _, t := range w.targets(root) {
		if w.isInner(t) {
			continue
		}
		w.eff.writes = append(w.eff.writes, parWrite{id: t, pos: e.Pos(), viaPtr: viaPtr, viaMap: viaMap})
	}
}

func (w *parWalker) targets(root *ast.Ident) []*ast.Ident {
	seen := make(map[string]bool)
	var out []*ast.Ident
	var walk func(*ast.Ident)
	walk = func(id *ast.Ident) {
		if id == nil || seen[id.Name] {
			return
		}
		seen[id.Name] = true
		if next := w.alias[id.Name]; len(next) > 0 {
			for _, t := range next {
				walk(t)
			}
			return
		}
		out = append(out, id)
	}
	walk(root)
	return out
}

func parVarRef(k ast.RefKind) bool {
	switch k {
	case ast.RefLocal, ast.RefParam, ast.RefGlobal, ast.RefSelf, ast.RefField:
		return true
	}
	return false
}

func (c *checker) collectParAliases(b *ast.Block) map[string][]*ast.Ident {
	edges := make(map[string][]*ast.Ident)
	add := func(name string, v ast.Expr) {
		if name == "_" || !c.refLike(c.exprType(v)) {
			return
		}
		for {
			if p, ok := v.(*ast.ParenExpr); ok {
				v = p.X
				continue
			}
			if u, ok := v.(*ast.UnaryExpr); ok && u.Op == token.Amp {
				v = u.X
				continue
			}
			break
		}
		root := rootIdent(v)
		if root == nil || root.Ref == nil || !parVarRef(root.Ref.Kind) {
			return
		}
		edges[name] = append(edges[name], root)
	}
	var walkExpr func(ast.Expr)
	var walkStmt func(ast.Stmt)
	walkExpr = func(e ast.Expr) {
		switch e := e.(type) {
		case *ast.MatchExpr:
			for _, cs := range e.Cases {
				if cs.Block != nil {
					walkStmt(cs.Block)
				}
				if cs.Expr != nil {
					walkExpr(cs.Expr)
				}
				for _, s := range cs.Body {
					walkStmt(s)
				}
			}
		case *ast.TryExpr:
			walkExpr(e.X)
		case *ast.OnErrExpr:
			walkExpr(e.X)
			if e.Block != nil {
				walkStmt(e.Block)
			}
		}
	}
	walkStmt = func(s ast.Stmt) {
		switch s := s.(type) {
		case *ast.Block:
			for _, x := range s.Stmts {
				walkStmt(x)
			}
		case *ast.DeclStmt:
			if s.Decl != nil {
				for i, name := range s.Decl.Names {
					if i < len(s.Decl.Values) {
						add(name, s.Decl.Values[i])
					}
				}
			}
		case *ast.AssignStmt:
			if s.Op != token.Assign || len(s.Lhs) != len(s.Rhs) {
				return
			}
			for i, lhs := range s.Lhs {
				if id, ok := lhs.(*ast.Ident); ok {
					add(id.Name, s.Rhs[i])
				}
			}
		case *ast.ExprStmt:
			walkExpr(s.X)
		case *ast.IfStmt:
			walkStmt(s.Then)
			if s.Else != nil {
				walkStmt(s.Else)
			}
		case *ast.ForStmt:
			if s.Range == nil {
				walkStmt(s.Init)
				walkStmt(s.Post)
			}
			walkStmt(s.Body)
		case *ast.ParStmt:
			walkStmt(s.Body)
		case *ast.WithinStmt:
			walkStmt(s.Body)
		}
	}
	for _, s := range b.Stmts {
		walkStmt(s)
	}
	return edges
}

func (c *checker) refLike(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.PointerType, *ast.SliceType, *ast.MapType, *ast.InterfaceType:
		return true
	case *ast.NamedType:
		if t.Decl != nil {
			switch t.Decl.Kind {
			case ast.TypeAlias:
				return c.refLike(t.Decl.Target)
			case ast.TypeInterface:
				return true
			}
		}
		return t.Package == "" && t.Name == "any"
	case *ast.GoType:
		gt := c.p.goOf[t]
		if gt != nil {
			switch gt.Underlying().(type) {
			case *types.Pointer, *types.Slice, *types.Map, *types.Interface:
				return true
			}
		}
	}
	return false
}

func pureParMethod(fn *ast.FuncDecl) bool {
	if fn == nil || fn.Body == nil {
		return false
	}
	pure := true
	var walkStmt func(ast.Stmt)
	var walkExpr func(ast.Expr)
	walkStmt = func(s ast.Stmt) {
		if !pure || s == nil {
			return
		}
		switch s := s.(type) {
		case *ast.Block:
			for _, x := range s.Stmts {
				walkStmt(x)
			}
		case *ast.DeclStmt:
			if s.Decl != nil {
				for _, v := range s.Decl.Values {
					walkExpr(v)
				}
			}
		case *ast.AssignStmt:
			pure = false
		case *ast.ExprStmt:
			walkExpr(s.X)
		case *ast.IfStmt:
			walkExpr(s.Cond)
			walkStmt(s.Then)
			walkStmt(s.Else)
		case *ast.ForStmt:
			if s.Range != nil {
				walkExpr(s.Range.X)
			} else {
				walkStmt(s.Init)
				walkExpr(s.Cond)
				walkStmt(s.Post)
			}
			walkStmt(s.Body)
		case *ast.ReturnStmt:
			for _, e := range s.Results {
				walkExpr(e)
			}
		case *ast.DeferStmt, *ast.ParStmt, *ast.WithinStmt:
			pure = false
		}
	}
	walkExpr = func(e ast.Expr) {
		if !pure || e == nil {
			return
		}
		switch e := e.(type) {
		case *ast.FuncLit:
			pure = false
		case *ast.CallExpr:
			if !pureParCall(e) {
				pure = false
				return
			}
			for _, a := range e.Args {
				walkExpr(a)
			}
		case *ast.BinaryExpr:
			walkExpr(e.X)
			walkExpr(e.Y)
		case *ast.UnaryExpr:
			walkExpr(e.X)
		case *ast.ParenExpr:
			walkExpr(e.X)
		case *ast.IndexExpr:
			walkExpr(e.X)
			walkExpr(e.Index)
		case *ast.SliceExpr:
			walkExpr(e.X)
			walkExpr(e.Low)
			walkExpr(e.High)
		case *ast.SelectorExpr:
			walkExpr(e.X)
		case *ast.CompositeLit:
			for _, en := range e.Entries {
				walkExpr(en.KeyExpr)
				walkExpr(en.Value)
			}
		case *ast.MatchExpr:
			walkExpr(e.Subject)
			for _, cs := range e.Cases {
				if cs.Block != nil {
					walkStmt(cs.Block)
				}
				if cs.Expr != nil {
					walkExpr(cs.Expr)
				}
				for _, s := range cs.Body {
					walkStmt(s)
				}
			}
		case *ast.TryExpr:
			walkExpr(e.X)
		case *ast.OnErrExpr:
			walkExpr(e.X)
			if e.Block != nil {
				walkStmt(e.Block)
			}
		}
	}
	walkStmt(fn.Body)
	return pure
}

func pureParCall(e *ast.CallExpr) bool {
	id, ok := e.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	switch id.Name {
	case "len", "cap", "min", "max", "has", "assert", "assert_eq", "panic", "str", "runes", "bytes":
		return true
	}
	return IsBuiltinType(id.Name) || (id.Ref != nil && id.Ref.Kind == ast.RefType)
}

func rootIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.SliceExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		case *ast.UnaryExpr:
			if x.Op == token.Star {
				e = x.X
				continue
			}
			return nil
		default:
			return nil
		}
	}
}

func (c *checker) disjoint(e ast.Expr, iter map[string]bool) bool {
	if !c.sliceIndexChain(e) {
		return false
	}
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && iter[id.Name] {
			found = true
			return false
		}
		return true
	})
	return found
}

func (c *checker) sliceIndexChain(e ast.Expr) bool {
	saw := false
	for {
		switch x := e.(type) {
		case *ast.IndexExpr:
			saw = true
			if !c.sliceOrArray(c.exprType(x.X)) {
				return false
			}
			e = x.X
		case *ast.SelectorExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		case *ast.UnaryExpr:
			if x.Op == token.Star {
				e = x.X
				continue
			}
			return false
		case *ast.Ident:
			return saw
		default:
			return false
		}
	}
}

func (c *checker) sliceOrArray(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.SliceType, *ast.ArrayType:
		return true
	case *ast.NamedType:
		if t.Decl != nil && t.Decl.Kind == ast.TypeAlias {
			return c.sliceOrArray(t.Decl.Target)
		}
	case *ast.GoType:
		gt := c.p.goOf[t]
		if gt != nil {
			switch gt.Underlying().(type) {
			case *types.Slice, *types.Array:
				return true
			}
		}
	}
	return false
}

func (c *checker) structValue(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.StructType:
		return true
	case *ast.NamedType:
		if t.Decl != nil {
			switch t.Decl.Kind {
			case ast.TypeAlias:
				return c.structValue(t.Decl.Target)
			case ast.TypeStruct:
				return true
			}
		}
	case *ast.GoType:
		gt := c.p.goOf[t]
		if gt != nil {
			_, ok := gt.Underlying().(*types.Struct)
			return ok
		}
	}
	return false
}
