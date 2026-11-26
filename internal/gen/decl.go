package gen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bongdo/ink/internal/ast"
)

func (g *generator) decl(w *writer, d ast.Decl) {
	switch d := d.(type) {
	case *ast.TypeDecl:
		g.typeDecl(w, d)
	case *ast.FuncDecl:
		g.funcDecl(w, d, nil)
	case *ast.VarDecl:
		g.packageVar(w, d)
	}
}

type servedArg struct {
	index int
	name  string
	decl  *ast.TypeDecl
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

func servedArgOf(fd *ast.FuncDecl) (servedArg, bool) {
	if fd == nil || len(fd.Params) == 0 {
		return servedArg{}, false
	}
	safe := safeParams(fd.Params)
	for i, p := range fd.Params {
		if _, ok := p.Type.(*ast.PointerType); !ok {
			continue
		}
		if d := servedPtrDecl(p.Type); d != nil {
			return servedArg{index: i, name: safe[i].Name, decl: d}, true
		}
	}
	return servedArg{}, false
}

func (g *generator) funcDecl(w *writer, fd *ast.FuncDecl, recv *ast.TypeDecl) {
	if recv != nil && recv.Served && !w.rawSelf && fd.Body != nil && !fd.NeedsCtx && len(recv.TypeParams) == 0 {
		g.servedMethod(w, fd, recv)
		return
	}
	if recv == nil && fd.Body != nil && !fd.NeedsCtx && len(fd.TypeParams) == 0 {
		if sa, ok := servedArgOf(fd); ok {
			g.servedFree(w, fd, sa)
			return
		}
	}
	name := fd.GoName
	if name == "" {
		name = fd.Name
	}
	main := recv == nil && fd.Name == "main" && w.f.pkg.IsMain
	if main {
		name = "_s_main"
	}
	if fd.NeedsCtx && !main {
		g.ctxFuncDecl(w, fd, recv, name)
		return
	}
	w.directive(fd)
	params := g.paramsStr(w.f, fd.Params, nil, fd.NeedsCtx)
	res := g.resultStr(w.f, fd.Results, fd.Err)
	sig := "func "
	if recv != nil {
		selfName := methodSelfName(fd, recv)
		w.self = selfName
		sig += "(" + selfName + " *" + typeNameOf(recv) + g.typeParamsUse(recv.TypeParams) + ") "
	}
	sig += name
	if recv == nil {
		sig += g.typeParamsDecl(fd.TypeParams)
	}
	sig += "(" + params + ")" + res
	w.line(sig + " {")
	w.indent++
	g.funcBody(w, fd.Body, fd.Results, fd.Err, fd.NeedsCtx)
	w.indent--
	w.line("}")
	w.line("")
	if recv == nil && g.into[fd] != "" {
		g.intoFunc(w, fd)
	}
	if main {
		g.mainWrapper(w, fd)
	}
}

func argNames(params []*ast.Param) []string {
	names := make([]string, len(params))
	for i, p := range params {
		names[i] = p.Name
	}
	return names
}

func (g *generator) servedMethod(w *writer, fd *ast.FuncDecl, recv *ast.TypeDecl) {
	name := fd.GoName
	if name == "" {
		name = fd.Name
	}
	raw := "_s_raw_" + name
	self := methodSelfName(fd, recv)
	params := safeParams(fd.Params)
	pstr := g.paramsStr(w.f, params, nil, false)
	res := g.resultStr(w.f, fd.Results, fd.Err)
	args := strings.Join(argNames(params), ", ")
	call := self + "." + raw + "(" + args + ")"

	w.directive(fd)
	w.line("func (" + self + " *" + typeNameOf(recv) + ") " + name + "(" + pstr + ")" + res + " {")
	w.indent++
	w.line("if " + self + "._s_served {")
	w.indent++
	w.line(self + "._s_mu.Lock()")
	w.line("defer " + self + "._s_mu.Unlock()")
	w.indent--
	w.line("}")
	if len(fd.Results) > 0 || fd.Err {
		w.line("return " + call)
	} else {
		w.line(call)
	}
	w.indent--
	w.line("}")
	w.line("")

	w.directive(fd)
	w.line("func (" + self + " *" + typeNameOf(recv) + ") " + raw + "(" + pstr + ")" + res + " {")
	w.indent++
	w.self = self
	w.rawSelf = true
	w.held = map[string]*ast.TypeDecl{self: recv}
	g.funcBody(w, fd.Body, fd.Results, fd.Err, false)
	w.indent--
	w.line("}")
	w.line("")
}

func (g *generator) servedFree(w *writer, fd *ast.FuncDecl, sa servedArg) {
	name := fd.GoName
	if name == "" {
		name = fd.Name
	}
	raw := "_s_held_" + name
	params := safeParams(fd.Params)
	pstr := g.paramsStr(w.f, params, nil, false)
	res := g.resultStr(w.f, fd.Results, fd.Err)
	args := strings.Join(argNames(params), ", ")
	arg := params[sa.index].Name
	call := raw + "(" + args + ")"

	w.directive(fd)
	w.line("func " + name + "(" + pstr + ")" + res + " {")
	w.indent++
	w.line("if " + arg + "._s_served {")
	w.indent++
	w.line(arg + "._s_mu.Lock()")
	w.line("defer " + arg + "._s_mu.Unlock()")
	w.indent--
	w.line("}")
	if len(fd.Results) > 0 || fd.Err {
		w.line("return " + call)
	} else {
		w.line(call)
	}
	w.indent--
	w.line("}")
	w.line("")

	w.directive(fd)
	w.line("func " + raw + "(" + pstr + ")" + res + " {")
	w.indent++
	w.held = map[string]*ast.TypeDecl{arg: sa.decl}
	g.funcBody(w, fd.Body, fd.Results, fd.Err, false)
	w.indent--
	w.line("}")
	w.line("")
}

func (g *generator) intoName(fd *ast.FuncDecl) string {
	if fd == nil || fd.Body == nil || fd.Err || fd.NeedsCtx || len(fd.Results) != 1 || len(fd.TypeParams) > 0 {
		return ""
	}
	if _, ok := servedArgOf(fd); ok {
		return ""
	}
	if fd.Name == "main" || strings.HasPrefix(fd.Name, "_s_") || !g.arrayType(fd.Results[0]) || bodyHasDefer(fd.Body) {
		return ""
	}
	ok := true
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		if !ok {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.OnErrStmt:
			ok = false
			return false
		case *ast.ReturnStmt:
			if len(n.Results) != 1 {
				ok = false
				return false
			}
			if isProp(n.Results[0]) {
				ok = false
				return false
			}
			if c, isCall := unparen(n.Results[0]).(*ast.CallExpr); isCall && callResults(c) != 1 {
				ok = false
				return false
			}
		}
		return true
	})
	if !ok {
		return ""
	}
	return "_s_into_" + fd.Name
}

