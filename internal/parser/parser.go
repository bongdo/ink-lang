package parser

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bongdo/ink/internal/ast"
	"github.com/bongdo/ink/internal/lexer"
	"github.com/bongdo/ink/internal/token"
)

const maxErrors = 100

type Error struct {
	Pos token.Pos
	End token.Pos
	Msg string
}

type parser struct {
	toks   []token.Token
	i      int
	errs   []Error
	silent int
	noLit  int
}

func Parse(path string, src []byte) (*ast.File, []Error) {
	toks, comments, lexErrs := lexer.Lex(src)
	p := &parser{toks: toks}
	f := &ast.File{Path: path, Src: src}
	for _, e := range lexErrs {
		p.addError(e.Pos, e.End, e.Msg)
	}
	for _, c := range comments {
		f.Comments = append(f.Comments, &ast.Comment{
			Span:  ast.Span{Start: c.Pos, End: c.End},
			Text:  c.Text,
			Block: c.Block,
		})
	}
	p.parseFile(f)
	sort.SliceStable(p.errs, func(i, j int) bool {
		return p.errs[i].Pos.Offset < p.errs[j].Pos.Offset
	})
	return f, p.errs
}

func (p *parser) parseFile(f *ast.File) {
	start := token.Pos{Offset: 0, Line: 1, Col: 1}
	for !p.at(token.EOF) {
		before := p.i
		switch p.kind() {
		case token.Semi:
			p.next()
		case token.Use:
			f.Uses = append(f.Uses, p.parseUse())
		case token.Fn:
			f.Decls = append(f.Decls, p.parseFuncDecl(nil))
		case token.Type:
			f.Decls = append(f.Decls, p.parseTypeDecl())
		case token.Var, token.Const:
			f.Decls = append(f.Decls, p.parseVarDecl(true))
		case token.Test:
			f.Decls = append(f.Decls, p.parseTestDecl())
		default:
			if msg := goOnlyMsg(p.tok()); msg != "" {
				p.errorf(p.tok().Pos, "%s", msg)
			} else {
				p.errorf(p.tok().Pos, "expected declaration, found %s", describe(p.tok()))
			}
			p.recoverTop()
		}
		if p.i == before {
			p.next()
		}
	}
	f.Span = ast.Span{Start: start, End: p.fileEnd()}
}

func (p *parser) tok() token.Token {
	if p.i >= len(p.toks) {
		eof := p.fileEnd()
		return token.Token{Kind: token.EOF, Pos: eof, End: eof}
	}
	return p.toks[p.i]
}

func (p *parser) kind() token.Kind {
	return p.tok().Kind
}

func (p *parser) at(k token.Kind) bool {
	return p.kind() == k
}

func (p *parser) peek(n int) token.Kind {
	j := p.i + n
	if j < 0 || j >= len(p.toks) {
		return token.EOF
	}
	return p.toks[j].Kind
}

func (p *parser) next() token.Token {
	t := p.tok()
	if t.Kind != token.EOF && p.i < len(p.toks) {
		p.i++
	}
	return t
}

func (p *parser) fileEnd() token.Pos {
	if len(p.toks) > 0 {
		return p.toks[len(p.toks)-1].End
	}
	return token.Pos{Offset: 0, Line: 1, Col: 1}
}

func (p *parser) lastEnd() token.Pos {
	if p.i > 0 {
		return p.toks[p.i-1].End
	}
	return p.tok().Pos
}

func (p *parser) fin(start token.Pos) ast.Span {
	end := p.lastEnd()
	if end.Offset < start.Offset {
		end = start
	}
	if end.Line < 1 {
		end.Line = 1
	}
	return ast.Span{Start: start, End: end}
}

func spanOf(t token.Token) ast.Span {
	return ast.Span{Start: t.Pos, End: t.End}
}

func (p *parser) addError(pos, end token.Pos, msg string) {
	if len(p.errs) >= maxErrors {
		return
	}
	p.errs = append(p.errs, Error{Pos: pos, End: end, Msg: msg})
}

func (p *parser) errorf(pos token.Pos, format string, args ...any) {
	if p.silent > 0 {
		return
	}
	p.addError(pos, p.endAt(pos), fmt.Sprintf(format, args...))
}

func (p *parser) endAt(pos token.Pos) token.Pos {
	t := p.tok()
	if t.Pos == pos && t.End.Offset > pos.Offset {
		return t.End
	}
	if p.i > 0 {
		prev := p.toks[p.i-1]
		if prev.Pos == pos && prev.End.Offset > pos.Offset {
			return prev.End
		}
	}
	return token.Pos{Offset: pos.Offset + 1, Line: pos.Line, Col: pos.Col + 1}
}

func describe(t token.Token) string {
	if t.Kind == token.Ident || t.Lit != "" {
		return strconv.Quote(t.Lit)
	}
	return t.Kind.String()
}

func goOnlyMsg(t token.Token) string {
	if t.Kind != token.Ident {
		return ""
	}
	switch t.Lit {
	case "package":
		return "files have no package clause; one directory is one package"
	case "import":
		return "import is not an Ink declaration; Ink imports with use"
	case "func":
		return "functions are declared with fn, not func"
	case "switch":
		return "switch does not exist; use if or match"
	case "chan":
		return "chan does not exist; keep channels in a Go adapter"
	case "select":
		return "select does not exist; use go or a Go adapter"
	case "goto":
		return "goto does not exist; use for, break and continue"
	case "fallthrough":
		return "fallthrough does not exist; give each match arm its own body"
	}
	return ""
}

func (p *parser) expect(k token.Kind) token.Pos {
	if p.at(k) {
		return p.next().End
	}
	p.errorf(p.tok().Pos, "expected %s, found %s", k, describe(p.tok()))
	return p.lastEnd()
}

func (p *parser) expectIdent() (token.Token, bool) {
	t := p.tok()
	if t.Kind != token.Ident {
		p.errorf(t.Pos, "expected identifier, found %s", describe(t))
		return t, false
	}
	return p.next(), true
}

func (p *parser) consumeSemi() (token.Pos, bool) {
	if p.at(token.Semi) {
		return p.next().End, true
	}
	p.errorf(p.tok().Pos, "expected ';', found %s", describe(p.tok()))
	return p.lastEnd(), false
}

func (p *parser) stringLit(t token.Token) string {
	s, err := strconv.Unquote(t.Lit)
	if err != nil {
		p.errorf(t.Pos, "invalid string literal")
		return t.Lit
	}
	return s
}

func (p *parser) recoverTop() {
	for !p.at(token.EOF) {
		switch p.kind() {
		case token.Semi, token.RBrace:
			p.next()
			return
		case token.Use, token.Fn, token.Type, token.Var, token.Const, token.Test:
			return
		}
		p.next()
	}
}

func (p *parser) recoverStmt() {
	for !p.at(token.EOF) {
		switch p.kind() {
		case token.Semi:
			p.next()
			return
		case token.RBrace, token.Case:
			return
		}
		p.next()
	}
}

func (p *parser) skipStmt() {
	depth := 0
	for !p.at(token.EOF) {
		switch p.kind() {
		case token.LBrace:
			depth++
		case token.RBrace:
			if depth == 0 {
				return
			}
			depth--
		case token.Semi:
			if depth == 0 {
				return
			}
		}
		p.next()
	}
}

