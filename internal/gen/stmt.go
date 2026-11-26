package gen

import (
	"go/types"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bongdo/ink/internal/ast"
	"github.com/bongdo/ink/internal/token"
)

func (w *writer) bodyContents(b *ast.Block, mark bool) {
	if b == nil {
		return
	}
	markName := ""
	if mark && w.deferStack && !blockTerminates(b) {
		markName = w.next("mark")
		w.linef("%s := _s_d.Mark()", markName)
	}
	savedOnErr := w.onErr
	if len(b.Stmts) > 0 {
		if oe, ok := b.Stmts[0].(*ast.OnErrStmt); ok {
			w.onErr = w.onErrClosure(oe.Body)
			w.genStmts(b.Stmts[1:])
			w.onErr = savedOnErr
			if markName != "" {
				w.linef("_s_d.RunTo(%s)", markName)
			}
			return
		}
	}
	w.genStmts(b.Stmts)
	if markName != "" {
		w.linef("_s_d.RunTo(%s)", markName)
	}
}

func (w *writer) genStmts(stmts []ast.Stmt) {
	for _, s := range stmts {
		if w.skip[s] {
			continue
		}
		w.stmt(s)
	}
}

func (w *writer) stmt(s ast.Stmt) {
	if s == nil {
		return
	}
	if _, ok := s.(*ast.EmptyStmt); ok {
		return
	}
	w.directive(s)
	switch s := s.(type) {
	case *ast.Block:
		w.writeIndent()
		w.buf.WriteString("{\n")
		w.indent++
		w.bodyContents(s, hasDirectDefer(s.Stmts))
		w.indent--
		w.writeIndent()
		w.buf.WriteString("}\n")
	case *ast.DeclStmt:
		w.declStmt(s.Decl)
	case *ast.AssignStmt:
		w.assignStmt(s)
	case *ast.ExprStmt:
		if _, ok := w.propStmt(s.X, false); ok {
			return
		}
		if call, ok := unparen(s.X).(*ast.CallExpr); ok && call.Info != nil && call.Info.Builtin == "check" {
			w.checkStmt(call)
			return
		}
		if m, ok := s.X.(*ast.MatchExpr); ok {
			w.matchDepth++
			w.matchSwitch(m, false)
			w.matchDepth--
			return
		}
		w.line(w.expr(s.X))
	case *ast.IfStmt:
		w.ifClause(s, true)
	case *ast.ForStmt:
		w.forStmt(s)
	case *ast.ReturnStmt:
		w.returnStmt(s)
	case *ast.ArenaStmt:
		w.arenaStmt(s)
	case *ast.BreakStmt:
		w.breakStmt("break")
	case *ast.ContinueStmt:
		w.breakStmt("continue")
	case *ast.DeferStmt:
		w.deferStmt(s)
	case *ast.ParStmt:
		w.parStmt(s)
	case *ast.GoCallStmt:
		w.goCallStmt(s)
	case *ast.ServeStmt:
		w.serveStmt(s)
	case *ast.OnErrStmt:
	case *ast.WithinStmt:
		w.withinStmt(s)
	}
}

func (w *writer) ifClause(s *ast.IfStmt, top bool) {
	if top {
		w.writeIndent()
	}
	w.buf.WriteString("if ")
	w.buf.WriteString(w.expr(s.Cond))
	w.buf.WriteString(" {\n")
	w.indent++
	w.bodyContents(s.Then, hasDirectDefer(s.Then.Stmts))
	w.indent--
	w.writeIndent()
	w.buf.WriteString("}")
	switch e := s.Else.(type) {
	case nil:
		w.buf.WriteString("\n")
	case *ast.Block:
		w.buf.WriteString(" else {\n")
		w.indent++
		w.bodyContents(e, hasDirectDefer(e.Stmts))
		w.indent--
		w.writeIndent()
		w.buf.WriteString("}\n")
	case *ast.IfStmt:
		w.buf.WriteString(" else ")
		w.ifClause(e, false)
	default:
		w.buf.WriteString("\n")
	}
}

func (w *writer) declStmt(d *ast.VarDecl) {
	if d == nil {
		return
	}
	prefix := "var"
	if d.Const {
		prefix = "const"
	}
	typePart := ""
	if d.Type != nil {
		typePart = " " + w.typeStr(d.Type)
	}
	if len(d.Values) == 1 {
		if values, isOr := w.propStmt(d.Values[0], true); isOr {
			if len(values) == 0 {
				return
			}
			names := strings.Join(d.Names, ", ")
			if allBlank(d.Names) && d.Type == nil {
				w.linef("%s = %s", blanks(len(values)), strings.Join(values, ", "))
			} else {
				w.linef("%s %s%s = %s", prefix, names, typePart, strings.Join(values, ", "))
			}
			return
		}
	}
	if len(d.Values) == 0 {
		return
	}
	values := make([]string, 0, len(d.Values))
	for _, v := range d.Values {
		if isProp(v) {
			w.f.g.errAt(v, "or is not supported in this declaration")
			continue
		}
		values = append(values, w.expr(v))
	}
	names := strings.Join(d.Names, ", ")
	if allBlank(d.Names) && d.Type == nil {
		if len(d.Values) == 1 {
			if call, ok := unparen(d.Values[0]).(*ast.CallExpr); ok {
				w.linef("%s = %s", blanks(discardCount(call)), values[0])
				return
			}
		}
		if len(values) == 1 {
			w.linef("_ = %s", values[0])
		} else {
			w.linef("_, _ = %s", strings.Join(values, ", "))
		}
		return
	}
	w.linef("%s %s%s = %s", prefix, names, typePart, strings.Join(values, ", "))
}

func discardCount(call *ast.CallExpr) int {
	if call == nil || call.Info == nil {
		return 1
	}
	n := call.Info.Results
	if call.Info.HasError {
		n++
	}
	if n < 1 {
		n = 1
	}
	return n
}

func allBlank(names []string) bool {
	for _, n := range names {
		if n != "_" {
			return false
		}
	}
	return true
}

func (w *writer) assignStmt(s *ast.AssignStmt) {
	if s.Op == token.Assign && len(s.Lhs) == 1 && len(s.Rhs) == 1 {
		if call, ok := unparen(s.Rhs[0]).(*ast.CallExpr); ok && w.intoCall(call, s.Lhs[0]) {
			return
		}
	}
	lhs := make([]string, 0, len(s.Lhs))
	for _, e := range s.Lhs {
		lhs = append(lhs, w.expr(e))
	}
	if len(s.Rhs) == 1 {
		if values, ok := w.propStmt(s.Rhs[0], true); ok {
			if len(values) == 0 {
				return
			}
			w.linef("%s = %s", strings.Join(lhs, ", "), strings.Join(values, ", "))
			return
		}
	}
	if allBlank(lhs) && len(s.Rhs) == 1 {
		if call, ok := unparen(s.Rhs[0]).(*ast.CallExpr); ok {
			rhs := w.expr(s.Rhs[0])
			w.linef("%s = %s", blanks(discardCount(call)), rhs)
			return
		}
	}
	rhs := make([]string, 0, len(s.Rhs))
	for _, e := range s.Rhs {
		if isProp(e) {
			w.f.g.errAt(e, "or is not supported in this assignment")
			continue
		}
		rhs = append(rhs, w.expr(e))
	}
	op := "="
	if s.Op != token.Assign {
		op = s.Op.String()
	}
	w.linef("%s %s %s", strings.Join(lhs, ", "), op, strings.Join(rhs, ", "))
}