func (g *generator) arrayType(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.ArrayType:
		return true
	case *ast.NamedType:
		if t.Decl != nil && t.Decl.Target != nil {
			return g.arrayType(t.Decl.Target)
		}
	}
	return false
}

func (g *generator) intoFunc(w *writer, fd *ast.FuncDecl) {
	name := g.into[fd]
	params := g.paramsStr(w.f, fd.Params, nil, false)
	dst := w.next("dst")
	sig := "func " + name + "(" + dst + " *" + w.typeStr(fd.Results[0])
	if params != "" {
		sig += ", " + params
	}
	w.line(sig + ") {")
	w.indent++
	sub := w.sub()
	sub.indent = w.indent
	sub.into = "*" + dst
	sub.results = fd.Results
	sub.err = false
	sub.deferStack = false
	sub.line("{")
	sub.indent++
	if fd.Body != nil {
		sub.genStmts(fd.Body.Stmts)
	}
	sub.indent--
	sub.line("}")
	if !blockTerminates(fd.Body) {
		sub.line(`panic("unreachable")`)
	}
	w.buf.WriteString(sub.buf.String())
	w.indent--
	w.line("}")
	w.line("")
}

func (g *generator) ctxFuncDecl(w *writer, fd *ast.FuncDecl, recv *ast.TypeDecl, name string) {
	params := safeParams(fd.Params)
	selfName := ""
	if recv != nil {
		selfName = methodSelfName(fd, recv)
	}
	res := g.resultStr(w.f, fd.Results, fd.Err)
	ctxName := ctxFuncName(fd)

	w.directive(fd)
	sig := "func "
	if recv != nil {
		sig += "(" + selfName + " *" + typeNameOf(recv) + g.typeParamsUse(recv.TypeParams) + ") "
	}
	sig += name
	if recv == nil {
		sig += g.typeParamsDecl(fd.TypeParams)
	}
	sig += "(" + g.paramsStr(w.f, params, nil, false) + ")" + res
	w.line(sig + " {")
	w.indent++
	callArgs := make([]string, 0, len(params)+2)
	if recv != nil {
		callArgs = append(callArgs, selfName)
	}
	callArgs = append(callArgs, w.f.background())
	for _, p := range params {
		callArgs = append(callArgs, p.Name)
	}
	call := ctxName
	if recv == nil && len(fd.TypeParams) > 0 {
		call += g.typeParamsUse(fd.TypeParams)
	} else if recv != nil && len(recv.TypeParams) > 0 {
		call += g.typeParamsUse(recv.TypeParams)
	}
	call += "(" + strings.Join(callArgs, ", ") + ")"
	if len(fd.Results) == 0 && !fd.Err {
		w.line(call)
	} else {
		w.line("return " + call)
	}
	w.indent--
	w.line("}")
	w.line("")

	w.directive(fd)
	sig = "func " + ctxName
	if recv == nil {
		sig += g.typeParamsDecl(fd.TypeParams)
	} else {
		sig += g.typeParamsDecl(recv.TypeParams)
	}
	sig += "("
	var parts []string
	if recv != nil {
		parts = append(parts, selfName+" *"+typeNameOf(recv)+g.typeParamsUse(recv.TypeParams))
	}
	parts = append(parts, "_s_ctx "+w.f.std("context")+".Context")
	if len(params) > 0 {
		parts = append(parts, g.paramsStr(w.f, params, nil, false))
	}
	sig += strings.Join(parts, ", ") + ")" + res
	w.line(sig + " {")
	w.indent++
	if recv != nil {
		w.self = selfName
	}
	g.funcBody(w, fd.Body, fd.Results, fd.Err, true)
	w.indent--
	w.line("}")
	w.line("")
}