func (p *parser) parseUse() *ast.Use {
	start := p.tok().Pos
	p.next()
	u := &ast.Use{}
	pathPos := start
	if (p.at(token.Path) || p.at(token.Ident)) && p.tok().Lit == "_" &&
		(p.peek(1) == token.Path || p.peek(1) == token.Ident) {
		p.errorf(p.tok().Pos, "blank import is written use <path> as _;")
		for !p.at(token.Semi) && !p.at(token.EOF) {
			p.next()
		}
		u.Path = "_"
	} else {
		switch p.kind() {
		case token.Path:
			u.Path = p.next().Lit
		case token.Ident:
			u.Path = p.next().Lit
		case token.String, token.RawString:
			t := p.next()
			pathPos = t.Pos
			u.Path = p.stringLit(t)
			p.errorf(t.Pos, "import paths are written without quotes: use %s;", u.Path)
		default:
			p.errorf(p.tok().Pos, "expected import path, found %s", describe(p.tok()))
		}
	}
	u.Local = strings.HasPrefix(u.Path, "./") || strings.HasPrefix(u.Path, "../")
	if p.at(token.As) {
		p.next()
		if t, ok := p.expectIdent(); ok {
			u.Alias = t.Lit
		}
	}
	if !u.Local && u.Alias == "" && versionSegment(u.Path) {
		p.errorf(pathPos, "versioned import path %q requires an alias: use %q as <name>;", u.Path, u.Path)
	}
	p.consumeSemi()
	u.Span = p.fin(start)
	return u
}

func versionSegment(path string) bool {
	path = strings.TrimSuffix(path, "/")
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		path = path[i+1:]
	}
	if len(path) < 2 || path[0] != 'v' {
		return false
	}
	n := 0
	for _, r := range path[1:] {
		if r < '0' || r > '9' {
			return false
		}
		n = n*10 + int(r-'0')
	}
	return n >= 2
}

func (p *parser) parseFuncDecl(owner *ast.TypeDecl) *ast.FuncDecl {
	start := p.tok().Pos
	p.next()
	fd := &ast.FuncDecl{Owner: owner}
	if t, ok := p.expectIdent(); ok {
		fd.Name = t.Lit
	}
	if p.at(token.LBracket) {
		fd.TypeParams = p.parseTypeParams()
	}
	fd.Params = p.parseParams()
	fd.Results = p.parseResults(true)
	fd.Err = p.parseErrSuffix()
	if p.at(token.Assign) {
		p.errorf(p.tok().Pos, "function bodies are declared without '='")
		p.next()
	}
	if p.at(token.LBrace) {
		fd.Body = p.parseBlock()
	} else {
		p.errorf(p.tok().Pos, "expected function body, found %s", describe(p.tok()))
	}
	p.consumeSemi()
	fd.Span = p.fin(start)
	return fd
}

func (p *parser) parseErrSuffix() bool {
	if p.at(token.Bang) {
		p.next()
		return true
	}
	if p.at(token.Or) {
		pos := p.tok().Pos
		p.next()
		p.parseErrorName()
		p.errorf(pos, "fallible results are marked with '!': write T! instead of T or error")
		return true
	}
	return false
}

func (p *parser) parseErrorName() {
	t := p.tok()
	if t.Kind == token.Ident && t.Lit == "error" {
		p.next()
		return
	}
	p.errorf(t.Pos, "expected 'error', found %s", describe(t))
	if t.Kind == token.Ident {
		p.next()
	}
}

func (p *parser) parseTypeParams() []*ast.TypeParam {
	var list []*ast.TypeParam
	p.next()
	for !p.at(token.RBracket) && !p.at(token.EOF) {
		t := p.tok()
		if t.Kind != token.Ident {
			p.errorf(t.Pos, "expected type parameter, found %s", describe(t))
			break
		}
		p.next()
		list = append(list, &ast.TypeParam{Span: spanOf(t), Name: t.Lit})
		if p.at(token.Comma) {
			p.next()
			continue
		}
		break
	}
	p.expect(token.RBracket)
	return list
}

func (p *parser) parseResults(allowList bool) []ast.Type {
	if p.at(token.LParen) {
		p.next()
		var list []ast.Type
		if !p.at(token.RParen) {
			for {
				t := p.parseType()
				if t == nil {
					break
				}
				list = append(list, t)
				if p.at(token.Comma) {
					p.next()
					continue
				}
				break
			}
		}
		p.expect(token.RParen)
		return list
	}
	if !p.canStartType() {
		return nil
	}
	first := p.parseType()
	if first == nil {
		return nil
	}
	list := []ast.Type{first}
	if !allowList {
		return list
	}
	for p.at(token.Comma) {
		p.next()
		if !p.canStartType() {
			break
		}
		t := p.parseType()
		if t == nil {
			break
		}
		list = append(list, t)
	}
	return list
}

func (p *parser) parseParams() []*ast.Param {
	var list []*ast.Param
	if !p.at(token.LParen) {
		p.errorf(p.tok().Pos, "expected '(', found %s", describe(p.tok()))
		return list
	}
	p.next()
	for !p.at(token.RParen) && !p.at(token.EOF) {
		t := p.tok()
		if t.Kind != token.Ident {
			p.errorf(t.Pos, "expected parameter name, found %s", describe(t))
			break
		}
		start := t.Pos
		p.next()
		param := &ast.Param{Name: t.Lit}
		if p.at(token.Colon) {
			p.next()
			param.Type = p.parseType()
		}
		param.Span = p.fin(start)
		list = append(list, param)
		if p.at(token.Comma) {
			p.next()
			continue
		}
		break
	}
	p.expect(token.RParen)
	return list
}

func (p *parser) parseTypeDecl() *ast.TypeDecl {
	start := p.tok().Pos
	p.next()
	td := &ast.TypeDecl{}
	if t, ok := p.expectIdent(); ok {
		td.Name = t.Lit
	}
	if p.at(token.LBracket) {
		td.TypeParams = p.parseTypeParams()
	}
	assign := false
	var assignPos token.Pos
	if p.at(token.Assign) {
		assignPos = p.tok().Pos
		p.next()
		assign = true
	}
	if assign && (p.at(token.Struct) || p.at(token.Enum) || p.at(token.Interface)) {
		p.errorf(assignPos, "type definitions are declared without '='")
	}
	switch p.kind() {
	case token.Struct:
		td.Kind = ast.TypeStruct
		p.next()
		p.parseStructBody(td)
	case token.Enum:
		td.Kind = ast.TypeEnum
		p.next()
		p.parseEnumBody(td)
	case token.Interface:
		td.Kind = ast.TypeInterface
		p.next()
		p.parseInterfaceBody(td)
	default:
		if !assign {
			p.errorf(p.tok().Pos, "expected '=', found %s", describe(p.tok()))
		}
		td.Kind = ast.TypeAlias
		td.Target = p.parseType()
	}
	p.consumeSemi()
	td.Span = p.fin(start)
	return td
}

func (p *parser) parseStructBody(td *ast.TypeDecl) {
	if !p.at(token.LBrace) {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
		return
	}
	p.next()
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		switch p.kind() {
		case token.Fn:
			td.Methods = append(td.Methods, p.parseFuncDecl(td))
		case token.Ident:
			if p.peek(1) == token.LBrace {
				td.Contracts = append(td.Contracts, p.parseContract())
				break
			}
			td.Fields = append(td.Fields, p.parseField())
			if p.at(token.Comma) {
				p.next()
			} else if !p.at(token.RBrace) && !p.at(token.Fn) && !p.startsContract() {
				p.errorf(p.tok().Pos, "expected ',' after field, found %s", describe(p.tok()))
			}
		case token.Semi:
			p.next()
		default:
			p.errorf(p.tok().Pos, "expected field or method, found %s", describe(p.tok()))
			p.next()
		}
	}
	p.expect(token.RBrace)
}

func (p *parser) startsContract() bool {
	return p.at(token.Ident) && p.peek(1) == token.LBrace
}

func (p *parser) contractShaped() bool {
	start := p.i
	if p.at(token.Ident) && p.peek(1) == token.LBrace {
		start = p.i + 1
	}
	if start >= len(p.toks) || p.toks[start].Kind != token.LBrace {
		return false
	}
	depth := 0
	for i := start; i < len(p.toks); i++ {
		switch p.toks[i].Kind {
		case token.LBrace:
			depth++
		case token.RBrace:
			depth--
			if depth == 0 {
				return false
			}
		case token.Ident:
			if depth == 1 && i+2 < len(p.toks) && p.toks[i+1].Kind == token.Colon &&
				(p.toks[i+2].Kind == token.String || p.toks[i+2].Kind == token.RawString) {
				return true
			}
		}
	}
	return false
}