func (w *writer) intoCall(call *ast.CallExpr, lhs ast.Expr) bool {
	if call.Info == nil || call.Info.Kind != ast.CallFunc || call.Info.HasError || call.Info.Results != 1 || len(call.TypeArgs) > 0 {
		return false
	}
	name := w.f.g.into[call.Info.Fn]
	if name == "" || !w.addressable(lhs) || !pureOperand(lhs) || call.Ellipsis || w.f.pkg.Dir == "" {
		return false
	}
	if filepath.Dir(w.f.g.sourcePath(call.Info.Fn)) != w.f.pkg.Dir {
		return false
	}
	for _, a := range call.Args {
		if !pureOperand(a) {
			return false
		}
	}
	args := make([]string, 0, len(call.Args)+1)
	args = append(args, "&"+w.expr(lhs))
	for _, a := range call.Args {
		args = append(args, w.expr(a))
	}
	w.linef("%s(%s)", name, strings.Join(args, ", "))
	return true
}

func pureOperand(e ast.Expr) bool {
	switch e := unparen(e).(type) {
	case *ast.BasicLit, *ast.Ident:
		return true
	case *ast.IndexExpr:
		return pureOperand(e.X) && pureOperand(e.Index)
	case *ast.SelectorExpr:
		return pureOperand(e.X)
	case *ast.UnaryExpr:
		switch e.Op {
		case token.Plus, token.Minus, token.Caret, token.Bang:
			return pureOperand(e.X)
		}
	case *ast.BinaryExpr:
		switch e.Op {
		case token.Plus, token.Minus, token.Star, token.Slash, token.Percent,
			token.Amp, token.Pipe, token.Caret, token.Shl, token.Shr,
			token.Eq, token.Ne, token.Lt, token.Le, token.Gt, token.Ge,
			token.AndAnd, token.OrOr:
			return pureOperand(e.X) && pureOperand(e.Y)
		}
	case *ast.CallExpr:
		if e.Info != nil && e.Info.Kind == ast.CallConversion {
			for _, a := range e.Args {
				if !pureOperand(a) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func (w *writer) addressable(e ast.Expr) bool {
	switch x := unparen(e).(type) {
	case *ast.Ident:
		return x.Name != "_"
	case *ast.IndexExpr:
		return !w.namedGo(w.f.g.p.ExprTypes[x.X]) && !w.mapType(w.f.g.p.ExprTypes[x.X]) && w.addressable(x.X)
	case *ast.SelectorExpr:
		return w.addressable(x.X)
	}
	return false
}

func (w *writer) namedGo(t ast.Type) bool {
	n, ok := t.(*ast.NamedType)
	return ok && n.Decl == nil
}

func (w *writer) mapType(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.MapType:
		return true
	case *ast.NamedType:
		if t.Decl != nil && t.Decl.Target != nil {
			return w.mapType(t.Decl.Target)
		}
	}
	return false
}

func (w *writer) returnStmt(s *ast.ReturnStmt) {
	if len(s.Results) == 1 {
		if isProp(s.Results[0]) {
			results := 1
			if call := propCall(s.Results[0]); call != nil && call.Info != nil {
				results = call.Info.Results
			}
			values, _ := w.propStmt(s.Results[0], results > 0)
			w.ret(values)
			return
		}
	}
	vals := make([]string, 0, len(s.Results))
	for _, e := range s.Results {
		vals = append(vals, w.expr(e))
	}
	if len(vals) == 1 && w.err && w.isErrorExpr(s.Results[0]) {
		w.returnErr(vals[0])
		return
	}
	if w.into != "" && len(vals) == 1 {
		if w.deferStack {
			w.line("_s_d.RunTo(0)")
		}
		w.linef("%s = %s", w.into, vals[0])
		w.line("return")
		return
	}
	if len(vals) == 1 {
		if call, ok := s.Results[0].(*ast.CallExpr); ok {
			if (callResults(call) > 1 || isFallibleCall(call)) && callResults(call) == len(w.results) {
				w.retRaw(vals[0])
				return
			}
		}
	}
	w.ret(vals)
}

func (w *writer) isErrorExpr(e ast.Expr) bool {
	return isErrorType(w.f.g.p.ExprTypes[e]) || isErrorCall(e)
}

func isErrorType(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.NamedType:
		return t.Package == "" && t.Name == "error"
	case *ast.GoType:
		return t.Text == "error"
	}
	return false
}

func isErrorCall(e ast.Expr) bool {
	c, ok := e.(*ast.CallExpr)
	return ok && c.Info != nil && c.Info.HasError && c.Info.Results == 0
}

func isFallibleCall(c *ast.CallExpr) bool {
	return c.Info != nil && c.Info.HasError
}

func callResults(c *ast.CallExpr) int {
	if c.Info == nil {
		return 0
	}
	return c.Info.Results
}

func (w *writer) ret(vals []string) {
	if w.deferStack {
		w.line("_s_d.RunTo(0)")
	}
	if len(vals) == 1 && vals[0] == "" {
		vals = nil
	}
	if w.err {
		n := len(w.results)
		if len(vals) == 0 {
			if n == 0 {
				w.line("return nil")
				return
			}
			w.line("return " + w.zeroResults() + ", nil")
			return
		}
		if len(vals) == n {
			w.line("return " + strings.Join(vals, ", ") + ", nil")
			return
		}
		w.line("return " + strings.Join(vals, ", "))
		return
	}
	if len(vals) == 0 {
		w.line("return")
		return
	}
	w.line("return " + strings.Join(vals, ", "))
}

func (w *writer) zeroResults() string {
	parts := make([]string, 0, len(w.results))
	for _, t := range w.results {
		parts = append(parts, "*new("+w.typeStr(t)+")")
	}
	return strings.Join(parts, ", ")
}

func (w *writer) retRaw(value string) {
	if w.deferStack {
		w.line("_s_d.RunTo(0)")
	}
	w.line("return " + value)
}

func (w *writer) returnErr(errName string) {
	if w.deferStack {
		w.line("_s_d.RunTo(0)")
	}
	if !w.err {
		w.linef("panic(%s)", errName)
		return
	}
	if len(w.results) == 0 {
		w.linef("return %s", errName)
		return
	}
	w.linef("return %s, %s", w.zeroResults(), errName)
}

func (w *writer) breakStmt(kind string) {
	if w.deferStack && len(w.loops) > 0 {
		if mark := w.loops[len(w.loops)-1]; mark != "" {
			w.linef("_s_d.RunTo(%s)", mark)
		}
	}
	if w.matchDepth > 0 && len(w.loopLabels) > 0 {
		if label := w.loopLabels[len(w.loopLabels)-1]; label != "" {
			kind += " " + label
		}
	}
	w.line(kind)
}

func propCall(e ast.Expr) *ast.CallExpr {
	switch e := e.(type) {
	case *ast.TryExpr:
		if c, ok := unparen(e.X).(*ast.CallExpr); ok {
			return c
		}
	case *ast.OnErrExpr:
		if c, ok := unparen(e.X).(*ast.CallExpr); ok {
			return c
		}
	}
	return nil
}

func isProp(e ast.Expr) bool {
	switch e.(type) {
	case *ast.TryExpr, *ast.OnErrExpr:
		return true
	}
	return false
}

func (w *writer) checkStmt(call *ast.CallExpr) {
	cond := "false"
	msg := "\"\""
	if len(call.Args) > 0 {
		cond = w.expr(call.Args[0])
	}
	if len(call.Args) > 1 {
		msg = w.expr(call.Args[1])
	}
	w.f.needPkg("errors")
	w.linef("if !(%s) {", cond)
	w.indent++
	w.propagateParError(w.f.std("errors") + ".New(" + msg + ")")
	w.indent--
	w.line("}")
}

func (w *writer) evalCall(x ast.Expr, wantValue bool) ([]string, string) {
	results := 1
	var callStr string
	if call, ok := x.(*ast.CallExpr); ok {
		if call.Info != nil {
			results = call.Info.Results
		}
		callStr = w.call(call)
	} else {
		callStr = w.expr(x)
	}
	errName := w.next("err")
	var values []string
	if results > 0 && wantValue {
		values = make([]string, 0, results)
		for i := 0; i < results; i++ {
			values = append(values, w.next("v"))
		}
	}
	switch {
	case results > 0 && wantValue:
		w.linef("%s, %s := %s", strings.Join(values, ", "), errName, callStr)
	case results > 0:
		w.linef("%s, %s := %s", blanks(results), errName, callStr)
	default:
		w.linef("%s := %s", errName, callStr)
	}
	return values, errName
}

func blanks(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "_"
	}
	return strings.Join(parts, ", ")
}

func (w *writer) propStmt(e ast.Expr, wantValue bool) ([]string, bool) {
	switch e := e.(type) {
	case *ast.TryExpr:
		values, errName := w.evalCall(e.X, wantValue)
		w.linef("if %s != nil {", errName)
		w.indent++
		w.propagateParError(errName)
		w.indent--
		w.line("}")
		return values, true
	case *ast.OnErrExpr:
		values, errName := w.evalCall(e.X, wantValue)
		w.linef("if %s != nil {", errName)
		w.indent++
		w.linef("err := %s", errName)
		w.line("_ = err")
		w.line("{")
		w.indent++
		if e.Block != nil {
			w.bodyContents(e.Block, hasDirectDefer(e.Block.Stmts))
		}
		w.indent--
		w.line("}")
		w.indent--
		w.line("}")
		return values, true
	}
	return nil, false
}

func (w *writer) tryExpr(e *ast.TryExpr) string {
	values, _ := w.propStmt(e, true)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (w *writer) onErrExpr(e *ast.OnErrExpr) string {
	values, _ := w.propStmt(e, true)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (w *writer) onErrClosure(body *ast.Block) string {
	name := w.next("onerr")
	w.linef("%s := func(err error) error {", name)
	w.indent++
	savedResults, savedErr := w.results, w.err
	savedPar, savedDefer := w.par, w.deferStack
	w.results, w.err = nil, true
	w.par, w.deferStack = nil, false
	if body != nil {
		w.bodyContents(body, false)
	}
	w.results, w.err = savedResults, savedErr
	w.par, w.deferStack = savedPar, savedDefer
	w.line("return nil")
	w.indent--
	w.line("}")
	w.linef("_ = %s", name)
	return name
}

func (w *writer) serveStmt(s *ast.ServeStmt) {}

func (w *writer) goCallStmt(s *ast.GoCallStmt) {
	expr := s.Call
	var handler *ast.Block
	if or, ok := unparen(s.Call).(*ast.OnErrExpr); ok {
		handler = or.Block
		expr = or.X
	}
	call, ok := unparen(expr).(*ast.CallExpr)
	if !ok {
		return
	}
	var args []string
	for _, a := range call.Args {
		if isConstExpr(a) {
			args = append(args, w.expr(a))
			continue
		}
		name := w.next("a")
		w.linef("%s := %s", name, w.expr(a))
		args = append(args, name)
	}
	if call.Ellipsis && len(args) > 0 {
		args[len(args)-1] += "..."
	}
	fun := ""
	switch f := unparen(call.Fun).(type) {
	case *ast.SelectorExpr:
		if isPackageExpr(f.X) {
			fun = w.expr(f)
			break
		}
		recv := w.next("r")
		rt := w.f.g.p.ExprTypes[f.X]
		if isPointerType(rt) || isInterfaceType(w, rt) || !isAddressable(f.X) {
			w.linef("%s := %s", recv, w.expr(f.X))
		} else {
			w.linef("%s := &%s", recv, w.expr(f.X))
		}
		fun = recv + "." + f.Name
	case *ast.Ident:
		if f.Ref != nil && f.Ref.Func == nil && f.Ref.Kind != ast.RefPackage {
			name := w.next("f")
			w.linef("%s := %s", name, w.expr(f))
			fun = name
		} else {
			fun = w.expr(f)
		}
	default:
		fun = w.expr(call.Fun)
	}
	if len(call.TypeArgs) > 0 {
		parts := make([]string, len(call.TypeArgs))
		for i, t := range call.TypeArgs {
			parts[i] = w.typeStr(t)
		}
		fun += "[" + strings.Join(parts, ", ") + "]"
	}
	callStr := fun + "(" + strings.Join(args, ", ") + ")"
	w.line("go func() {")
	w.indent++
	if call.Info != nil && call.Info.HasError {
		w.linef("if err := %s; err != nil {", callStr)
		w.indent++
		if handler != nil {
			savedResults, savedErr := w.results, w.err
			savedPar, savedDefer, savedOnErr := w.par, w.deferStack, w.onErr
			w.results, w.err = nil, false
			w.par, w.deferStack, w.onErr = nil, false, ""
			w.bodyContents(handler, false)
			w.results, w.err = savedResults, savedErr
			w.par, w.deferStack, w.onErr = savedPar, savedDefer, savedOnErr
		} else {
			w.line("panic(err)")
		}
		w.indent--
		w.line("}")
	} else {
		w.line(callStr)
	}
	w.indent--
	w.line("}()")
}

func isConstExpr(e ast.Expr) bool {
	switch e := unparen(e).(type) {
	case *ast.BasicLit, *ast.NilLit:
		return true
	case *ast.UnaryExpr:
		return isConstExpr(e.X)
	}
	return false
}

func isInterfaceType(w *writer, t ast.Type) bool {
	if n, ok := t.(*ast.NamedType); ok {
		if n.Package == "" && (n.Name == "any" || n.Name == "error") {
			return true
		}
		if n.Decl != nil && n.Decl.Kind == ast.TypeInterface {
			return true
		}
	}
	if gt := w.f.g.p.GoTypeOf(t); gt != nil {
		_, ok := gt.Underlying().(*types.Interface)
		return ok
	}
	return false
}

func isAddressable(e ast.Expr) bool {
	switch e := unparen(e).(type) {
	case *ast.Ident:
		return e.Ref == nil || e.Ref.Kind != ast.RefPackage
	case *ast.SelectorExpr:
		return isAddressable(e.X)
	case *ast.IndexExpr:
		return true
	case *ast.UnaryExpr:
		return e.Op == token.Star
	}
	return false
}

func isPackageExpr(e ast.Expr) bool {
	id, ok := unparen(e).(*ast.Ident)
	return ok && id.Ref != nil && id.Ref.Kind == ast.RefPackage
}

func (w *writer) propagateParError(errName string) {
	if w.par != nil {
		if w.deferStack {
			w.line("_s_d.RunTo(0)")
		}
		if w.par.pe != "" {
			w.linef("%s.Add(%s, %s)", w.par.pe, w.par.ord, errName)
		}
		if w.par.chunk != "" {
			w.linef("continue %s", w.par.chunk)
		} else {
			w.line("return")
		}
		return
	}
	w.returnErr(errName)
}

func (w *writer) fuseCopy(b *ast.Block) {
	if b == nil {
		return
	}
	for i := 0; i+1 < len(b.Stmts); i++ {
		par, ok := b.Stmts[i].(*ast.ForStmt)
		if !ok || !par.Par || chunkLoopOf(par) == nil {
			continue
		}
		cp, ok := b.Stmts[i+1].(*ast.ForStmt)
		if !ok || cp.Par || cp.Range != nil || len(cp.Body.Stmts) != 1 {
			continue
		}
		if !loopPure(cp) || !w.sameLoop(par, cp) || !w.rowCopy(par, cp) {
			continue
		}
		if w.skip == nil {
			w.skip = make(map[ast.Stmt]bool)
			w.fuse = make(map[*ast.ForStmt]*ast.ForStmt)
		}
		w.skip[cp] = true
		w.fuse[par] = cp
	}
}

func (w *writer) sameLoop(a, b *ast.ForStmt) bool {
	return loopVar(a) != "" && loopVar(a) == loopVar(b) && w.loopHead(a) == w.loopHead(b)
}

func (w *writer) loopHead(s *ast.ForStmt) string {
	sub := w.sub()
	return sub.initString(s.Init) + "|" + sub.expr(s.Cond) + "|" + sub.postString(s.Post)
}

func loopPure(s *ast.ForStmt) bool {
	d, ok := s.Init.(*ast.DeclStmt)
	if !ok || d.Decl == nil || len(d.Decl.Names) != 1 || len(d.Decl.Values) != 1 || !isIntLit(d.Decl.Values[0]) {
		return false
	}
	if s.Cond == nil || !sideEffectFree(s.Cond) {
		return false
	}
	post, ok := s.Post.(*ast.AssignStmt)
	if !ok || post.Op != token.PlusAssign || len(post.Lhs) != 1 || len(post.Rhs) != 1 {
		return false
	}
	lhs, ok := unparen(post.Lhs[0]).(*ast.Ident)
	if !ok || lhs.Name != d.Decl.Names[0] {
		return false
	}
	return isPosIntLit(post.Rhs[0])
}

func loopVar(s *ast.ForStmt) string {
	d, ok := s.Init.(*ast.DeclStmt)
	if !ok || d.Decl == nil || len(d.Decl.Names) != 1 {
		return ""
	}
	return d.Decl.Names[0]
}

func (w *writer) rowCopy(par, cp *ast.ForStmt) bool {
	es, ok := cp.Body.Stmts[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := unparen(es.X).(*ast.CallExpr)
	if !ok || call.Info == nil || call.Info.Builtin != "copy" || len(call.Args) != 2 {
		return false
	}
	dst, ok := unparen(call.Args[0]).(*ast.SliceExpr)
	if !ok || dst.High != nil {
		return false
	}
	src, ok := unparen(call.Args[1]).(*ast.SliceExpr)
	if !ok || src.High != nil {
		return false
	}
	y := loopVar(par)
	rows := rootIdent(src.X)
	px := rootIdent(dst.X)
	if y == "" || rows == "" || px == "" || rows == px {
		return false
	}
	if !rowIndex(src.X, rows, y) || rootIdent(dst.X) != px {
		return false
	}
	stride, ok := unparen(dst.Low).(*ast.BinaryExpr)
	if !ok || stride.Op != token.Star {
		return false
	}
	if id, ok := unparen(stride.X).(*ast.Ident); !ok || id.Name != y {
		return false
	}
	if hasFuncLit(par.Body) || shadows(par.Body, rows, px, y) || !writesRow(par.Body, rows, y) || !onlyRowsY(par.Body, rows, y) || usesIdent(par.Body, px) || !constExpr(stride.Y) {
		return false
	}
	n := w.rowLen(w.f.g.p.ExprTypes[src.X])
	if n == nil || w.expr(n) != w.expr(stride.Y) {
		return false
	}
	return true
}

func constExpr(e ast.Expr) bool {
	switch x := unparen(e).(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		return x.Ref != nil && x.Ref.Kind == ast.RefConst
	}
	return false
}

func rootIdent(e ast.Expr) string {
	for {
		switch x := unparen(e).(type) {
		case *ast.Ident:
			return x.Name
		case *ast.IndexExpr:
			e = x.X
		case *ast.SelectorExpr:
			e = x.X
		case *ast.SliceExpr:
			e = x.X
		default:
			return ""
		}
	}
}

func rowIndex(e ast.Expr, rows, y string) bool {
	for {
		ix, ok := unparen(e).(*ast.IndexExpr)
		if !ok {
			return false
		}
		if id, ok := unparen(ix.X).(*ast.Ident); ok && id.Name == rows {
			yi, ok := unparen(ix.Index).(*ast.Ident)
			return ok && yi.Name == y
		}
		e = ix.X
	}
}

// shortcut: any closure disables row-copy fusion; widen only with symbol-aware effects.
func hasFuncLit(b *ast.Block) bool {
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		if found {
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
			found = true
			return false
		}
		return true
	})
	return found
}

func shadows(b *ast.Block, names ...string) bool {
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.DeclStmt:
			if n.Decl != nil {
				for _, name := range n.Decl.Names {
					for _, want := range names {
						if name == want {
							found = true
							return false
						}
					}
				}
			}
		}
		return true
	})
	return found
}