func methodSelfName(fd *ast.FuncDecl, recv *ast.TypeDecl) string {
	for _, p := range fd.Params {
		if p.Name == "self" {
			return "_s_self"
		}
	}
	for _, tp := range recv.TypeParams {
		if tp.Name == "self" {
			return "_s_self"
		}
	}
	if hasLocalSelf(fd.Body) {
		return "_s_self"
	}
	return "self"
}

func typeNameOf(recv *ast.TypeDecl) string {
	if recv.GoName != "" {
		return recv.GoName
	}
	return recv.Name
}

func safeParams(params []*ast.Param) []*ast.Param {
	out := make([]*ast.Param, 0, len(params))
	for i, p := range params {
		name := p.Name
		if name == "" || name == "_" {
			name = fmt.Sprintf("_s_a%d", i)
		}
		out = append(out, &ast.Param{Span: p.Span, Name: name, Type: p.Type})
	}
	return out
}

func ctxFuncName(fd *ast.FuncDecl) string {
	name := fd.GoName
	if name == "" {
		name = fd.Name
	}
	if fd.Owner != nil {
		return "_s_" + typeNameOf(fd.Owner) + "_" + name + "_ctx"
	}
	return "_s_" + name + "_ctx"
}

func (g *generator) funcBody(w *writer, body *ast.Block, results []ast.Type, err bool, needsCtx bool) {
	w.results = results
	w.err = err
	if needsCtx {
		w.ctx = "_s_ctx"
		w.f.needPkg("context")
	}
	if bodyHasDefer(body) {
		w.deferStack = true
		w.f.needPkg("rt")
		w.line("var _s_d _s_rt.DeferStack")
		w.line("defer _s_d.RunTo(0)")
	}
	var stmts []ast.Stmt
	if body != nil {
		stmts = body.Stmts
	}
	w.line("{")
	w.indent++
	w.bodyContents(body, hasDirectDefer(stmts))
	w.indent--
	w.line("}")
	terminates := blockTerminates(body)
	if err && len(results) == 0 && !terminates {
		w.line("return nil")
	}
	if len(results) > 0 && !terminates {
		// shortcut: sema and gen each decide termination; unify them when the two ever disagree.
		w.line(`panic("unreachable")`)
	}
}