func (p *parser) skipContractBlock() {
	if !p.at(token.LBrace) {
		return
	}
	p.next()
	p.skipContract()
	if p.at(token.RBrace) {
		p.next()
	}
}

func (p *parser) parseContract() *ast.Contract {
	start := p.tok().Pos
	name := p.next().Lit
	c := &ast.Contract{Name: name}
	if !p.at(token.LBrace) {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
		c.Span = p.fin(start)
		return c
	}
	p.next()
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		if p.at(token.Comma) || p.at(token.Semi) {
			p.next()
			continue
		}
		if !p.at(token.Ident) {
			p.errorf(p.tok().Pos, "expected field name, found %s", describe(p.tok()))
			p.skipContract()
			break
		}
		fieldStart := p.tok().Pos
		field := p.next().Lit
		if !p.at(token.Colon) {
			p.errorf(p.tok().Pos, "expected ':', found %s", describe(p.tok()))
			p.skipContract()
			break
		}
		p.next()
		if !p.at(token.String) && !p.at(token.RawString) {
			p.errorf(p.tok().Pos, "expected external name string, found %s", describe(p.tok()))
			p.skipContract()
			break
		}
		ext := p.stringLit(p.next())
		c.Fields = append(c.Fields, &ast.ContractField{
			Span:     p.fin(fieldStart),
			Field:    field,
			External: ext,
		})
		if p.at(token.Comma) || p.at(token.Semi) {
			p.next()
		} else if !p.at(token.RBrace) {
			p.errorf(p.tok().Pos, "expected ',' or ';', found %s", describe(p.tok()))
			p.skipContract()
			break
		}
	}
	if p.at(token.RBrace) {
		p.next()
	}
	if p.at(token.Semi) || p.at(token.Comma) {
		p.next()
	}
	c.Span = p.fin(start)
	return c
}

func (p *parser) skipContract() {
	depth := 0
	for !p.at(token.EOF) {
		switch p.kind() {
		case token.LBrace:
			depth++
			p.next()
		case token.RBrace:
			if depth == 0 {
				return
			}
			depth--
			p.next()
		default:
			p.next()
		}
	}
}

func (p *parser) parseField() *ast.Field {
	start := p.tok().Pos
	t := p.next()
	f := &ast.Field{Name: t.Lit}
	p.expect(token.Colon)
	f.Type = p.parseType()
	f.Span = p.fin(start)
	return f
}

func (p *parser) parseEnumBody(td *ast.TypeDecl) {
	if !p.at(token.LBrace) {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
		return
	}
	p.next()
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		switch p.kind() {
		case token.Ident:
			if p.startsContract() && p.contractShaped() {
				p.errorf(p.tok().Pos, "contract: blocks are only allowed in struct bodies")
				p.next()
				p.skipContractBlock()
				if p.at(token.Comma) || p.at(token.Semi) {
					p.next()
				}
				continue
			}
			td.Variants = append(td.Variants, p.parseVariant(td))
			if p.at(token.Comma) {
				p.next()
			} else if !p.at(token.RBrace) {
				p.errorf(p.tok().Pos, "expected ',' after variant, found %s", describe(p.tok()))
			}
		case token.Semi:
			p.next()
		default:
			p.errorf(p.tok().Pos, "expected variant, found %s", describe(p.tok()))
			p.next()
		}
	}
	p.expect(token.RBrace)
}

func (p *parser) parseVariant(td *ast.TypeDecl) *ast.Variant {
	start := p.tok().Pos
	v := &ast.Variant{Enum: td}
	if t, ok := p.expectIdent(); ok {
		v.Name = t.Lit
	}
	if p.at(token.LBrace) {
		p.next()
		for !p.at(token.RBrace) && !p.at(token.EOF) {
			if p.at(token.Ident) {
				v.Fields = append(v.Fields, p.parseField())
				if p.at(token.Comma) {
					p.next()
					continue
				}
				if !p.at(token.RBrace) {
					p.errorf(p.tok().Pos, "expected ',' after variant field, found %s", describe(p.tok()))
				}
				break
			}
			p.errorf(p.tok().Pos, "expected variant field, found %s", describe(p.tok()))
			p.next()
		}
		p.expect(token.RBrace)
	}
	v.Span = p.fin(start)
	return v
}

func (p *parser) parseInterfaceBody(td *ast.TypeDecl) {
	if !p.at(token.LBrace) {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
		return
	}
	p.next()
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		switch p.kind() {
		case token.Ident:
			if p.startsContract() {
				p.errorf(p.tok().Pos, "contract: blocks are only allowed in struct bodies")
				p.next()
				p.skipContractBlock()
				if p.at(token.Comma) || p.at(token.Semi) {
					p.next()
				}
				continue
			}
			td.IFace = append(td.IFace, p.parseIFaceMethod())
		case token.Fn:
			td.IFace = append(td.IFace, p.parseIFaceMethod())
		case token.Semi:
			p.next()
		default:
			p.errorf(p.tok().Pos, "expected interface method, found %s", describe(p.tok()))
			p.next()
		}
	}
	p.expect(token.RBrace)
}

func (p *parser) parseIFaceMethod() *ast.IFaceMethod {
	start := p.tok().Pos
	if p.at(token.Fn) {
		p.errorf(p.tok().Pos, "interface methods are declared without 'fn'")
		p.next()
	}
	m := &ast.IFaceMethod{}
	if t, ok := p.expectIdent(); ok {
		m.Name = t.Lit
	}
	m.Params = p.parseParams()
	m.Results = p.parseResults(true)
	m.Err = p.parseErrSuffix()
	p.consumeSemi()
	m.Span = p.fin(start)
	return m
}

func (p *parser) parseTestDecl() *ast.TestDecl {
	start := p.tok().Pos
	p.next()
	td := &ast.TestDecl{}
	switch p.kind() {
	case token.String, token.RawString:
		td.Name = p.stringLit(p.next())
	default:
		p.errorf(p.tok().Pos, "expected test name, found %s", describe(p.tok()))
	}
	if p.at(token.LBrace) {
		td.Body = p.parseBlock()
	} else {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
	}
	p.consumeSemi()
	td.Span = p.fin(start)
	return td
}

func (p *parser) parseVarDecl(requireSemi bool) *ast.VarDecl {
	start := p.tok().Pos
	vd := &ast.VarDecl{Const: p.kind() == token.Const}
	p.next()
	for p.at(token.Ident) {
		vd.Names = append(vd.Names, p.next().Lit)
		if !p.at(token.Comma) {
			break
		}
		comma := p.next()
		if !p.at(token.Ident) {
			p.errorf(comma.Pos, "expected variable name after ','")
			break
		}
	}
	if len(vd.Names) == 0 {
		p.errorf(p.tok().Pos, "expected variable name, found %s", describe(p.tok()))
	}
	if msg := goOnlyMsg(p.tok()); msg != "" {
		p.errorf(p.tok().Pos, "%s", msg)
		p.skipStmt()
		if requireSemi {
			p.consumeSemi()
		}
		vd.Span = p.fin(start)
		return vd
	}
	if p.at(token.Colon) {
		p.next()
		vd.Type = p.parseType()
	}
	if p.at(token.Assign) {
		p.next()
		vd.Values = p.parseExprList(true)
	} else {
		p.errorf(p.tok().Pos, "expected '=', found %s", describe(p.tok()))
	}
	if requireSemi {
		p.consumeSemi()
	}
	vd.Span = p.fin(start)
	return vd
}