func writesRow(b *ast.Block, rows, y string) bool {
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		if found {
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if a, ok := n.(*ast.AssignStmt); ok {
			for _, l := range a.Lhs {
				if rowIndex(l, rows, y) {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}

// shortcut: any rows use other than rows[y] disables fusion, including len(rows).
func onlyRowsY(b *ast.Block, rows, y string) bool {
	allowed := make(map[*ast.Ident]bool)
	ast.Inspect(b, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		ix, ok := n.(*ast.IndexExpr)
		if !ok {
			return true
		}
		id, ok := unparen(ix.X).(*ast.Ident)
		if !ok || id.Name != rows {
			return true
		}
		if yi, ok := unparen(ix.Index).(*ast.Ident); ok && yi.Name == y {
			allowed[id] = true
		}
		return true
	})
	ok := true
	ast.Inspect(b, func(n ast.Node) bool {
		if !ok {
			return false
		}
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
		if id, isID := n.(*ast.Ident); isID && id.Name == rows && !allowed[id] {
			ok = false
			return false
		}
		return true
	})
	return ok
}

func usesIdent(b *ast.Block, name string) bool {
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = true
			return false
		}
		return !found
	})
	return found
}

func (w *writer) rowLen(t ast.Type) ast.Expr {
	switch t := t.(type) {
	case *ast.ArrayType:
		return t.Len
	case *ast.NamedType:
		if t.Decl != nil && t.Decl.Target != nil {
			return w.rowLen(t.Decl.Target)
		}
	}
	return nil
}