func blockTerminates(b *ast.Block) bool {
	if b == nil || len(b.Stmts) == 0 {
		return false
	}
	return stmtTerminates(b.Stmts[len(b.Stmts)-1])
}

func stmtTerminates(s ast.Stmt) bool {
	switch s := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.ArenaStmt:
		return blockTerminates(s.Body)
	case *ast.Block:
		return blockTerminates(s)
	case *ast.IfStmt:
		return s.Else != nil && stmtTerminates(s.Then) && stmtTerminates(s.Else)
	case *ast.ExprStmt:
		if m, ok := s.X.(*ast.MatchExpr); ok {
			return matchTerminates(m)
		}
		return panicStatement(s.X)
	case *ast.ForStmt:
		return !s.Par && s.Range == nil && s.Cond == nil && !loopBreaks(s.Body)
	case *ast.WithinStmt:
		return blockTerminates(s.Body)
	}
	return false
}

func matchTerminates(m *ast.MatchExpr) bool {
	if m == nil || len(m.Cases) == 0 || !switchHasDefault(m) {
		return false
	}
	for _, cs := range m.Cases {
		if !caseTerminates(cs) {
			return false
		}
	}
	return true
}

func caseTerminates(cs *ast.Case) bool {
	if cs == nil {
		return false
	}
	if cs.Block != nil {
		return blockTerminates(cs.Block)
	}
	if len(cs.Body) > 0 {
		return stmtTerminates(cs.Body[len(cs.Body)-1])
	}
	return cs.Expr != nil && panicStatement(cs.Expr)
}

func switchHasDefault(m *ast.MatchExpr) bool {
	return caseDefault(m) || enumMatch(m)
}

func caseDefault(m *ast.MatchExpr) bool {
	value := !m.TypeSwitch && !enumMatch(m)
	for _, cs := range m.Cases {
		if cs.Default {
			return true
		}
		if cs.Pat == nil || cs.Pat.Wildcard {
			return true
		}
		if value && cs.Pat.Literal == nil && cs.Pat.Name == "" {
			return true
		}
	}
	return false
}

func enumMatch(m *ast.MatchExpr) bool {
	if n, ok := m.SubjectType.(*ast.NamedType); ok && n.Decl != nil && n.Decl.Kind == ast.TypeEnum {
		return true
	}
	for _, cs := range m.Cases {
		if cs.Pat != nil && cs.Pat.Variant != nil {
			return true
		}
	}
	return false
}

func panicStatement(e ast.Expr) bool {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			break
		}
		e = p.X
	}
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	name, ok := builtinName(c)
	return ok && name == "panic"
}

func loopBreaks(b *ast.Block) bool {
	if b == nil {
		return false
	}
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncLit, *ast.ForStmt:
			return false
		case *ast.BreakStmt:
			found = true
			return false
		}
		return !found
	})
	return found
}