func (p *parser) parseStmt() ast.Stmt {
	switch p.kind() {
	case token.LBrace:
		start := p.tok().Pos
		b := p.parseBlock()
		p.consumeSemi()
		b.Span = p.fin(start)
		return b
	case token.Var, token.Const:
		return p.parseSimpleStmt(true)
	case token.If:
		return p.parseIfStmt()
	case token.For:
		return p.parseForStmt(false, p.tok().Pos)
	case token.Go:
		return p.parseGoStmt()
	case token.On:
		return p.parseOnErrStmt()
	case token.Par:
		p.errorf(p.tok().Pos, "par was renamed to go")
		return p.parseGoStmt()
	case token.Within:
		pos := p.tok().Pos
		p.errorf(pos, "within was removed; use context.WithTimeout")
		p.next()
		p.parseExprNoLit()
		if p.at(token.LBrace) {
			p.parseBlock()
		}
		p.consumeSemi()
		return nil
	case token.Return:
		return p.parseReturnStmt()
	case token.Break:
		start := p.next().Pos
		p.consumeSemi()
		return &ast.BreakStmt{Span: p.fin(start)}
	case token.Continue:
		start := p.next().Pos
		p.consumeSemi()
		return &ast.ContinueStmt{Span: p.fin(start)}
	case token.Defer:
		return p.parseDeferStmt()
	case token.Fail:
		return p.parseFailStmt()
	case token.Arena:
		return p.parseArenaStmt()
	case token.Semi:
		t := p.next()
		return &ast.EmptyStmt{Span: spanOf(t)}
	default:
		if msg := goOnlyMsg(p.tok()); msg != "" {
			p.errorf(p.tok().Pos, "%s", msg)
			p.skipStmt()
			return nil
		}
		return p.parseSimpleStmt(true)
	}
}

func (p *parser) parseFailStmt() ast.Stmt {
	start := p.next().Pos
	s := &ast.FailStmt{}
	if p.at(token.LParen) {
		p.next()
		if !p.at(token.RParen) {
			s.Err = p.parseExprNoOr()
			if p.at(token.Comma) {
				p.next()
				s.Msg = p.parseExprNoOr()
			}
		}
		p.expect(token.RParen)
	}
	p.consumeSemi()
	s.Span = p.fin(start)
	return s
}

func (p *parser) parseArenaStmt() ast.Stmt {
	start := p.next().Pos
	s := &ast.ArenaStmt{}
	if p.at(token.LBrace) {
		s.Body = p.parseBlock()
	} else {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
	}
	p.consumeSemi()
	s.Span = p.fin(start)
	return s
}

func (p *parser) parseIfStmt() ast.Stmt {
	start := p.tok().Pos
	p.next()
	cond := p.parseIfCond()
	var then *ast.Block
	if p.at(token.LBrace) {
		then = p.parseBlock()
	} else {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
	}
	var els ast.Stmt
	elseIf := false
	if p.at(token.Else) {
		p.next()
		switch p.kind() {
		case token.If:
			els = p.parseIfStmt()
			elseIf = true
		case token.LBrace:
			els = p.parseBlock()
		default:
			p.errorf(p.tok().Pos, "expected 'if' or '{', found %s", describe(p.tok()))
		}
	}
	if !elseIf {
		p.consumeSemi()
	}
	return &ast.IfStmt{Span: p.fin(start), Cond: cond, Then: then, Else: els}
}

func (p *parser) parseIfCond() ast.Expr {
	if p.at(token.LParen) {
		p.next()
		var e ast.Expr
		if !p.at(token.RParen) {
			e = p.parseExprNoOr()
		}
		p.expect(token.RParen)
		return e
	}
	p.errorf(p.tok().Pos, "expected '(', found %s", describe(p.tok()))
	return p.parseExprNoOr()
}

func (p *parser) parseForStmt(par bool, start token.Pos) *ast.ForStmt {
	if !p.at(token.For) {
		p.errorf(p.tok().Pos, "expected 'for', found %s", describe(p.tok()))
		return &ast.ForStmt{Span: p.fin(start), Par: par}
	}
	p.next()
	fs := &ast.ForStmt{Par: par}
	if p.at(token.LParen) {
		p.next()
	} else {
		p.errorf(p.tok().Pos, "expected '(', found %s", describe(p.tok()))
	}
	if !p.at(token.RParen) {
		if p.rangeAhead() {
			fs.Range = p.parseRangeClause()
		} else {
			if p.at(token.Semi) {
				p.next()
			} else {
				fs.Init = p.parseSimpleStmt(false)
				p.consumeSemi()
			}
			if !p.at(token.Semi) {
				fs.Cond = p.parseExprNoOr()
			}
			p.consumeSemi()
			if !p.at(token.RParen) {
				fs.Post = p.parseSimpleStmt(false)
			}
		}
	}
	p.expect(token.RParen)
	if p.at(token.Limit) {
		p.next()
		fs.Limit = p.parseExprNoLit()
	}
	if p.at(token.LBrace) {
		fs.Body = p.parseBlock()
	} else {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
	}
	p.consumeSemi()
	fs.Span = p.fin(start)
	return fs
}

func (p *parser) rangeAhead() bool {
	if p.kind() != token.Var {
		return false
	}
	j := p.i + 1
	if j >= len(p.toks) || p.toks[j].Kind != token.Ident {
		return false
	}
	j++
	for j < len(p.toks) && p.toks[j].Kind == token.Comma {
		j++
		if j >= len(p.toks) || p.toks[j].Kind != token.Ident {
			return false
		}
		j++
	}
	return j < len(p.toks) && p.toks[j].Kind == token.In
}

func (p *parser) parseRangeClause() *ast.RangeClause {
	start := p.tok().Pos
	p.next()
	rc := &ast.RangeClause{}
	for p.at(token.Ident) {
		rc.Names = append(rc.Names, p.next().Lit)
		if !p.at(token.Comma) {
			break
		}
		p.next()
	}
	if len(rc.Names) == 0 {
		p.errorf(p.tok().Pos, "expected range variable, found %s", describe(p.tok()))
	}
	if p.at(token.In) {
		p.next()
	} else {
		p.errorf(p.tok().Pos, "expected 'in', found %s", describe(p.tok()))
	}
	rc.X = p.parseExprNoOr()
	rc.Span = p.fin(start)
	return rc
}

func (p *parser) parseGoStmt() ast.Stmt {
	start := p.tok().Pos
	p.next()
	if p.at(token.For) {
		return p.parseForStmt(true, start)
	}
	if p.at(token.LBrace) {
		body := p.parseBlock()
		p.consumeSemi()
		return &ast.ParStmt{Span: p.fin(start), Body: body}
	}
	call := p.parseExpr()
	p.consumeSemi()
	return &ast.GoCallStmt{Span: p.fin(start), Call: call}
}

func (p *parser) parseOnErrStmt() ast.Stmt {
	start := p.tok().Pos
	p.next()
	if !p.at(token.Ident) || p.tok().Lit != "err" {
		p.errorf(p.tok().Pos, "expected 'err' after 'on'")
	}
	if p.at(token.Ident) {
		p.next()
	}
	var body *ast.Block
	if p.at(token.LBrace) {
		body = p.parseBlock()
	} else {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
	}
	p.consumeSemi()
	return &ast.OnErrStmt{Span: p.fin(start), Body: body}
}

func (p *parser) parseWithinStmt() ast.Stmt {
	start := p.tok().Pos
	p.next()
	d := p.parseExprNoLit()
	var body *ast.Block
	if p.at(token.LBrace) {
		body = p.parseBlock()
	} else {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
	}
	p.consumeSemi()
	return &ast.WithinStmt{Span: p.fin(start), Duration: d, Body: body}
}

func (p *parser) parseReturnStmt() ast.Stmt {
	start := p.tok().Pos
	p.next()
	var results []ast.Expr
	if !p.at(token.Semi) && !p.at(token.RBrace) && !p.at(token.EOF) {
		results = p.parseExprList(true)
	}
	p.consumeSemi()
	return &ast.ReturnStmt{Span: p.fin(start), Results: results}
}