func (w *writer) arenaStmt(s *ast.ArenaStmt) {
	savedSkip, savedFuse := w.skip, w.fuse
	defer func() { w.skip, w.fuse = savedSkip, savedFuse }()
	w.fuseCopy(s.Body)
	sites := arenaSites(s.Body)
	saved := w.scratch
	var local []string
	for _, call := range sites {
		if _, ok := w.scratch[call]; ok {
			continue
		}
		if w.scratch == nil {
			w.scratch = make(map[*ast.CallExpr]string)
		}
		name := w.next("scr")
		w.linef("var %s %s.Scratch[%s]", name, w.f.std("rt"), w.typeStr(call.TypeArgs[0]))
		w.scratch[call] = name
		local = append(local, name)
	}
	mark := ""
	if len(sites) > 0 && !blockTerminates(s.Body) {
		mark = w.next("mark")
		w.linef("%s := _s_d.Mark()", mark)
	}
	for _, call := range sites {
		scratch := w.scratch[call]
		batch := w.next("b")
		w.linef("%s := %s.Begin()", batch, scratch)
		w.linef("_s_d.Push(func() { %s.End(%s) })", scratch, batch)
		w.scratch[call] = batch
	}
	w.writeIndent()
	w.buf.WriteString("{\n")
	w.indent++
	w.bodyContents(s.Body, hasDirectDefer(s.Body.Stmts))
	w.indent--
	w.writeIndent()
	w.buf.WriteString("}\n")
	if mark != "" {
		w.linef("_s_d.RunTo(%s)", mark)
	}
	for _, name := range local {
		w.linef("%s.Release()", name)
	}
	w.scratch = saved
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

func arenaSites(body *ast.Block) []*ast.CallExpr {
	if body == nil {
		return nil
	}
	var out []*ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ArenaStmt:
			return false
		case *ast.CallExpr:
			if n.Info != nil && n.Info.Kind == ast.CallBuiltin && len(n.TypeArgs) == 1 &&
				(n.Info.Builtin == "new" || n.Info.Builtin == "make") {
				out = append(out, n)
			}
		}
		return true
	})
	return out
}

func (w *writer) loopScratch(body *ast.Block) []string {
	if body == nil {
		return nil
	}
	if w.scratch == nil {
		w.scratch = make(map[*ast.CallExpr]string)
	}
	byType := make(map[string]string)
	var names []string
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.ForStmt, *ast.ParStmt:
			return false
		case *ast.ArenaStmt:
			for _, call := range arenaSites(n.Body) {
				typ := w.typeStr(call.TypeArgs[0])
				name, ok := byType[typ]
				if !ok {
					name = w.next("scr")
					byType[typ] = name
					names = append(names, name)
					w.linef("var %s %s.Scratch[%s]", name, w.f.std("rt"), typ)
				}
				w.scratch[call] = name
			}
			return false
		}
		return true
	})
	return names
}