func (g *generator) mainWrapper(w *writer, fd *ast.FuncDecl) {
	w.directive(fd)
	w.line("func main() {")
	w.indent++
	args := ""
	if fd.NeedsCtx {
		w.line("_s_ctx := " + w.f.background())
		args = "_s_ctx"
	}
	call := "_s_main(" + args + ")"
	if fd.Err {
		errPkg := w.f.std("fmt")
		osPkg := w.f.std("os")
		w.linef("if _s_err := %s; _s_err != nil {", call)
		w.indent++
		w.line(errPkg + ".Fprintln(" + osPkg + ".Stderr, _s_err)")
		w.line(osPkg + ".Exit(1)")
		w.indent--
		w.line("}")
	} else {
		w.line(call)
	}
	w.indent--
	w.line("}")
	w.line("")
}

func (g *generator) typeDecl(w *writer, d *ast.TypeDecl) {
	name := d.GoName
	if name == "" {
		name = d.Name
	}
	switch d.Kind {
	case ast.TypeAlias:
		w.directive(d)
		w.line("type " + name + g.typeParamsDecl(d.TypeParams) + " = " + g.typeStr(w.f, d.Target))
		w.line("")
	case ast.TypeStruct:
		g.structDecl(w, d, name)
	case ast.TypeEnum:
		g.enumDecl(w, d, name)
	case ast.TypeInterface:
		w.directive(d)
		if len(d.IFace) == 0 {
			w.line("type " + name + g.typeParamsDecl(d.TypeParams) + " interface{}")
			w.line("")
			return
		}
		w.line("type " + name + g.typeParamsDecl(d.TypeParams) + " interface {")
		w.indent++
		for _, m := range d.IFace {
			w.line(g.ifaceMethodStr(w.f, m))
		}
		w.indent--
		w.line("}")
		w.line("")
	}
}

func (g *generator) structDecl(w *writer, d *ast.TypeDecl, name string) {
	w.directive(d)
	wire := wireNames(d)
	if len(d.Fields) == 0 && !d.Served {
		w.line("type " + name + g.typeParamsDecl(d.TypeParams) + " struct{}")
	} else {
		w.line("type " + name + g.typeParamsDecl(d.TypeParams) + " struct {")
		w.indent++
		for _, fld := range d.Fields {
			tag := ""
			if wire != nil && fld.Name != "_" {
				ext, ok := wire[fld.Name]
				if !ok {
					ext = "-"
				}
				tag = " `json:" + strconv.Quote(ext) + "`"
			}
			w.line(fld.Name + " " + g.typeStr(w.f, fld.Type) + tag)
		}
		if d.Served {
			w.line("_s_mu " + w.f.std("sync") + ".Mutex")
			w.line("_s_served bool")
		}
		w.indent--
		w.line("}")
	}
	w.line("")
	for _, m := range d.Methods {
		mw := newWriter(w.f, backgroundCtx)
		g.funcDecl(mw, m, d)
		w.buf.WriteString(mw.buf.String())
	}
	for _, c := range d.Contracts {
		if c.Name == "wire" {
			continue
		}
		w.directive(c)
		w.line("func (self *" + name + ") " + c.Name + "() map[string]any {")
		w.indent++
		w.line("return map[string]any{")
		w.indent++
		for _, cf := range c.Fields {
			w.line(strconv.Quote(cf.External) + ": self." + cf.Field + ",")
		}
		w.indent--
		w.line("}")
		w.indent--
		w.line("}")
		w.line("")
	}
}

func wireNames(d *ast.TypeDecl) map[string]string {
	for _, c := range d.Contracts {
		if c.Name != "wire" {
			continue
		}
		names := make(map[string]string, len(c.Fields))
		for _, f := range c.Fields {
			names[f.Field] = f.External
		}
		return names
	}
	return nil
}

func (g *generator) enumDecl(w *writer, d *ast.TypeDecl, name string) {
	marker := "_s_is" + name
	w.directive(d)
	w.line("type " + name + " interface{ " + marker + "() }")
	w.line("")
	for _, v := range d.Variants {
		vt := g.variantName(v)
		w.directive(v)
		if len(v.Fields) == 0 {
			w.line("type " + vt + " struct{}")
		} else {
			w.line("type " + vt + " struct {")
			w.indent++
			for _, fld := range v.Fields {
				w.line(fld.Name + " " + g.typeStr(w.f, fld.Type))
			}
			w.indent--
			w.line("}")
		}
		w.directive(v)
		w.line("func (*" + vt + ") " + marker + "() {}")
		w.line("")
	}
}