func (p *parser) parseDeferStmt() ast.Stmt {
	start := p.tok().Pos
	p.next()
	var call *ast.CallExpr
	x := p.parseExprNoOr()
	if x != nil {
		c, ok := x.(*ast.CallExpr)
		if !ok {
			p.errorf(x.Pos(), "defer requires a call expression")
			c = &ast.CallExpr{Span: ast.Span{Start: x.Pos(), End: x.EndPos()}, Fun: x}
		}
		call = c
	}
	p.consumeSemi()
	return &ast.DeferStmt{Span: p.fin(start), Call: call}
}

func (p *parser) parseSimpleStmt(requireSemi bool) ast.Stmt {
	start := p.tok().Pos
	if p.at(token.Var) || p.at(token.Const) {
		vd := p.parseVarDecl(false)
		if requireSemi {
			if end, ok := p.consumeSemi(); ok {
				vd.End = end
			}
		}
		return &ast.DeclStmt{Span: p.fin(start), Decl: vd}
	}
	var lhs []ast.Expr
	for {
		e := p.parseExprNoOr()
		if e == nil {
			break
		}
		lhs = append(lhs, e)
		if p.at(token.Comma) {
			p.next()
			if !p.canStartExpr() {
				break
			}
			continue
		}
		break
	}
	var assign *ast.AssignStmt
	var expr ast.Expr
	if p.at(token.Colon) && p.peek(1) == token.Assign {
		p.errorf(p.tok().Pos, "no ':=' in Ink; declare with var")
		p.next()
		p.next()
		assign = &ast.AssignStmt{Lhs: lhs, Op: token.Assign, Rhs: p.parseExprList(true)}
	} else if p.isAssignOp() {
		op := p.next().Kind
		assign = &ast.AssignStmt{Lhs: lhs, Op: op, Rhs: p.parseExprList(true)}
	} else if len(lhs) == 1 && p.at(token.Or) {
		expr = p.parseOr(lhs[0])
	} else if len(lhs) == 1 && p.at(token.On) {
		expr = p.parseOn(lhs[0])
	} else if len(lhs) > 1 {
		p.errorf(lhs[1].Pos(), "multiple expressions require an assignment")
		expr = lhs[0]
	} else if len(lhs) == 1 {
		expr = lhs[0]
	}
	if requireSemi {
		p.consumeSemi()
	}
	span := p.fin(start)
	if assign != nil {
		assign.Span = span
		return assign
	}
	if expr != nil {
		return &ast.ExprStmt{Span: span, X: expr}
	}
	return nil
}

func (p *parser) isAssignOp() bool {
	switch p.kind() {
	case token.Assign, token.PlusAssign, token.MinusAssign, token.StarAssign,
		token.SlashAssign, token.PercentAssign, token.AmpAssign, token.PipeAssign,
		token.CaretAssign, token.ShlAssign, token.ShrAssign:
		return true
	}
	return false
}

func (p *parser) parseBlock() *ast.Block {
	start := p.tok().Pos
	b := &ast.Block{}
	if !p.at(token.LBrace) {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
		b.Span = ast.Span{Start: start, End: start}
		return b
	}
	p.next()
	for !p.at(token.RBrace) && !p.at(token.EOF) && !p.at(token.Case) {
		if p.at(token.Semi) {
			p.next()
			continue
		}
		before := p.i
		st := p.parseStmt()
		if st != nil {
			b.Stmts = append(b.Stmts, st)
		}
		if p.i == before {
			p.recoverStmt()
		}
	}
	p.expect(token.RBrace)
	b.Span = p.fin(start)
	return b
}

func (p *parser) parseExpr() ast.Expr {
	x := p.parseExprNoOr()
	if x == nil {
		return nil
	}
	if p.at(token.Or) {
		return p.parseOr(x)
	}
	if p.at(token.On) {
		return p.parseOn(x)
	}
	return x
}

func (p *parser) parseExprNoOr() ast.Expr {
	return p.parseBinary(0)
}

func (p *parser) parseExprNoLit() ast.Expr {
	p.noLit++
	x := p.parseExprNoOr()
	p.noLit--
	return x
}

func (p *parser) parseOr(x ast.Expr) ast.Expr {
	p.next()
	start := x.Pos()
	switch p.kind() {
	case token.Return:
		pos := p.tok().Pos
		p.next()
		p.errorf(pos, "use 'or fail' instead of 'or return'")
		return &ast.OrExpr{Span: p.fin(start), X: x, Mode: ast.OrFail}
	case token.Fail:
		p.next()
		var arg ast.Expr
		if p.at(token.LParen) {
			p.next()
			if !p.at(token.RParen) {
				arg = p.parseExprNoOr()
				if p.at(token.Comma) {
					p.errorf(p.tok().Pos, "fail takes at most one message")
					p.next()
					p.parseExprNoOr()
				}
			}
			p.expect(token.RParen)
		}
		return &ast.OrExpr{Span: p.fin(start), X: x, Mode: ast.OrFail, FailArg: arg}
	case token.Ident:
		name := p.next()
		switch name.Lit {
		case "err":
			p.errorf(name.Pos, "use 'on err { ... }' instead of 'or err { ... }'")
			if !p.at(token.LBrace) {
				p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
				return &ast.OrExpr{Span: p.fin(start), X: x, Mode: ast.OrFail}
			}
			block := p.parseBlock()
			return &ast.OrExpr{Span: p.fin(start), X: x, Mode: ast.OrHandle, Name: "err", Block: block}
		case "panic":
			p.errorf(name.Pos, "or panic was removed; use on err { panic(...) } or or fail")
			var arg ast.Expr
			if p.at(token.LParen) {
				p.next()
				if !p.at(token.RParen) {
					arg = p.parseExprNoOr()
				}
				p.expect(token.RParen)
			}
			return &ast.OrExpr{Span: p.fin(start), X: x, Mode: ast.OrFail, FailArg: arg}
		default:
			p.errorf(name.Pos, "expected 'fail' after 'or'")
			return &ast.OrExpr{Span: p.fin(start), X: x, Mode: ast.OrFail}
		}
	default:
		p.errorf(p.tok().Pos, "expected 'fail' after 'or'")
		return &ast.OrExpr{Span: p.fin(start), X: x, Mode: ast.OrFail}
	}
}

func (p *parser) parseOn(x ast.Expr) ast.Expr {
	p.next()
	start := x.Pos()
	if !p.at(token.Ident) || p.tok().Lit != "err" {
		p.errorf(p.tok().Pos, "expected 'err' after 'on'")
	}
	if p.at(token.Ident) {
		p.next()
	}
	var block *ast.Block
	if p.at(token.LBrace) {
		block = p.parseBlock()
	} else {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
	}
	return &ast.OrExpr{Span: p.fin(start), X: x, Mode: ast.OrHandle, Name: "err", Block: block}
}

func (p *parser) parseExprList(top bool) []ast.Expr {
	var list []ast.Expr
	for {
		var e ast.Expr
		if top {
			e = p.parseExpr()
		} else {
			e = p.parseExprNoOr()
		}
		if e == nil {
			break
		}
		list = append(list, e)
		if !p.at(token.Comma) {
			break
		}
		p.next()
		if !p.canStartExpr() {
			break
		}
	}
	return list
}

func (p *parser) canStartExpr() bool {
	switch p.kind() {
	case token.Ident, token.Int, token.Float, token.String, token.RawString,
		token.Rune, token.Duration, token.True, token.False, token.Nil,
		token.Fn, token.Match, token.LParen, token.Plus, token.Minus,
		token.Bang, token.Caret, token.Tilde, token.Amp, token.Star,
		token.Map, token.Struct, token.Interface:
		return true
	}
	return false
}

func (p *parser) canStartType() bool {
	switch p.kind() {
	case token.Ident, token.Star, token.Map, token.Fn, token.Struct,
		token.Interface, token.LParen:
		return true
	}
	return false
}