func (w *writer) deferStmt(s *ast.DeferStmt) {
	c := s.Call
	if c == nil {
		return
	}
	w.f.needPkg("rt")
	w.deferStack = true
	var args []string
	var callStr string
	if isConversion(c) {
		for _, a := range c.Args {
			name := w.next("x")
			w.linef("%s := %s", name, w.expr(a))
			args = append(args, name)
		}
		callStr = w.typeStr(w.conversionTarget(c)) + "(" + strings.Join(args, ", ") + ")"
	} else if name, ok := builtinName(c); ok {
		for _, a := range c.Args {
			tmp := w.next("x")
			w.linef("%s := %s", tmp, w.expr(a))
			args = append(args, tmp)
		}
		if c.Ellipsis && len(args) > 0 {
			args[len(args)-1] += "..."
		}
		callStr = w.builtinCall(name, c, args)
	} else {
		fun, prefix, _ := w.callTarget(c)
		if len(c.TypeArgs) > 0 {
			parts := make([]string, 0, len(c.TypeArgs))
			for _, t := range c.TypeArgs {
				parts = append(parts, w.typeStr(t))
			}
			fun += "[" + strings.Join(parts, ", ") + "]"
		}
		funName := w.next("f")
		w.linef("%s := %s", funName, fun)
		callStr = funName
		var pre []string
		for _, p := range prefix {
			tmp := w.next("x")
			w.linef("%s := %s", tmp, p)
			pre = append(pre, tmp)
		}
		args = append(args, pre...)
		for _, a := range c.Args {
			tmp := w.next("x")
			w.linef("%s := %s", tmp, w.expr(a))
			args = append(args, tmp)
		}
		if c.Ellipsis && len(args) > 0 {
			args[len(args)-1] += "..."
		}
		callStr += "(" + strings.Join(args, ", ") + ")"
	}
	w.linef("_s_d.Push(func() { %s })", callStr)
}

func bodyPushesDefer(b *ast.Block) bool {
	if b == nil {
		return false
	}
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n.(type) {
		case *ast.FuncLit, *ast.ForStmt, *ast.ParStmt:
			return false
		case *ast.DeferStmt, *ast.WithinStmt, *ast.ArenaStmt:
			found = true
			return false
		}
		return true
	})
	return found
}

func loopJumps(b *ast.Block) bool {
	if b == nil {
		return false
	}
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncLit, *ast.ForStmt:
			return false
		case *ast.BreakStmt, *ast.ContinueStmt:
			found = true
			return false
		}
		return !found
	})
	return found
}

func loopNeedsLabel(b *ast.Block) bool {
	if b == nil {
		return false
	}
	for _, s := range b.Stmts {
		if matchJump(s, false) {
			return true
		}
	}
	return false
}

func matchJump(s ast.Stmt, inMatch bool) bool {
	if s == nil {
		return false
	}
	switch s := s.(type) {
	case *ast.BreakStmt, *ast.ContinueStmt:
		return inMatch
	case *ast.Block:
		if s == nil {
			return false
		}
		for _, x := range s.Stmts {
			if matchJump(x, inMatch) {
				return true
			}
		}
	case *ast.IfStmt:
		return matchJumpExpr(s.Cond, inMatch) || matchJump(s.Then, inMatch) || matchJump(s.Else, inMatch)
	case *ast.ForStmt:
		return false
	case *ast.ExprStmt:
		return matchJumpExpr(s.X, inMatch)
	case *ast.AssignStmt:
		for _, e := range s.Lhs {
			if matchJumpExpr(e, inMatch) {
				return true
			}
		}
		for _, e := range s.Rhs {
			if matchJumpExpr(e, inMatch) {
				return true
			}
		}
	case *ast.DeclStmt:
		if s.Decl != nil {
			for _, e := range s.Decl.Values {
				if matchJumpExpr(e, inMatch) {
					return true
				}
			}
		}
	case *ast.ReturnStmt:
		for _, e := range s.Results {
			if matchJumpExpr(e, inMatch) {
				return true
			}
		}
	case *ast.ArenaStmt:
		return matchJump(s.Body, inMatch)
	case *ast.WithinStmt:
		return matchJump(s.Body, inMatch)
	}
	return false
}

func matchJumpExpr(e ast.Expr, inMatch bool) bool {
	switch e := e.(type) {
	case nil:
		return false
	case *ast.FuncLit:
		return false
	case *ast.MatchExpr:
		for _, cs := range e.Cases {
			if cs.Block != nil && matchJump(cs.Block, true) {
				return true
			}
			for _, s := range cs.Body {
				if matchJump(s, true) {
					return true
				}
			}
		}
		return false
	case *ast.OnErrExpr:
		if e.Block != nil && matchJump(e.Block, inMatch) {
			return true
		}
		return matchJumpExpr(e.X, inMatch)
	case *ast.TryExpr:
		return matchJumpExpr(e.X, inMatch)
	case *ast.CallExpr:
		for _, a := range e.Args {
			if matchJumpExpr(a, inMatch) {
				return true
			}
		}
		return matchJumpExpr(e.Fun, inMatch)
	case *ast.BinaryExpr:
		return matchJumpExpr(e.X, inMatch) || matchJumpExpr(e.Y, inMatch)
	case *ast.UnaryExpr:
		return matchJumpExpr(e.X, inMatch)
	case *ast.ParenExpr:
		return matchJumpExpr(e.X, inMatch)
	case *ast.IndexExpr:
		return matchJumpExpr(e.X, inMatch) || matchJumpExpr(e.Index, inMatch)
	case *ast.SliceExpr:
		return matchJumpExpr(e.X, inMatch) || matchJumpExpr(e.Low, inMatch) || matchJumpExpr(e.High, inMatch)
	case *ast.SelectorExpr:
		return matchJumpExpr(e.X, inMatch)
	case *ast.CompositeLit:
		for _, en := range e.Entries {
			if matchJumpExpr(en.KeyExpr, inMatch) || matchJumpExpr(en.Value, inMatch) {
				return true
			}
		}
	}
	return false
}

func typedVarInit(s ast.Stmt) *ast.VarDecl {
	d, ok := s.(*ast.DeclStmt)
	if !ok || d.Decl == nil || d.Decl.Type == nil {
		return nil
	}
	return d.Decl
}

func (w *writer) forStmt(s *ast.ForStmt) {
	if s.Par {
		w.parForStmt(s)
		return
	}
	typed := typedVarInit(s.Init)
	if typed != nil {
		w.line("{")
		w.indent++
		w.declStmt(typed)
	}
	label := ""
	if loopNeedsLabel(s.Body) {
		label = w.next("loop")
		w.line(label + ":")
	}
	w.loopLabels = append(w.loopLabels, label)
	scratch := w.loopScratch(s.Body)
	push := bodyPushesDefer(s.Body)
	if s.Range != nil {
		w.rangeHead(s.Range)
	} else {
		init := ""
		if typed == nil {
			init = w.initString(s.Init)
		}
		cond := ""
		if s.Cond != nil {
			cond = w.expr(s.Cond)
		}
		post := w.postString(s.Post)
		switch {
		case init == "" && post == "" && cond != "":
			w.linef("for %s {", cond)
		case init == "" && post == "":
			w.line("for {")
		default:
			w.linef("for %s; %s; %s {", init, cond, post)
		}
	}
	w.indent++
	mark := ""
	terminates := blockTerminates(s.Body)
	if w.deferStack && push && (!terminates || loopJumps(s.Body)) {
		mark = w.next("mark")
		w.linef("%s := _s_d.Mark()", mark)
	}
	w.loops = append(w.loops, mark)
	if s.Body != nil {
		w.genStmts(s.Body.Stmts)
	}
	if mark != "" && !terminates {
		w.linef("_s_d.RunTo(%s)", mark)
	}
	w.loops = w.loops[:len(w.loops)-1]
	w.loopLabels = w.loopLabels[:len(w.loopLabels)-1]
	w.indent--
	w.line("}")
	if typed != nil {
		w.indent--
		w.line("}")
	}
	for _, name := range scratch {
		w.linef("%s.Release()", name)
	}
}

