package gen

import (
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
	w.genStmts(b.Stmts)
	if markName != "" {
		w.linef("_s_d.RunTo(%s)", markName)
	}
}

func (w *writer) genStmts(stmts []ast.Stmt) {
	for _, s := range stmts {
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
		if or, ok := s.X.(*ast.OrExpr); ok {
			w.orPrelude(or, false)
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
	case *ast.FailStmt:
		w.failStmt(s)
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
		if o, isOr := d.Values[0].(*ast.OrExpr); isOr {
			values := w.orPrelude(o, true)
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
		if _, ok := v.(*ast.OrExpr); ok {
			w.f.g.errAt(v, "or is not supported in this declaration")
			continue
		}
		values = append(values, w.expr(v))
	}
	names := strings.Join(d.Names, ", ")
	if allBlank(d.Names) && d.Type == nil {
		if len(values) == 1 {
			w.linef("_ = %s", values[0])
		} else {
			w.linef("_, _ = %s", strings.Join(values, ", "))
		}
		return
	}
	w.linef("%s %s%s = %s", prefix, names, typePart, strings.Join(values, ", "))
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
	lhs := make([]string, 0, len(s.Lhs))
	for _, e := range s.Lhs {
		lhs = append(lhs, w.expr(e))
	}
	if len(s.Rhs) == 1 {
		if or, ok := s.Rhs[0].(*ast.OrExpr); ok {
			values := w.orPrelude(or, true)
			if len(values) == 0 {
				return
			}
			w.linef("%s = %s", strings.Join(lhs, ", "), strings.Join(values, ", "))
			return
		}
	}
	rhs := make([]string, 0, len(s.Rhs))
	for _, e := range s.Rhs {
		if _, ok := e.(*ast.OrExpr); ok {
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

func (w *writer) returnStmt(s *ast.ReturnStmt) {
	if len(s.Results) == 1 {
		if or, ok := s.Results[0].(*ast.OrExpr); ok {
			results := 1
			if call, ok := or.X.(*ast.CallExpr); ok && call.Info != nil {
				results = call.Info.Results
			}
			w.ret(w.orPrelude(or, results > 0))
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
		w.linef("_s_d.RunTo(%s)", w.loops[len(w.loops)-1])
	}
	if w.matchDepth > 0 && len(w.loopLabels) > 0 {
		if label := w.loopLabels[len(w.loopLabels)-1]; label != "" {
			kind += " " + label
		}
	}
	w.line(kind)
}

func (w *writer) orPrelude(or *ast.OrExpr, wantValue bool) []string {
	results := 1
	var callStr string
	if call, ok := or.X.(*ast.CallExpr); ok {
		if call.Info != nil {
			results = call.Info.Results
		}
		callStr = w.call(call)
	} else {
		callStr = w.expr(or.X)
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
	w.orError(or, errName)
	return values
}

func blanks(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "_"
	}
	return strings.Join(parts, ", ")
}

func (w *writer) orError(or *ast.OrExpr, errName string) {
	switch or.Mode {
	case ast.OrReturn:
		w.linef("if %s != nil {", errName)
		w.indent++
		w.propagateParError(errName)
		w.indent--
		w.line("}")
	case ast.OrHandle:
		w.linef("if %s != nil {", errName)
		w.indent++
		if or.Name != "" && or.Name != "_" {
			w.linef("%s := %s", or.Name, errName)
			w.linef("_ = %s", or.Name)
		}
		w.line("{")
		w.indent++
		if or.Block != nil {
			w.bodyContents(or.Block, hasDirectDefer(or.Block.Stmts))
		}
		w.indent--
		w.line("}")
		w.indent--
		w.line("}")
	case ast.OrFail:
		w.linef("if %s != nil {", errName)
		w.indent++
		target := errName
		if or.FailArg != nil {
			w.f.needPkg("fmt")
			name := w.next("e")
			w.linef("%s := fmt.Errorf(\"%%s: %%w\", %s, %s)", name, w.expr(or.FailArg), errName)
			target = name
		}
		w.propagateParError(target)
		w.indent--
		w.line("}")
	case ast.OrPanic:
		arg := errName
		if or.PanicArg != nil {
			arg = w.expr(or.PanicArg)
		}
		w.linef("if %s != nil {", errName)
		w.indent++
		if w.par != nil {
			if w.deferStack {
				w.line("_s_d.RunTo(0)")
			}
			w.f.needPkg("rt")
			w.linef("%s.Add(%s, _s_rt.AsError(%s))", w.par.pe, w.par.ord, arg)
			if w.par.cancel != "" {
				w.linef("%s()", w.par.cancel)
			}
			w.line("return")
		} else {
			w.linef("panic(%s)", arg)
		}
		w.indent--
		w.line("}")
	}
}

func (w *writer) propagateParError(errName string) {
	if w.par != nil {
		if w.deferStack {
			w.line("_s_d.RunTo(0)")
		}
		if w.par.pe != "" {
			w.linef("%s.Add(%s, %s)", w.par.pe, w.par.ord, errName)
		}
		if w.par.cancel != "" {
			w.linef("%s()", w.par.cancel)
		}
		w.line("return")
		return
	}
	w.returnErr(errName)
}

func (w *writer) failStmt(s *ast.FailStmt) {
	errExpr := w.expr(s.Err)
	if s.Msg != nil {
		w.f.needPkg("fmt")
		name := w.next("e")
		w.linef("%s := fmt.Errorf(\"%%s: %%w\", %s, %s)", name, w.expr(s.Msg), errExpr)
		errExpr = name
	}
	w.propagateParError(errExpr)
}

func (w *writer) arenaStmt(s *ast.ArenaStmt) {
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
		fun, prefix, handled := w.callTarget(c)
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
		if len(prefix) == 0 && !handled && w.callNeedsCtx(c) {
			tmp := w.next("x")
			w.linef("%s := %s", tmp, w.currentCtx())
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
	case *ast.FailStmt:
		return matchJumpExpr(s.Err, inMatch) || matchJumpExpr(s.Msg, inMatch)
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
	case *ast.OrExpr:
		if e.Block != nil && matchJump(e.Block, inMatch) {
			return true
		}
		return matchJumpExpr(e.PanicArg, inMatch)
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
	if w.deferStack && (!terminates || loopJumps(s.Body)) {
		mark = w.next("mark")
		w.linef("%s := _s_d.Mark()", mark)
		w.loops = append(w.loops, mark)
	}
	if s.Body != nil {
		w.genStmts(s.Body.Stmts)
	}
	if mark != "" {
		if !terminates {
			w.linef("_s_d.RunTo(%s)", mark)
		}
		w.loops = w.loops[:len(w.loops)-1]
	}
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
			if or, ok := s.Decl.Values[0].(*ast.OrExpr); ok {
				values := w.orPrelude(or, true)
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
			if _, ok := v.(*ast.OrExpr); ok {
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
			if _, ok := s.Rhs[0].(*ast.OrExpr); ok {
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
			if _, ok := s.Rhs[0].(*ast.OrExpr); ok {
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
	w.linef("var %s %s.WaitGroup", wg, w.f.std("sync"))
	ctx := w.next("ctx")
	cancel := w.next("cancel")
	pe := w.next("pe")
	w.linef("%s, %s := %s.WithCancel(%s)", ctx, cancel, w.f.std("context"), w.currentCtx())
	w.linef("_ = %s", ctx)
	w.linef("var %s _s_rt.ParErrors", pe)
	w.f.needPkg("rt")
	w.linef("%s.Add(%d)", wg, len(s.Body.Stmts))
	for i, st := range s.Body.Stmts {
		sub := w.sub()
		sub.ctx = ctx
		sub.results = nil
		sub.err = false
		sub.deferStack = true
		sub.par = &parRef{pe: pe, ord: strconv.Itoa(i), cancel: cancel}
		sub.line("var _s_d _s_rt.DeferStack")
		sub.line("defer _s_d.RunTo(0)")
		sub.stmt(st)
		w.line("go func() {")
		w.indent++
		w.linef("defer %s.Done()", wg)
		w.linef("defer func() { if r := recover(); r != nil { %s.Add(%s, _s_rt.NewPanicError(r)); %s() } }()", pe, sub.par.ord, cancel)
		w.buf.WriteString(sub.buf.String())
		w.indent--
		w.line("}()")
	}
	w.linef("%s.Wait()", wg)
	errName := w.next("err")
	w.linef("if %s := %s.First(); %s != nil {", errName, pe, errName)
	w.indent++
	w.linef("%s()", cancel)
	w.propagateParError(errName)
	w.indent--
	w.line("}")
	w.linef("%s()", cancel)
	w.indent--
	w.line("}")
}

func (w *writer) parForStmt(s *ast.ForStmt) {
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
	ctx := w.next("ctx")
	cancel := w.next("cancel")
	pe := w.next("pe")
	w.linef("%s, %s := %s.WithCancel(%s)", ctx, cancel, w.f.std("context"), w.currentCtx())
	w.linef("_ = %s", ctx)
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
	w.linef("%s <- struct{}{}", sem)
	var order []rename
	var orderNames []string
	if s.Range != nil {
		for _, n := range s.Range.Names {
			if n == "_" {
				continue
			}
			capture := w.next("i")
			w.linef("%s := %s", capture, n)
			w.linef("_ = %s", capture)
			order = append(order, rename{name: capture})
			orderNames = append(orderNames, n)
		}
	} else if d, ok := s.Init.(*ast.DeclStmt); ok && d.Decl != nil {
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
	ordName := "-1"
	if !isMap {
		ordName = w.next("ord")
		w.linef("%s := %s", ordName, ord)
		w.linef("%s++", ord)
	}
	w.linef("%s.Add(1)", wg)
	sub := w.sub()
	sub.ctx = ctx
	sub.results = nil
	sub.err = false
	sub.deferStack = true
	sub.par = &parRef{pe: pe, ord: ordName, cancel: cancel}
	sub.line("var _s_d _s_rt.DeferStack")
	sub.line("defer _s_d.RunTo(0)")
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
	w.linef("defer func() { if r := recover(); r != nil { %s.Add(%s, _s_rt.NewPanicError(r)); %s() } }()", pe, ordName, cancel)
	w.buf.WriteString(sub.buf.String())
	w.indent--
	w.line("}()")
	w.indent--
	w.line("}")
	w.linef("%s.Wait()", wg)
	errName := w.next("err")
	w.linef("if %s := %s.First(); %s != nil {", errName, pe, errName)
	w.indent++
	w.linef("%s()", cancel)
	w.propagateParError(errName)
	w.indent--
	w.line("}")
	w.linef("%s()", cancel)
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