func (p *parser) parseBinary(minPrec int) ast.Expr {
	x := p.parseUnary()
	if x == nil {
		return nil
	}
	for {
		op := p.kind()
		prec := binaryPrec(op)
		if prec == 0 || prec < minPrec {
			break
		}
		p.next()
		y := p.parseBinary(prec + 1)
		if y == nil {
			p.errorf(p.tok().Pos, "expected expression after operator")
			break
		}
		x = &ast.BinaryExpr{
			Span: ast.Span{Start: x.Pos(), End: y.EndPos()},
			Op:   op,
			X:    x,
			Y:    y,
		}
	}
	return x
}

func binaryPrec(k token.Kind) int {
	switch k {
	case token.OrOr:
		return 1
	case token.AndAnd:
		return 2
	case token.Eq, token.Ne, token.Lt, token.Le, token.Gt, token.Ge:
		return 3
	case token.Plus, token.Minus, token.Pipe, token.Caret:
		return 4
	case token.Star, token.Slash, token.Percent, token.Amp, token.Shl, token.Shr:
		return 5
	}
	return 0
}

func (p *parser) parseUnary() ast.Expr {
	switch p.kind() {
	case token.Plus, token.Minus, token.Bang, token.Caret, token.Tilde,
		token.Amp, token.Star:
		op := p.next()
		x := p.parseUnary()
		if x == nil {
			return nil
		}
		return &ast.UnaryExpr{
			Span: ast.Span{Start: op.Pos, End: x.EndPos()},
			Op:   op.Kind,
			X:    x,
		}
	}
	return p.parsePostfix()
}

func (p *parser) parsePostfix() ast.Expr {
	x := p.parsePrimary()
	if x == nil {
		return nil
	}
	for {
		switch p.kind() {
		case token.Dot:
			p.next()
			t := p.tok()
			if t.Kind != token.Ident {
				p.errorf(t.Pos, "expected selector name, found %s", describe(t))
				return x
			}
			p.next()
			x = &ast.SelectorExpr{Span: p.fin(x.Pos()), X: x, Name: t.Lit}
		case token.LParen:
			x = p.parseCall(x, nil)
		case token.LBracket:
			x = p.parseIndex(x)
		default:
			return x
		}
	}
}

func (p *parser) parseCall(fun ast.Expr, typeArgs []ast.Type) ast.Expr {
	start := fun.Pos()
	p.next()
	var args []ast.Expr
	ellipsis := false
	for !p.at(token.RParen) && !p.at(token.EOF) {
		arg := p.parseExprNoOr()
		if arg == nil {
			break
		}
		if p.at(token.Ellipsis) {
			p.next()
			ellipsis = true
		}
		args = append(args, arg)
		if p.at(token.Comma) {
			p.next()
			continue
		}
		break
	}
	p.expect(token.RParen)
	return &ast.CallExpr{
		Span:     p.fin(start),
		Fun:      fun,
		TypeArgs: typeArgs,
		Args:     args,
		Ellipsis: ellipsis,
	}
}

func (p *parser) parseIndex(x ast.Expr) ast.Expr {
	start := x.Pos()
	p.next()
	if p.at(token.Colon) {
		p.next()
		var high ast.Expr
		if !p.at(token.RBracket) && !p.at(token.EOF) {
			high = p.parseExprNoOr()
		}
		p.expect(token.RBracket)
		return &ast.SliceExpr{Span: p.fin(start), X: x, High: high}
	}
	if p.at(token.RBracket) {
		p.errorf(p.tok().Pos, "empty index")
		p.next()
		return &ast.IndexExpr{Span: p.fin(start), X: x}
	}
	save := p.i
	saveErrs := len(p.errs)
	p.silent++
	types, ok := p.parseTypeList()
	ok = ok && p.at(token.RBracket)
	p.silent--
	p.errs = p.errs[:saveErrs]
	if ok && p.peek(1) == token.LParen && p.typeArgsAllowed(x, types) {
		p.next()
		return p.parseCall(x, types)
	}
	p.i = save
	low := p.parseExprNoOr()
	if p.at(token.Colon) {
		p.next()
		var high ast.Expr
		if !p.at(token.RBracket) && !p.at(token.EOF) {
			high = p.parseExprNoOr()
		}
		p.expect(token.RBracket)
		return &ast.SliceExpr{Span: p.fin(start), X: x, Low: low, High: high}
	}
	p.expect(token.RBracket)
	return &ast.IndexExpr{Span: p.fin(start), X: x, Index: low}
}

func (p *parser) typeArgsAllowed(callee ast.Expr, types []ast.Type) bool {
	if len(types) > 1 {
		return true
	}
	if len(types) == 0 {
		return false
	}
	if t, ok := types[0].(*ast.NamedType); ok && t.Package == "" && builtinTypes[t.Name] {
		return true
	}
	name := ""
	switch c := callee.(type) {
	case *ast.Ident:
		name = c.Name
	case *ast.SelectorExpr:
		name = c.Name
	}
	if name == "make" || name == "new" {
		return true
	}
	if name != "" {
		r, _ := utf8.DecodeRuneInString(name)
		return unicode.IsUpper(r)
	}
	return false
}

var builtinTypes = map[string]bool{
	"void": true, "bool": true,
	"i8": true, "i16": true, "i32": true, "i64": true,
	"u8": true, "u16": true, "u32": true, "u64": true,
	"int": true, "uint": true, "f32": true, "f64": true,
	"str": true, "rune": true, "byte": true, "usize": true,
	"isize": true, "any": true, "error": true, "Duration": true,
	"Type": true,
}

func (p *parser) parsePrimary() ast.Expr {
	t := p.tok()
	switch t.Kind {
	case token.Ident:
		if p.noLit == 0 {
			if x := p.tryComposite(); x != nil {
				return x
			}
			if p.peek(1) == token.LBracket {
				if x := p.trySliceCtor(); x != nil {
					return x
				}
			}
		}
		if t.Lit == "make" && p.peek(1) == token.LBracket {
			p.errorf(t.Pos, "make was removed; use T[](n), e.g. i32[](3)")
		}
		p.next()
		return &ast.Ident{Span: spanOf(t), Name: t.Lit}
	case token.Int, token.Float, token.String, token.RawString, token.Rune, token.Duration:
		p.next()
		return &ast.BasicLit{Span: spanOf(t), Kind: t.Kind, Value: t.Lit}
	case token.True, token.False:
		p.next()
		return &ast.BasicLit{Span: spanOf(t), Kind: t.Kind, Value: t.Kind.String()}
	case token.Nil:
		return p.parseNilLit()
	case token.Fn:
		return p.parseFuncLit()
	case token.Match:
		return p.parseMatch()
	case token.Map, token.Struct, token.Interface:
		if x := p.tryComposite(); x != nil {
			return x
		}
		p.errorf(t.Pos, "expected composite literal, found %s", describe(t))
		return nil
	case token.LParen:
		start := p.next().Pos
		x := p.parseExprNoOr()
		end := p.expect(token.RParen)
		return &ast.ParenExpr{Span: ast.Span{Start: start, End: end}, X: x}
	default:
		p.errorf(t.Pos, "expected expression, found %s", describe(t))
		return nil
	}
}

func (p *parser) tryComposite() ast.Expr {
	save := p.i
	saveErrs := len(p.errs)
	p.silent++
	typ := p.parseType()
	ok := typ != nil && p.at(token.LBrace)
	p.silent--
	p.errs = p.errs[:saveErrs]
	if !ok {
		p.i = save
		return nil
	}
	p.i = save
	typ = p.parseType()
	if typ == nil || !p.at(token.LBrace) {
		return nil
	}
	return p.parseCompositeLit(typ)
}

func (p *parser) trySliceCtor() ast.Expr {
	save := p.i
	saveErrs := len(p.errs)
	p.silent++
	typ := p.parseType()
	ok := typ != nil && p.at(token.LParen)
	p.silent--
	p.errs = p.errs[:saveErrs]
	p.i = save
	if !ok {
		return nil
	}
	typ = p.parseType()
	slice, isSlice := typ.(*ast.SliceType)
	if !isSlice || !p.at(token.LParen) {
		p.i = save
		return nil
	}
	return p.parseSliceCtor(slice.Elem)
}