func (w *writer) rangeHead(r *ast.RangeClause) {
	if r == nil {
		w.line("for {")
		return
	}
	x := w.expr(r.X)
	if allBlank(r.Names) || len(r.Names) == 0 {
		w.linef("for range %s {", x)
		return
	}
	if len(r.Names) == 1 {
		if r.IsMap {
			w.linef("for %s := range %s {", r.Names[0], x)
		} else {
			w.linef("for _, %s := range %s {", r.Names[0], x)
		}
		return
	}
	w.linef("for %s, %s := range %s {", r.Names[0], r.Names[1], x)
}

func (w *writer) initString(s ast.Stmt) string {
	switch s := s.(type) {
	case nil:
		return ""
	case *ast.DeclStmt:
		if s.Decl == nil {
			return ""
		}
		typePart := ""
		if s.Decl.Type != nil {
			typePart = " " + w.typeStr(s.Decl.Type)
		}
		if len(s.Decl.Values) == 1 {
			if values, ok := w.propStmt(s.Decl.Values[0], true); ok {
				if len(values) == 0 {
					return ""
				}
				if allBlank(s.Decl.Names) && s.Decl.Type == nil {
					return blanks(len(values)) + " = " + strings.Join(values, ", ")
				}
				if s.Decl.Type != nil {
					return "var " + strings.Join(s.Decl.Names, ", ") + typePart + " = " + strings.Join(values, ", ")
				}
				return strings.Join(s.Decl.Names, ", ") + " := " + strings.Join(values, ", ")
			}
		}
		values := make([]string, 0, len(s.Decl.Values))
		for _, v := range s.Decl.Values {
			if isProp(v) {
				w.f.g.errAt(v, "or is not supported in a for initializer")
				continue
			}
			values = append(values, w.expr(v))
		}
		if allBlank(s.Decl.Names) && s.Decl.Type == nil {
			if len(values) == 1 {
				return "_ = " + values[0]
			}
			return "_, _ = " + strings.Join(values, ", ")
		}
		if s.Decl.Type != nil {
			return "var " + strings.Join(s.Decl.Names, ", ") + typePart + " = " + strings.Join(values, ", ")
		}
		return strings.Join(s.Decl.Names, ", ") + " := " + strings.Join(values, ", ")
	case *ast.AssignStmt:
		if len(s.Rhs) == 1 {
			if isProp(s.Rhs[0]) {
				w.f.g.errAt(s, "or is not supported in a for initializer")
			}
		}
		lhs := make([]string, 0, len(s.Lhs))
		for _, e := range s.Lhs {
			lhs = append(lhs, w.expr(e))
		}
		rhs := make([]string, 0, len(s.Rhs))
		for _, e := range s.Rhs {
			rhs = append(rhs, w.expr(e))
		}
		op := "="
		if s.Op != token.Assign {
			op = s.Op.String()
		}
		return strings.Join(lhs, ", ") + " " + op + " " + strings.Join(rhs, ", ")
	case *ast.ExprStmt:
		return w.expr(s.X)
	}
	return ""
}

func (w *writer) postString(s ast.Stmt) string {
	switch s := s.(type) {
	case nil:
		return ""
	case *ast.AssignStmt:
		if len(s.Rhs) == 1 {
			if isProp(s.Rhs[0]) {
				w.f.g.errAt(s, "or is not supported in a for post statement")
			}
		}
		lhs := make([]string, 0, len(s.Lhs))
		for _, e := range s.Lhs {
			lhs = append(lhs, w.expr(e))
		}
		rhs := make([]string, 0, len(s.Rhs))
		for _, e := range s.Rhs {
			rhs = append(rhs, w.expr(e))
		}
		op := "="
		if s.Op != token.Assign {
			op = s.Op.String()
		}
		return strings.Join(lhs, ", ") + " " + op + " " + strings.Join(rhs, ", ")
	case *ast.ExprStmt:
		return w.expr(s.X)
	case *ast.DeclStmt:
		if s.Decl == nil || len(s.Decl.Values) == 0 {
			return ""
		}
		values := make([]string, 0, len(s.Decl.Values))
		for _, v := range s.Decl.Values {
			values = append(values, w.expr(v))
		}
		return strings.Join(s.Decl.Names, ", ") + " := " + strings.Join(values, ", ")
	}
	return ""
}

func (w *writer) parStmt(s *ast.ParStmt) {
	if s.Body == nil {
		return
	}
	w.line("{")
	w.indent++
	wg := w.next("wg")
	pe := w.next("pe")
	w.linef("var %s %s.WaitGroup", wg, w.f.std("sync"))
	w.linef("var %s _s_rt.ParErrors", pe)
	w.f.needPkg("rt")
	w.linef("%s.Add(%d)", wg, len(s.Body.Stmts))
	for i, st := range s.Body.Stmts {
		sub := w.sub()
		sub.results = nil
		sub.err = false
		sub.deferStack = true
		sub.par = &parRef{pe: pe, ord: strconv.Itoa(i)}
		sub.line("var _s_d _s_rt.DeferStack")
		sub.line("defer _s_d.RunTo(0)")
		sub.stmt(st)
		w.line("go func() {")
		w.indent++
		w.linef("defer %s.Done()", wg)
		w.buf.WriteString(sub.buf.String())
		w.indent--
		w.line("}()")
	}
	w.linef("%s.Wait()", wg)
	errName := w.next("err")
	w.linef("if %s := %s.First(); %s != nil {", errName, pe, errName)
	w.indent++
	w.unhandled(errName)
	w.indent--
	w.line("}")
	w.indent--
	w.line("}")
}

func (w *writer) unhandled(errName string) {
	if w.onErr != "" {
		e := w.next("e")
		w.linef("if %s := %s(%s); %s != nil {", e, w.onErr, errName, e)
		w.indent++
		w.propagateParError(e)
		w.indent--
		w.line("}")
		return
	}
	w.propagateParError(errName)
}

type chunkLoop struct {
	name  string
	start ast.Expr
	end   ast.Expr
	step  ast.Expr
	limit ast.Expr
}

func chunkLoopOf(s *ast.ForStmt) *chunkLoop {
	if s.Range != nil || s.Init == nil || s.Cond == nil || s.Post == nil {
		return nil
	}
	d, ok := s.Init.(*ast.DeclStmt)
	if !ok || d.Decl == nil || d.Decl.Type != nil || len(d.Decl.Names) != 1 || len(d.Decl.Values) != 1 {
		return nil
	}
	name := d.Decl.Names[0]
	if name == "_" || !isIntLit(d.Decl.Values[0]) {
		return nil
	}
	cond, ok := s.Cond.(*ast.BinaryExpr)
	if !ok || cond.Op != token.Lt {
		return nil
	}
	x, ok := unparen(cond.X).(*ast.Ident)
	if !ok || x.Name != name {
		return nil
	}
	if exprUsesName(cond.Y, name) || !sideEffectFree(cond.Y) {
		return nil
	}
	post, ok := s.Post.(*ast.AssignStmt)
	if !ok || post.Op != token.PlusAssign || len(post.Lhs) != 1 || len(post.Rhs) != 1 {
		return nil
	}
	lhs, ok := unparen(post.Lhs[0]).(*ast.Ident)
	if !ok || lhs.Name != name {
		return nil
	}
	if !isPosIntLit(post.Rhs[0]) {
		return nil
	}
	if bodyWritesName(s.Body, name) || !chunkSafeBody(s.Body) {
		return nil
	}
	return &chunkLoop{name: name, start: d.Decl.Values[0], end: cond.Y, step: post.Rhs[0], limit: s.Limit}
}