func (g *generator) packageVar(w *writer, d *ast.VarDecl) {
	for _, v := range d.Values {
		if isProp(v) {
			g.errAt(v, "propagation is not supported in a package-level declaration")
			return
		}
	}
	if len(d.Values) == 0 {
		return
	}
	values := make([]string, 0, len(d.Values))
	for _, v := range d.Values {
		values = append(values, w.expr(v))
	}
	prefix := "var"
	if d.Const {
		prefix = "const"
	}
	typePart := ""
	if d.Type != nil {
		typePart = " " + w.typeStr(d.Type)
	}
	w.directive(d)
	w.line(prefix + " " + strings.Join(d.Names, ", ") + typePart + " = " + strings.Join(values, ", "))
	w.line("")
}

func (g *generator) testDecl(w *writer, t *ast.TestDecl) {
	id := t.GoID
	if id == "" {
		id = "Unnamed"
	}
	wrapper := "Test" + id
	helper := "_s_test" + id
	needsCtx := g.testNeedsCtx(w.f, t)
	w.directive(t)
	w.line("func " + wrapper + "(t *" + w.f.std("testing") + ".T) {")
	w.indent++
	w.line("defer func() {")
	w.indent++
	w.line("if r := recover(); r != nil {")
	w.indent++
	w.directive(t)
	w.line("t.Fatalf(\"%v\", r)")
	w.indent--
	w.line("}")
	w.indent--
	w.line("}()")
	args := ""
	if needsCtx {
		args = w.f.background()
	}
	w.linef("if _s_err := %s(%s); _s_err != nil {", helper, args)
	w.indent++
	w.line("t.Fatal(_s_err)")
	w.indent--
	w.line("}")
	w.indent--
	w.line("}")
	w.line("")
	w.directive(t)
	sig := "func " + helper + "("
	if needsCtx {
		sig += "_s_ctx " + w.f.std("context") + ".Context"
	}
	sig += ") error"
	w.line(sig + " {")
	w.indent++
	w.err = true
	w.results = nil
	if needsCtx {
		w.ctx = "_s_ctx"
	}
	if bodyHasDefer(t.Body) {
		w.deferStack = true
		w.f.needPkg("rt")
		w.line("var _s_d _s_rt.DeferStack")
		w.line("defer _s_d.RunTo(0)")
	}
	var stmts []ast.Stmt
	if t.Body != nil {
		stmts = t.Body.Stmts
	}
	w.line("{")
	w.indent++
	w.bodyContents(t.Body, hasDirectDefer(stmts))
	w.indent--
	w.line("}")
	w.line("return nil")
	w.indent--
	w.line("}")
	w.line("")
}

func (g *generator) testNeedsCtx(f *fileGen, t *ast.TestDecl) bool {
	if t.Body == nil {
		return false
	}
	probe := newWriter(f, "")
	found := false
	ast.Inspect(t.Body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.ParStmt:
			found = true
		case *ast.WithinStmt:
			found = true
		case *ast.ForStmt:
			if n.Par {
				found = true
			}
		case *ast.CallExpr:
			if probe.callNeedsCtx(n) {
				found = true
			}
		}
		return !found
	})
	return found
}

func hasLocalSelf(b *ast.Block) bool {
	if b == nil {
		return false
	}
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.VarDecl:
			for _, name := range n.Names {
				if name == "self" {
					found = true
				}
			}
		case *ast.RangeClause:
			for _, name := range n.Names {
				if name == "self" {
					found = true
				}
			}
		case *ast.Param:
			if n.Name == "self" {
				found = true
			}
		case *ast.Pattern:
			for _, name := range n.Fields {
				if name == "self" {
					found = true
				}
			}
		case *ast.Ident:
			if n.Name == "self" && n.Ref != nil && n.Ref.Kind == ast.RefLocal {
				found = true
			}
		}
		return !found
	})
	return found
}