func (p *parser) parseSliceCtor(elem ast.Type) ast.Expr {
	start := elem.Pos()
	p.expect(token.LParen)
	call := &ast.CallExpr{
		Slice:    true,
		Fun:      &ast.Ident{Span: ast.Span{Start: start, End: elem.EndPos()}, Name: "make"},
		TypeArgs: []ast.Type{elem},
	}
	named := false
	for !p.at(token.RParen) && !p.at(token.EOF) {
		if p.at(token.Ident) && p.peek(1) == token.Colon {
			label := p.next()
			p.next()
			value := p.parseExprNoOr()
			switch label.Lit {
			case "size":
				if call.Size != nil {
					p.errorf(label.Pos, "size given twice")
				}
				call.Size = value
			case "cap":
				if call.Cap != nil {
					p.errorf(label.Pos, "cap given twice")
				}
				call.Cap = value
			default:
				p.errorf(label.Pos, "unknown slice argument %s; use size or cap", label.Lit)
			}
			named = true
		} else {
			if named || call.Size != nil {
				p.errorf(p.tok().Pos, "one size comes first, then cap: or size:")
			}
			call.Size = p.parseExprNoOr()
		}
		if p.at(token.Comma) {
			p.next()
			continue
		}
		break
	}
	end := p.expect(token.RParen)
	call.Span = ast.Span{Start: start, End: end}
	return call
}

func (p *parser) parseCompositeLit(typ ast.Type) ast.Expr {
	start := typ.Pos()
	_, isMap := typ.(*ast.MapType)
	p.expect(token.LBrace)
	c := &ast.CompositeLit{Type: typ}
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		entryStart := p.tok().Pos
		var key string
		var keyExpr, value ast.Expr
		if !isMap && p.at(token.Ident) && p.peek(1) == token.Colon {
			key = p.next().Lit
			p.next()
			value = p.parseExprNoOr()
		} else {
			first := p.parseExprNoOr()
			if first == nil {
				break
			}
			if p.at(token.Colon) {
				p.next()
				keyExpr = first
				value = p.parseExprNoOr()
			} else if p.at(token.Assign) {
				p.next()
				if !isMap {
					if id, ok := first.(*ast.Ident); ok {
						key = id.Name
					} else {
						keyExpr = first
					}
				} else {
					keyExpr = first
				}
				value = p.parseExprNoOr()
			} else {
				value = first
			}
		}
		if value == nil {
			break
		}
		c.Entries = append(c.Entries, &ast.CompositeEntry{
			Span:    p.fin(entryStart),
			Key:     key,
			KeyExpr: keyExpr,
			Value:   value,
		})
		if p.at(token.Comma) {
			p.next()
			continue
		}
		break
	}
	p.expect(token.RBrace)
	c.Span = p.fin(start)
	return c
}

func (p *parser) parseNilLit() ast.Expr {
	start := p.next().Pos
	var typ ast.Type
	if p.at(token.LBracket) {
		p.next()
		typ = p.parseType()
		p.expect(token.RBracket)
	}
	return &ast.NilLit{Span: p.fin(start), Type: typ}
}

func (p *parser) parseFuncLit() ast.Expr {
	start := p.next().Pos
	fl := &ast.FuncLit{}
	fl.Params = p.parseParams()
	fl.Results = p.parseResults(true)
	fl.Err = p.parseErrSuffix()
	switch {
	case p.at(token.Arrow):
		p.next()
		fl.Arrow = p.parseExprNoOr()
	case p.at(token.LBrace):
		fl.Body = p.parseBlock()
	default:
		p.errorf(p.tok().Pos, "expected '=>' or '{', found %s", describe(p.tok()))
	}
	fl.Span = p.fin(start)
	return fl
}

func (p *parser) parseMatch() *ast.MatchExpr {
	start := p.next().Pos
	m := &ast.MatchExpr{}
	if p.at(token.LParen) {
		p.next()
		if !p.at(token.RParen) {
			m.Subject = p.parseExprNoOr()
		}
		p.expect(token.RParen)
	} else {
		p.errorf(p.tok().Pos, "expected '(', found %s", describe(p.tok()))
		m.Subject = p.parseExprNoOr()
	}
	if !p.at(token.LBrace) {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
		m.Span = p.fin(start)
		return m
	}
	p.next()
	for !p.at(token.RBrace) && !p.at(token.EOF) {
		if p.at(token.Semi) {
			p.next()
			continue
		}
		if !p.at(token.Case) && !p.at(token.Else) {
			p.errorf(p.tok().Pos, "expected 'case' or 'else', found %s", describe(p.tok()))
			p.next()
			continue
		}
		m.Cases = append(m.Cases, p.parseCase())
	}
	p.expect(token.RBrace)
	m.IsExpr = len(m.Cases) > 0
	for _, c := range m.Cases {
		if c.Expr == nil {
			m.IsExpr = false
		}
	}
	m.Span = p.fin(start)
	return m
}

func (p *parser) parseCase() *ast.Case {
	startTok := p.next()
	start := startTok.Pos
	c := &ast.Case{}
	if startTok.Kind == token.Else {
		c.Default = true
		c.Pat = &ast.Pattern{Span: spanOf(startTok), Wildcard: true}
	} else {
		c.Pat = p.parsePattern()
	}
	p.expect(token.Arrow)
	switch {
	case p.at(token.LBrace):
		c.Block = p.parseBlock()
	case p.at(token.Case), p.at(token.RBrace), p.at(token.EOF):
		p.errorf(p.tok().Pos, "expected case body, found %s", describe(p.tok()))
	case p.at(token.Semi):
		p.errorf(p.tok().Pos, "expected case body, found ';'")
	default:
		var stmts []ast.Stmt
		for !p.at(token.Case) && !p.at(token.Else) && !p.at(token.RBrace) && !p.at(token.EOF) {
			before := p.i
			st := p.parseStmt()
			if st != nil {
				stmts = append(stmts, st)
			}
			if p.i == before {
				p.recoverStmt()
			}
			for p.at(token.Semi) {
				p.next()
			}
		}
		if len(stmts) == 1 {
			if es, ok := stmts[0].(*ast.ExprStmt); ok {
				c.Expr = es.X
			} else {
				c.Body = stmts
			}
		} else if len(stmts) > 1 {
			c.Body = stmts
		}
	}
	for p.at(token.Semi) {
		p.next()
	}
	c.Span = p.fin(start)
	return c
}