func isIntLit(e ast.Expr) bool {
	lit, ok := unparen(e).(*ast.BasicLit)
	return ok && lit.Kind == token.Int
}

func isPosIntLit(e ast.Expr) bool {
	lit, ok := unparen(e).(*ast.BasicLit)
	if !ok || lit.Kind != token.Int {
		return false
	}
	n, err := strconv.ParseInt(lit.Value, 0, 64)
	return err == nil && n > 0
}

func exprUsesName(e ast.Expr, name string) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name {
			found = true
			return false
		}
		return !found
	})
	return found
}

func sideEffectFree(e ast.Expr) bool {
	switch e := unparen(e).(type) {
	case *ast.BasicLit, *ast.Ident:
		return true
	case *ast.BinaryExpr:
		return sideEffectFree(e.X) && sideEffectFree(e.Y)
	case *ast.UnaryExpr:
		return sideEffectFree(e.X)
	case *ast.CallExpr:
		if e.Info == nil {
			return false
		}
		switch e.Info.Builtin {
		case "len", "cap", "min", "max":
			for _, a := range e.Args {
				if !sideEffectFree(a) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func bodyWritesName(b *ast.Block, name string) bool {
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			for _, l := range n.Lhs {
				if id, ok := unparen(l).(*ast.Ident); ok && id.Name == name {
					found = true
					return false
				}
			}
		}
		return true
	})
	return found
}

func chunkSafeBody(b *ast.Block) bool {
	if b == nil {
		return true
	}
	ok := true
	ast.Inspect(b, func(n ast.Node) bool {
		if !ok {
			return false
		}
		switch n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.OnErrExpr, *ast.DeferStmt, *ast.OnErrStmt, *ast.WithinStmt,
			*ast.ArenaStmt, *ast.ParStmt, *ast.GoCallStmt, *ast.ReturnStmt:
			ok = false
			return false
		}
		return true
	})
	return ok
}

func bodyCanFail(b *ast.Block) bool {
	can := false
	ast.Inspect(b, func(n ast.Node) bool {
		if can {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.TryExpr:
			can = true
			return false
		case *ast.CallExpr:
			if n.Info != nil && n.Info.Builtin == "check" {
				can = true
				return false
			}
		}
		return true
	})
	return can
}

func (w *writer) chunkedParFor(s *ast.ForStmt, cl *chunkLoop) {
	w.f.needPkg("rt")
	w.line("{")
	w.indent++
	wg := w.next("wg")
	w.linef("var %s %s.WaitGroup", wg, w.f.std("sync"))
	pe := w.next("pe")
	w.linef("var %s _s_rt.ParErrors", pe)
	limit := w.next("limit")
	if cl.limit != nil {
		w.linef("%s := int(%s)", limit, w.expr(cl.limit))
	} else {
		w.linef("%s := %s.GOMAXPROCS(0)", limit, w.f.std("runtime"))
	}
	start := w.next("start")
	end := w.next("end")
	step := w.next("step")
	w.linef("%s := %s", start, w.expr(cl.start))
	w.linef("%s := int(%s)", end, w.expr(cl.end))
	w.linef("%s := %s", step, w.expr(cl.step))
	// shortcut: int range arithmetic overflows near MaxInt; frame loops are small.
	n := w.next("n")
	per := w.next("per")
	w.linef("%s := (%s - %s + %s - 1) / %s", n, end, start, step, step)
	w.linef("if %s < 0 { %s = 0 }", n, n)
	w.linef("if %s > %s { %s = %s }", limit, n, limit, n)
	w.linef("if %s < 1 { %s = 1 }", limit, limit)
	w.linef("%s := (%s + %s - 1) / %s", per, n, limit, limit)
	idx := w.next("w")
	w.linef("for %s := 0; %s < %s; %s++ {", idx, idx, limit, idx)
	w.indent++
	lo := w.next("lo")
	hi := w.next("hi")
	w.linef("%s := %s + %s*%s*%s", lo, start, idx, per, step)
	w.linef("%s := %s + %s*%s", hi, lo, per, step)
	w.linef("if %s > %s { %s = %s }", hi, end, hi, end)
	w.linef("%s.Add(1)", wg)
	w.linef("go func(%s, %s int) {", lo, hi)
	w.indent++
	w.linef("defer %s.Done()", wg)
	sub := w.sub()
	sub.results = nil
	sub.err = false
	sub.deferStack = false
	sub.par = &parRef{pe: pe, ord: cl.name}
	if bodyCanFail(s.Body) {
		label := sub.next("iter")
		sub.par.chunk = label
		sub.linef("%s:", label)
	}
	sub.linef("for %s := %s; %s < %s; %s += %s {", cl.name, lo, cl.name, hi, cl.name, step)
	sub.indent++
	if s.Body != nil {
		sub.genStmts(s.Body.Stmts)
	}
	if cp := w.fuse[s]; cp != nil {
		sub.genStmts(cp.Body.Stmts)
	}
	sub.indent--
	sub.line("}")
	w.buf.WriteString(sub.buf.String())
	w.indent--
	w.linef("}(%s, %s)", lo, hi)
	w.indent--
	w.line("}")
	w.linef("%s.Wait()", wg)
	errName := w.next("err")
	w.linef("if %s := %s.First(); %s != nil {", errName, pe, errName)
	w.indent++
	w.unhandled(errName)
	w.indent--
	w.line("}")
	w.indent--
	w.line("}")
}

func (w *writer) rangeChunkable(r *ast.RangeClause) bool {
	if r == nil || r.IsMap {
		return false
	}
	return w.chunkableRangeType(w.f.g.p.ExprTypes[r.X])
}

func (w *writer) chunkableRangeType(t ast.Type) bool {
	switch t := t.(type) {
	case *ast.SliceType, *ast.ArrayType:
		return true
	case *ast.NamedType:
		if t.Decl != nil && t.Decl.Kind == ast.TypeAlias {
			return w.chunkableRangeType(t.Decl.Target)
		}
		if gt := w.f.g.p.GoTypeOf(t); gt != nil {
			switch gt.Underlying().(type) {
			case *types.Slice, *types.Array:
				return true
			}
		}
	}
	return false
}

func (w *writer) chunkedRangeParFor(s *ast.ForStmt) {
	r := s.Range
	w.f.needPkg("rt")
	w.line("{")
	w.indent++
	wg := w.next("wg")
	w.linef("var %s %s.WaitGroup", wg, w.f.std("sync"))
	pe := w.next("pe")
	w.linef("var %s _s_rt.ParErrors", pe)
	xs := w.next("xs")
	w.linef("%s := %s", xs, w.expr(r.X))
	n := w.next("n")
	w.linef("%s := len(%s)", n, xs)
	limit := w.next("limit")
	if s.Limit != nil {
		w.linef("%s := int(%s)", limit, w.expr(s.Limit))
	} else {
		w.linef("%s := %s.GOMAXPROCS(0)", limit, w.f.std("runtime"))
	}
	w.linef("if %s > %s { %s = %s }", limit, n, limit, n)
	w.linef("if %s < 1 { %s = 1 }", limit, limit)
	per := w.next("per")
	w.linef("%s := (%s + %s - 1) / %s", per, n, limit, limit)
	wkr := w.next("w")
	w.linef("for %s := 0; %s < %s; %s++ {", wkr, wkr, limit, wkr)
	w.indent++
	lo := w.next("lo")
	hi := w.next("hi")
	w.linef("%s := %s * %s", lo, wkr, per)
	w.linef("%s := %s + %s", hi, lo, per)
	w.linef("if %s > %s { %s = %s }", hi, n, hi, n)
	w.linef("%s.Add(1)", wg)
	w.linef("go func(%s, %s int) {", lo, hi)
	w.indent++
	w.linef("defer %s.Done()", wg)
	sub := w.sub()
	sub.results = nil
	sub.err = false
	sub.deferStack = false
	idx := sub.next("i")
	sub.par = &parRef{pe: pe, ord: idx}
	if bodyCanFail(s.Body) {
		label := sub.next("iter")
		sub.par.chunk = label
		sub.linef("%s:", label)
	}
	sub.linef("for %s := %s; %s < %s; %s++ {", idx, lo, idx, hi, idx)
	sub.indent++
	switch len(r.Names) {
	case 0:
	case 1:
		if r.Names[0] != "_" {
			sub.linef("%s := %s[%s]", r.Names[0], xs, idx)
			sub.linef("_ = %s", r.Names[0])
		}
	default:
		if r.Names[0] != "_" {
			sub.linef("%s := %s", r.Names[0], idx)
			sub.linef("_ = %s", r.Names[0])
		}
		if r.Names[1] != "_" {
			sub.linef("%s := %s[%s]", r.Names[1], xs, idx)
			sub.linef("_ = %s", r.Names[1])
		}
	}
	if s.Body != nil {
		sub.genStmts(s.Body.Stmts)
	}
	sub.indent--
	sub.line("}")
	w.buf.WriteString(sub.buf.String())
	w.indent--
	w.linef("}(%s, %s)", lo, hi)
	w.indent--
	w.line("}")
	w.linef("%s.Wait()", wg)
	errName := w.next("err")
	w.linef("if %s := %s.First(); %s != nil {", errName, pe, errName)
	w.indent++
	w.unhandled(errName)
	w.indent--
	w.line("}")
	w.indent--
	w.line("}")
}

func (w *writer) parForStmt(s *ast.ForStmt) {
	if cl := chunkLoopOf(s); cl != nil {
		w.chunkedParFor(s, cl)
		return
	}
	if s.Range != nil && w.rangeChunkable(s.Range) && chunkSafeBody(s.Body) {
		w.chunkedRangeParFor(s)
		return
	}
	scratch := w.loopScratch(s.Body)
	w.line("{")
	w.indent++
	wg := w.next("wg")
	sem := w.next("sem")
	w.linef("var %s %s.WaitGroup", wg, w.f.std("sync"))
	limit := ""
	if s.Limit != nil {
		limit = w.expr(s.Limit)
	} else {
		limit = w.f.std("runtime") + ".GOMAXPROCS(0)"
	}
	w.linef("%s := make(chan struct{}, %s)", sem, limit)
	pe := w.next("pe")
	w.linef("var %s _s_rt.ParErrors", pe)
	w.f.needPkg("rt")
	isMap := s.Range != nil && s.Range.IsMap
	ord := ""
	if !isMap {
		ord = w.next("ord")
		w.linef("%s := 0", ord)
	}
	init := ""
	if typed := typedVarInit(s.Init); typed != nil {
		w.declStmt(typed)
	} else {
		init = w.initString(s.Init)
	}
	var head string
	if s.Range != nil {
		x := w.expr(s.Range.X)
		switch {
		case allBlank(s.Range.Names) || len(s.Range.Names) == 0:
			head = "for range " + x + " {"
		case len(s.Range.Names) == 1 && s.Range.IsMap:
			head = "for " + s.Range.Names[0] + " := range " + x + " {"
		case len(s.Range.Names) == 1:
			head = "for _, " + s.Range.Names[0] + " := range " + x + " {"
		default:
			head = "for " + s.Range.Names[0] + ", " + s.Range.Names[1] + " := range " + x + " {"
		}
	} else {
		cond := ""
		if s.Cond != nil {
			cond = w.expr(s.Cond)
		}
		post := w.postString(s.Post)
		switch {
		case init == "" && post == "" && cond != "":
			head = "for " + cond + " {"
		case init == "" && post == "":
			head = "for {"
		default:
			head = "for " + init + "; " + cond + "; " + post + " {"
		}
	}
	w.line(head)
	w.indent++
	if s.Range != nil {
		for _, n := range s.Range.Names {
			if n != "_" {
				w.linef("_ = %s", n)
			}
		}
	}
	w.linef("%s <- struct{}{}", sem)
	var order []rename
	var orderNames []string
	if s.Range == nil {
		if d, ok := s.Init.(*ast.DeclStmt); ok && d.Decl != nil {
			for _, n := range d.Decl.Names {
				if n == "_" {
					continue
				}
				capture := w.next("i")
				w.linef("%s := %s", capture, n)
				w.linef("_ = %s", capture)
				order = append(order, rename{name: capture, decl: d.Decl})
				orderNames = append(orderNames, n)
			}
		} else if a, ok := s.Init.(*ast.AssignStmt); ok {
			for _, l := range a.Lhs {
				id, ok := l.(*ast.Ident)
				if !ok || id.Name == "_" {
					continue
				}
				capture := w.next("i")
				w.linef("%s := %s", capture, id.Name)
				w.linef("_ = %s", capture)
				var decl ast.Node
				if id.Ref != nil {
					decl = id.Ref.Decl
				}
				order = append(order, rename{name: capture, decl: decl})
				orderNames = append(orderNames, id.Name)
			}
		}
	}
	ordName := "-1"
	if !isMap {
		ordName = w.next("ord")
		w.linef("%s := %s", ordName, ord)
		w.linef("_ = %s", ordName)
		w.linef("%s++", ord)
	}
	w.linef("%s.Add(1)", wg)
	sub := w.sub()
	sub.results = nil
	sub.err = false
	sub.deferStack = bodyHasDefer(s.Body)
	sub.par = &parRef{pe: pe, ord: ordName}
	if sub.deferStack {
		sub.line("var _s_d _s_rt.DeferStack")
		sub.line("defer _s_d.RunTo(0)")
	}
	if s.Body != nil {
		for i, r := range order {
			sub.renames[orderNames[i]] = r
		}
		sub.genStmts(s.Body.Stmts)
	}
	w.line("go func() {")
	w.indent++
	w.linef("defer %s.Done()", wg)
	w.linef("defer func() { <-%s }()", sem)
	w.buf.WriteString(sub.buf.String())
	w.indent--
	w.line("}()")
	w.indent--
	w.line("}")
	w.linef("%s.Wait()", wg)
	errName := w.next("err")
	w.linef("if %s := %s.First(); %s != nil {", errName, pe, errName)
	w.indent++
	w.unhandled(errName)
	w.indent--
	w.line("}")
	w.indent--
	w.line("}")
	for _, name := range scratch {
		w.linef("%s.Release()", name)
	}
}

func (w *writer) withinStmt(s *ast.WithinStmt) {
	w.f.needPkg("rt")
	w.line("{")
	w.indent++
	ctx := w.next("ctx")
	cancel := w.next("cancel")
	mark := ""
	if !blockTerminates(s.Body) {
		mark = w.next("mark")
		w.linef("%s := _s_d.Mark()", mark)
	}
	w.linef("%s, %s := %s.WithTimeout(%s, %s)", ctx, cancel, w.f.std("context"), w.currentCtx(), w.expr(s.Duration))
	w.linef("_ = %s", ctx)
	w.linef("_s_d.Push(%s)", cancel)
	old := w.ctx
	w.ctx = ctx
	if s.Body != nil {
		w.genStmts(s.Body.Stmts)
	}
	w.ctx = old
	if mark != "" {
		w.linef("_s_d.RunTo(%s)", mark)
	}
	w.indent--
	w.line("}")
}

func bodyHasDefer(b *ast.Block) bool {
	if b == nil {
		return false
	}
	found := false
	ast.Inspect(b, func(n ast.Node) bool {
		if found {
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		switch n.(type) {
		case *ast.DeferStmt, *ast.WithinStmt, *ast.ArenaStmt:
			found = true
			return false
		}
		return true
	})
	return found
}

func hasDirectDefer(stmts []ast.Stmt) bool {
	for _, s := range stmts {
		if _, ok := s.(*ast.DeferStmt); ok {
			return true
		}
	}
	return false
}