func (p *parser) parsePattern() *ast.Pattern {
	t := p.tok()
	start := t.Pos
	switch t.Kind {
	case token.Int, token.String, token.RawString, token.Rune, token.Duration,
		token.True, token.False:
		p.next()
		value := t.Lit
		if t.Kind == token.True || t.Kind == token.False {
			value = t.Kind.String()
		}
		return &ast.Pattern{
			Span:    spanOf(t),
			Literal: &ast.BasicLit{Span: spanOf(t), Kind: t.Kind, Value: value},
		}
	case token.Minus:
		p.next()
		lit := p.tok()
		switch lit.Kind {
		case token.Int, token.Float, token.Duration:
			p.next()
			span := ast.Span{Start: start, End: lit.End}
			return &ast.Pattern{
				Span:    span,
				Literal: &ast.BasicLit{Span: span, Kind: lit.Kind, Value: "-" + lit.Lit},
			}
		}
		p.errorf(lit.Pos, "expected number after '-' in pattern, found %s", describe(lit))
		return nil
	case token.Nil:
		lit := p.parseNilLit().(*ast.NilLit)
		return &ast.Pattern{Span: lit.Span, Literal: lit}
	case token.Star:
		typ := p.parseType()
		pat := &ast.Pattern{Span: spanOf(t), TypePat: typ}
		if p.at(token.As) {
			p.next()
			if b, ok := p.expectIdent(); ok {
				pat.Bind = b.Lit
			}
		}
		pat.Span = p.fin(start)
		return pat
	case token.Ident:
		p.next()
		if t.Lit == "_" {
			p.errorf(t.Pos, "use else for the default case")
			return &ast.Pattern{Span: spanOf(t), Wildcard: true}
		}
		pat := &ast.Pattern{Name: t.Lit}
		if p.at(token.Dot) {
			p.next()
			id := p.tok()
			if id.Kind != token.Ident {
				p.errorf(id.Pos, "expected pattern name, found %s", describe(id))
				pat.Span = p.fin(start)
				return pat
			}
			p.next()
			pat.Qualifier = t.Lit
			pat.Name = id.Lit
		}
		if p.at(token.As) {
			p.next()
			if b, ok := p.expectIdent(); ok {
				pat.Bind = b.Lit
			}
		}
		if p.at(token.LBrace) {
			p.next()
			for !p.at(token.RBrace) && !p.at(token.EOF) {
				id := p.tok()
				if id.Kind != token.Ident {
					p.errorf(id.Pos, "expected field name, found %s", describe(id))
					break
				}
				p.next()
				pat.Fields = append(pat.Fields, id.Lit)
				if p.at(token.Comma) {
					p.next()
					continue
				}
				if !p.at(token.RBrace) {
					p.errorf(p.tok().Pos, "expected ',' after field, found %s", describe(p.tok()))
				}
				break
			}
			p.expect(token.RBrace)
		}
		pat.Span = p.fin(start)
		return pat
	default:
		p.errorf(t.Pos, "expected pattern, found %s", describe(t))
		return nil
	}
}

func (p *parser) parseType() ast.Type {
	t := p.parseTypePrimary()
	if t == nil {
		return nil
	}
	return p.parseTypePostfix(t)
}

func (p *parser) parseTypePrimary() ast.Type {
	t := p.tok()
	switch t.Kind {
	case token.Ident:
		return p.parseNamedType()
	case token.Star:
		start := p.next().Pos
		elem := p.parseType()
		if elem == nil {
			return nil
		}
		return &ast.PointerType{Span: ast.Span{Start: start, End: elem.EndPos()}, Elem: elem}
	case token.Map:
		return p.parseMapType()
	case token.Fn:
		return p.parseFuncType()
	case token.Struct:
		return p.parseStructType()
	case token.Interface:
		return p.parseInterfaceType()
	case token.LParen:
		p.next()
		inner := p.parseType()
		p.expect(token.RParen)
		return inner
	default:
		p.errorf(t.Pos, "expected type, found %s", describe(t))
		return nil
	}
}

func (p *parser) parseNamedType() ast.Type {
	start := p.tok().Pos
	var parts []string
	for {
		id := p.tok()
		if id.Kind != token.Ident {
			p.errorf(id.Pos, "expected type name, found %s", describe(id))
			return nil
		}
		p.next()
		parts = append(parts, id.Lit)
		if !p.at(token.Dot) {
			break
		}
		if len(parts) >= 3 {
			p.errorf(p.tok().Pos, "type name has too many qualifiers")
			return nil
		}
		p.next()
	}
	nt := &ast.NamedType{Span: p.fin(start)}
	switch len(parts) {
	case 1:
		nt.Name = parts[0]
	case 2:
		nt.Package = parts[0]
		nt.Name = parts[1]
	case 3:
		nt.Package = parts[0] + "." + parts[1]
		nt.Name = parts[2]
	}
	return nt
}

func (p *parser) parseMapType() ast.Type {
	start := p.next().Pos
	p.expect(token.LBracket)
	key := p.parseType()
	p.expect(token.Comma)
	value := p.parseType()
	end := p.expect(token.RBracket)
	if key == nil || value == nil {
		return nil
	}
	return &ast.MapType{
		Span:  ast.Span{Start: start, End: end},
		Key:   key,
		Value: value,
	}
}

func (p *parser) parseFuncType() ast.Type {
	start := p.next().Pos
	var params []ast.Type
	if p.at(token.LParen) {
		p.next()
		for !p.at(token.RParen) && !p.at(token.EOF) {
			t := p.parseType()
			if t == nil {
				break
			}
			params = append(params, t)
			if p.at(token.Comma) {
				p.next()
				continue
			}
			break
		}
		p.expect(token.RParen)
	} else {
		p.errorf(p.tok().Pos, "expected '(', found %s", describe(p.tok()))
	}
	ft := &ast.FuncType{Params: params}
	ft.Results = p.parseResults(false)
	ft.Err = p.parseErrSuffix()
	ft.Span = p.fin(start)
	return ft
}

func (p *parser) parseStructType() ast.Type {
	start := p.next().Pos
	st := &ast.StructType{}
	if p.at(token.LBrace) {
		p.next()
		for !p.at(token.RBrace) && !p.at(token.EOF) {
			if p.at(token.Ident) {
				st.Fields = append(st.Fields, p.parseField())
				if p.at(token.Comma) {
					p.next()
				} else if !p.at(token.RBrace) {
					p.errorf(p.tok().Pos, "expected ',' after field, found %s", describe(p.tok()))
				}
			} else if p.at(token.Semi) {
				p.next()
			} else {
				p.errorf(p.tok().Pos, "expected field, found %s", describe(p.tok()))
				p.next()
			}
		}
		p.expect(token.RBrace)
	} else {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
	}
	st.Span = p.fin(start)
	return st
}

func (p *parser) parseInterfaceType() ast.Type {
	start := p.next().Pos
	it := &ast.InterfaceType{}
	if p.at(token.LBrace) {
		p.next()
		for !p.at(token.RBrace) && !p.at(token.EOF) {
			switch p.kind() {
			case token.Fn, token.Ident:
				it.Methods = append(it.Methods, p.parseIFaceMethod())
			case token.Semi:
				p.next()
			default:
				p.errorf(p.tok().Pos, "expected interface method, found %s", describe(p.tok()))
				p.next()
			}
		}
		p.expect(token.RBrace)
	} else {
		p.errorf(p.tok().Pos, "expected '{', found %s", describe(p.tok()))
	}
	it.Span = p.fin(start)
	return it
}

func (p *parser) parseTypePostfix(t ast.Type) ast.Type {
	for p.at(token.LBracket) {
		p.next()
		if p.at(token.RBracket) {
			end := p.next().End
			t = &ast.SliceType{Span: ast.Span{Start: t.Pos(), End: end}, Elem: t}
			continue
		}
		save := p.i
		saveErrs := len(p.errs)
		p.silent++
		args, ok := p.parseTypeList()
		ok = ok && p.at(token.RBracket)
		p.silent--
		p.errs = p.errs[:saveErrs]
		if ok {
			end := p.next().End
			t = &ast.GenericType{Span: ast.Span{Start: t.Pos(), End: end}, Base: t, Args: args}
			continue
		}
		p.i = save
		length := p.parseExprNoOr()
		if length == nil {
			return nil
		}
		var end token.Pos
		if p.at(token.RBracket) {
			end = p.next().End
		} else {
			p.errorf(p.tok().Pos, "expected ']', found %s", describe(p.tok()))
			end = p.lastEnd()
		}
		if end.Offset < length.EndPos().Offset {
			end = length.EndPos()
		}
		t = &ast.ArrayType{
			Span: ast.Span{Start: t.Pos(), End: end},
			Elem: t,
			Len:  length,
		}
	}
	return t
}

func (p *parser) parseTypeList() ([]ast.Type, bool) {
	var list []ast.Type
	for {
		t := p.parseType()
		if t == nil {
			return nil, false
		}
		list = append(list, t)
		if p.at(token.Comma) {
			p.next()
			if p.at(token.RBracket) {
				break
			}
			continue
		}
		break
	}
	return list, true
}
