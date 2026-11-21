package lsp

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/bongdo/ink/internal/ast"
	"github.com/bongdo/ink/internal/diag"
	"github.com/bongdo/ink/internal/token"
)

type renameTarget struct {
	decl ast.Node
	name string
}

func (s *server) targetAt(p TextDocumentPositionParams) (*fileInfo, *renameTarget) {
	fi, ok := s.fileAt(p.TextDocument.URI)
	if !ok || fi == nil {
		return nil, nil
	}
	node := s.nodeAt(fi, OffsetAt(fi.src, p.Position))
	if node != nil {
		if ref := refOf(node); ref != nil {
			if target := refTarget(ref); target != nil {
				return fi, target
			}
			if ref.Kind == ast.RefPackage || ref.Kind == ast.RefBuiltin || ref.Go != nil {
				return fi, nil
			}
		}
		if entry, ok := node.(*ast.CompositeEntry); ok && entry.Field != nil && entry.Key != "" {
			return fi, &renameTarget{decl: entry.Field, name: entry.Key}
		}
		if named, ok := node.(*ast.NamedType); ok {
			if named.Variant != nil {
				return fi, &renameTarget{decl: named.Variant, name: named.Variant.Name}
			}
			if named.Decl != nil {
				return fi, &renameTarget{decl: named.Decl, name: named.Name}
			}
			return fi, nil
		}
	}
	if decl, name := s.declAt(fi, OffsetAt(fi.src, p.Position)); decl != nil {
		return fi, &renameTarget{decl: decl, name: name}
	}
	return fi, nil
}

func refTarget(ref *ast.Ref) *renameTarget {
	if ref == nil {
		return nil
	}
	switch ref.Kind {
	case ast.RefLocal, ast.RefParam, ast.RefGlobal, ast.RefConst:
		if ref.Decl != nil {
			return &renameTarget{decl: ref.Decl, name: ref.Name}
		}
	case ast.RefFunc:
		if ref.Func != nil {
			return &renameTarget{decl: ref.Func, name: ref.Name}
		}
	case ast.RefType:
		if ref.Type != nil {
			return &renameTarget{decl: ref.Type, name: ref.Name}
		}
	case ast.RefField:
		if ref.Field != nil {
			return &renameTarget{decl: ref.Field, name: ref.Name}
		}
	case ast.RefEnumVariant:
		if ref.Variant != nil {
			return &renameTarget{decl: ref.Variant, name: ref.Name}
		}
	}
	return nil
}

func (s *server) declAt(fi *fileInfo, offset int) (ast.Node, string) {
	var decl ast.Node
	var name string
	s.inspectFile(fi, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			if nameAt(fi.src, n, n.Name, offset) {
				decl, name = n, n.Name
			}
		case *ast.TypeDecl:
			if nameAt(fi.src, n, n.Name, offset) {
				decl, name = n, n.Name
			}
		case *ast.Field:
			if nameAt(fi.src, n, n.Name, offset) {
				decl, name = n, n.Name
			}
		case *ast.Variant:
			if nameAt(fi.src, n, n.Name, offset) {
				decl, name = n, n.Name
			}
		case *ast.Param:
			if nameAt(fi.src, n, n.Name, offset) {
				decl, name = n, n.Name
			}
		case *ast.TypeParam:
			if nameAt(fi.src, n, n.Name, offset) {
				decl, name = n, n.Name
			}
		case *ast.VarDecl:
			if got, ok := varNameAt(fi.src, n, offset); ok {
				decl, name = n, got
			}
		case *ast.RangeClause:
			if got, ok := namesAt(fi.src, n, n.Names, offset); ok {
				decl, name = n, got
			}
		case *ast.Case:
			if n.Pat != nil {
				names := append([]string(nil), n.Pat.Fields...)
				if n.Pat.Bind != "" {
					names = append(names, n.Pat.Bind)
				}
				if got, ok := namesAt(fi.src, n, names, offset); ok {
					decl, name = n, got
				}
			}
		case *ast.OrExpr:
			if n.Name != "" && nameAt(fi.src, n, n.Name, offset) {
				decl, name = n, n.Name
			}
		}
		return true
	})
	return decl, name
}

func nameAt(src []byte, node ast.Node, name string, offset int) bool {
	if name == "" || node == nil {
		return false
	}
	start, end, ok := findWord(src, node.Pos().Offset, node.EndPos().Offset, name)
	return ok && start <= offset && offset <= end
}

func varNameAt(src []byte, d *ast.VarDecl, offset int) (string, bool) {
	return namesAt(src, d, d.Names, offset)
}

func namesAt(src []byte, node ast.Node, names []string, offset int) (string, bool) {
	start := node.Pos().Offset
	end := node.EndPos().Offset
	for _, name := range names {
		if name == "" || name == "_" {
			continue
		}
		a, b, ok := findWord(src, start, end, name)
		if !ok {
			continue
		}
		if a <= offset && offset <= b {
			return name, true
		}
		start = b
	}
	return "", false
}

func (s *server) references(p ReferenceParams) (any, error) {
	_, target := s.targetAt(p.TextDocumentPositionParams)
	locations := []Location{}
	if target == nil || s.last == nil {
		return locations, nil
	}
	return s.findReferences(target, p.Context.IncludeDeclaration), nil
}

func (s *server) renameConflict(fi *fileInfo, target *renameTarget, name string) bool {
	if fi.prog == nil {
		return false
	}
	pkg := fi.prog.Packages[fi.rel]
	if pkg == nil {
		return false
	}
	switch d := target.decl.(type) {
	case *ast.FuncDecl:
		other := pkg.Funcs[name]
		return other != nil && other != d
	case *ast.TypeDecl:
		other := pkg.Types[name]
		return other != nil && other != d
	case *ast.VarDecl:
		other := pkg.Vars[name]
		return other != nil && other != d
	case *ast.Variant:
		for _, t := range pkg.Types {
			for _, v := range t.Variants {
				if v != d && v.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func (s *server) findReferences(target *renameTarget, includeDecl bool) []Location {
	locations := []Location{}
	if s.last == nil || target == nil {
		return locations
	}
	seen := make(map[string]bool)
	add := func(fi *fileInfo, r Range) {
		key := fi.path + "\x00" + fmt.Sprint(r.Start.Line, ":", r.Start.Character, ":", r.End.Line, ":", r.End.Character)
		if seen[key] {
			return
		}
		seen[key] = true
		locations = append(locations, Location{URI: pathToURI(fi.path), Range: r})
	}
	if includeDecl {
		if fi := s.fileOfNode(target.decl); fi != nil {
			add(fi, nameRange(fi.src, target.decl, target.name))
		}
	}
	paths := make([]string, 0, len(s.last.files))
	for path := range s.last.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fi := s.last.files[path]
		s.inspectFile(fi, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				if refMatches(n.Ref, n.Name, target) {
					add(fi, spanRange(fi.src, n))
				}
			case *ast.SelectorExpr:
				if refMatches(n.Ref, n.Name, target) {
					if r, ok := lastWordRange(fi.src, n.Pos().Offset, n.EndPos().Offset, n.Name); ok {
						add(fi, r)
					}
				}
			case *ast.NamedType:
				if namedTypeMatches(n, target) {
					if r, ok := lastWordRange(fi.src, n.Pos().Offset, n.EndPos().Offset, n.Name); ok {
						add(fi, r)
					}
				}
			case *ast.CompositeEntry:
				if n.Key != "" {
					if decl, ok := target.decl.(*ast.Field); ok && n.Field == decl && n.Key == target.name {
						if start, end, ok := findWord(fi.src, n.Pos().Offset, n.EndPos().Offset, n.Key); ok {
							add(fi, Range{Start: PositionAt(fi.src, start), End: PositionAt(fi.src, end)})
						}
					}
				}
			}
			return true
		})
	}
	sort.SliceStable(locations, func(i, j int) bool {
		if locations[i].URI != locations[j].URI {
			return locations[i].URI < locations[j].URI
		}
		if locations[i].Range.Start.Line != locations[j].Range.Start.Line {
			return locations[i].Range.Start.Line < locations[j].Range.Start.Line
		}
		return locations[i].Range.Start.Character < locations[j].Range.Start.Character
	})
	return locations
}

func refMatches(ref *ast.Ref, name string, target *renameTarget) bool {
	if ref == nil || target == nil || name != target.name {
		return false
	}
	switch decl := target.decl.(type) {
	case *ast.VarDecl:
		return ref.Decl == decl
	case *ast.FuncDecl:
		return ref.Func == decl
	case *ast.TypeDecl:
		return ref.Type == decl
	case *ast.Field:
		return ref.Field == decl
	case *ast.Variant:
		return ref.Variant == decl
	default:
		return ref.Decl == target.decl
	}
}

func namedTypeMatches(n *ast.NamedType, target *renameTarget) bool {
	if n == nil || target == nil || n.Name != target.name {
		return false
	}
	switch decl := target.decl.(type) {
	case *ast.TypeDecl:
		return n.Decl == decl && n.Variant == nil
	case *ast.Variant:
		return n.Variant == decl
	}
	return false
}

func lastWordRange(src []byte, start, end int, word string) (Range, bool) {
	if start < 0 {
		start = 0
	}
	if end > len(src) {
		end = len(src)
	}
	if word == "" || start >= end {
		return Range{}, false
	}
	needle := []byte(word)
	for i := end; i-len(needle) >= start; {
		rel := bytes.LastIndex(src[start:i], needle)
		if rel < 0 {
			return Range{}, false
		}
		a := start + rel
		b := a + len(needle)
		before := a == 0 || !identByte(src[a-1])
		after := b >= len(src) || !identByte(src[b])
		if before && after {
			return Range{Start: PositionAt(src, a), End: PositionAt(src, b)}, true
		}
		i = a + len(needle) - 1
	}
	return Range{}, false
}

func (s *server) prepareRename(p TextDocumentPositionParams) (any, error) {
	_, target := s.targetAt(p)
	if target == nil {
		return nil, nil
	}
	fi := s.fileOfNode(target.decl)
	if fi == nil {
		return nil, nil
	}
	return PrepareRenameResult{Range: nameRange(fi.src, target.decl, target.name), Placeholder: target.name}, nil
}

func (s *server) rename(p RenameParams) (any, error) {
	_, target := s.targetAt(p.TextDocumentPositionParams)
	if target == nil {
		return nil, nil
	}
	if !validIdent(p.NewName) {
		return nil, fmt.Errorf("invalid identifier %q", p.NewName)
	}
	fi := s.fileOfNode(target.decl)
	if fi == nil {
		return nil, nil
	}
	if s.renameConflict(fi, target, p.NewName) {
		return nil, fmt.Errorf("rename would collide with %s", p.NewName)
	}
	locations := s.findReferences(target, true)
	if len(locations) == 0 {
		return nil, nil
	}
	if !exported(p.NewName) {
		for _, location := range locations {
			path, err := uriToPath(location.URI)
			if err != nil {
				continue
			}
			if other := s.last.files[path]; other != nil && other.rel != fi.rel {
				return nil, fmt.Errorf("cannot rename exported symbol %s to unexported %s", target.name, p.NewName)
			}
		}
	}
	changes := make(map[string][]TextEdit)
	for _, location := range locations {
		changes[location.URI] = append(changes[location.URI], TextEdit{Range: location.Range, NewText: p.NewName})
	}
	return WorkspaceEdit{Changes: changes}, nil
}

func validIdent(name string) bool {
	if name == "" || name == "_" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	if token.Lookup(name) != token.Ident || token.GoReserved(name) {
		return false
	}
	for _, builtin := range builtinTypes {
		if name == builtin {
			return false
		}
	}
	return true
}

func (s *server) workspaceSymbols(p WorkspaceSymbolParams) (any, error) {
	symbols := []SymbolInformation{}
	if s.last == nil {
		return symbols, nil
	}
	query := strings.ToLower(p.Query)
	matches := func(name, container string) bool {
		if query == "" {
			return true
		}
		return strings.Contains(strings.ToLower(name), query) || strings.Contains(strings.ToLower(container), query)
	}
	paths := make([]string, 0, len(s.last.files))
	for path := range s.last.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		fi := s.last.files[path]
		if fi.file == nil {
			continue
		}
		add := func(name, container string, kind int, node ast.Node) {
			if name == "" || !matches(name, container) {
				return
			}
			symbols = append(symbols, SymbolInformation{
				Name:          name,
				Kind:          kind,
				Location:      Location{URI: pathToURI(fi.path), Range: nameRange(fi.src, node, name)},
				ContainerName: container,
			})
		}
		for _, decl := range fi.file.Decls {
			switch decl := decl.(type) {
			case *ast.TypeDecl:
				add(decl.Name, "", typeSymbolKind(decl), decl)
				if decl.Kind == ast.TypeEnum {
					for _, variant := range decl.Variants {
						add(variant.Name, decl.Name, dskEnumMember, variant)
					}
				}
				if decl.Kind == ast.TypeStruct {
					for _, method := range decl.Methods {
						add(method.Name, decl.Name, dskMethod, method)
					}
				}
				if decl.Kind == ast.TypeInterface {
					for _, method := range decl.IFace {
						add(method.Name, decl.Name, dskMethod, method)
					}
				}
			case *ast.FuncDecl:
				add(decl.Name, "", dskFunction, decl)
			case *ast.VarDecl:
				for _, name := range decl.Names {
					kind := dskVariable
					if decl.Const {
						kind = dskConstant
					}
					add(name, "", kind, decl)
				}
			case *ast.TestDecl:
				add(decl.Name, "", dskFunction, decl)
			}
		}
	}
	return symbols, nil
}

func (s *server) codeActions(p CodeActionParams) (any, error) {
	fi, ok := s.fileAt(p.TextDocument.URI)
	if !ok {
		return nil, errInvalidParams
	}
	actions := []CodeAction{}
	if fi == nil {
		return actions, nil
	}
	for _, diagnostic := range p.Context.Diagnostics {
		code := diagnostic.Code
		if code == "" {
			code = diag.Classify(diagnostic.Message).Code
		}
		title, edits := s.codeActionFix(fi, diagnostic, code)
		if len(edits) == 0 {
			continue
		}
		diagnostics := []Diagnostic{diagnostic}
		actions = append(actions, CodeAction{
			Title:       title,
			Kind:        "quickfix",
			Diagnostics: diagnostics,
			Edit:        &WorkspaceEdit{Changes: map[string][]TextEdit{p.TextDocument.URI: edits}},
			IsPreferred: true,
		})
	}
	return actions, nil
}

func (s *server) codeActionFix(fi *fileInfo, diagnostic Diagnostic, code string) (string, []TextEdit) {
	switch code {
	case "const-local":
		return "Replace const with var", replaceConst(fi, diagnostic)
	case "decl-equals":
		return "Remove '='", removeDeclEquals(fi, diagnostic)
	case "iface-fn":
		return "Remove 'fn' from interface method", removeIfaceFn(fi, diagnostic)
	case "import-simple":
		return "Remove quotes from import", fixImportSimple(fi, diagnostic)
	case "import-alias":
		return "Add import alias", fixImportAlias(fi, diagnostic)
	case "err-manual":
		return "Use or fail", fixErrManual(fi, diagnostic)
	case "nil-untyped":
		return "Add explicit nil type", fixNilUntyped(fi, diagnostic)
	}
	return "", nil
}

func diagnosticOffset(fi *fileInfo, diagnostic Diagnostic) (int, bool) {
	offset := OffsetAt(fi.src, diagnostic.Range.Start)
	if offset < 0 || offset > len(fi.src) {
		return 0, false
	}
	return offset, true
}

func replaceConst(fi *fileInfo, diagnostic Diagnostic) []TextEdit {
	offset, ok := diagnosticOffset(fi, diagnostic)
	if !ok {
		return nil
	}
	var found *ast.VarDecl
	ast.Inspect(fi.file, func(n ast.Node) bool {
		v, ok := n.(*ast.VarDecl)
		if ok && v.Const && (v.Pos().Offset == offset || containsNode(v, offset)) {
			if found == nil || v.EndPos().Offset-v.Pos().Offset < found.EndPos().Offset-found.Pos().Offset {
				found = v
			}
		}
		return true
	})
	if found == nil {
		return nil
	}
	start := found.Pos().Offset
	end := start + len("const")
	if start < 0 || end > len(fi.src) || string(fi.src[start:end]) != "const" {
		return nil
	}
	return []TextEdit{{Range: Range{Start: PositionAt(fi.src, start), End: PositionAt(fi.src, end)}, NewText: "var"}}
}

func removeDeclEquals(fi *fileInfo, diagnostic Diagnostic) []TextEdit {
	offset, ok := diagnosticOffset(fi, diagnostic)
	if !ok || offset >= len(fi.src) || fi.src[offset] != '=' {
		return nil
	}
	start := offset
	end := offset + 1
	prevSpace := offset > 0 && (fi.src[offset-1] == ' ' || fi.src[offset-1] == '\t')
	nextSpace := end < len(fi.src) && (fi.src[end] == ' ' || fi.src[end] == '\t')
	text := ""
	switch {
	case prevSpace && nextSpace:
		end++
	case !prevSpace && !nextSpace:
		text = " "
	}
	return []TextEdit{{Range: Range{Start: PositionAt(fi.src, start), End: PositionAt(fi.src, end)}, NewText: text}}
}

func removeIfaceFn(fi *fileInfo, diagnostic Diagnostic) []TextEdit {
	offset, ok := diagnosticOffset(fi, diagnostic)
	if !ok || offset+2 > len(fi.src) || string(fi.src[offset:offset+2]) != "fn" {
		return nil
	}
	end := offset + 2
	if end < len(fi.src) && (fi.src[end] == ' ' || fi.src[end] == '\t') {
		end++
	}
	return []TextEdit{{Range: Range{Start: PositionAt(fi.src, offset), End: PositionAt(fi.src, end)}, NewText: ""}}
}

func useAt(fi *fileInfo, offset int) *ast.Use {
	for _, use := range fi.file.Uses {
		if containsNode(use, offset) {
			return use
		}
	}
	return nil
}

func quotedRange(src []byte, use *ast.Use) (int, int, bool) {
	start := use.Pos().Offset
	end := use.EndPos().Offset
	for i := start; i < end; i++ {
		if src[i] != '"' && src[i] != '`' {
			continue
		}
		quote := src[i]
		for j := i + 1; j < end; j++ {
			if quote != '`' && src[j] == '\\' {
				j++
				continue
			}
			if src[j] == quote {
				return i, j + 1, true
			}
		}
		break
	}
	return 0, 0, false
}

func fixImportSimple(fi *fileInfo, diagnostic Diagnostic) []TextEdit {
	offset, ok := diagnosticOffset(fi, diagnostic)
	if !ok {
		return nil
	}
	use := useAt(fi, offset)
	if use == nil || use.Local || use.Path == "" {
		return nil
	}
	start, end, ok := quotedRange(fi.src, use)
	if !ok {
		return nil
	}
	return []TextEdit{{Range: Range{Start: PositionAt(fi.src, start), End: PositionAt(fi.src, end)}, NewText: use.Path}}
}

func fixImportAlias(fi *fileInfo, diagnostic Diagnostic) []TextEdit {
	offset, ok := diagnosticOffset(fi, diagnostic)
	if !ok {
		return nil
	}
	use := useAt(fi, offset)
	if use == nil || use.Local || use.Path == "" {
		return nil
	}
	end, ok := pathEnd(fi, use)
	if !ok {
		return nil
	}
	if hasAliasWord(fi.src, end, use.EndPos().Offset) {
		return nil
	}
	name := importAlias(use.Path)
	if !validIdent(name) {
		return nil
	}
	return []TextEdit{{Range: Range{Start: PositionAt(fi.src, end), End: PositionAt(fi.src, end)}, NewText: " as " + name}}
}

func pathEnd(fi *fileInfo, use *ast.Use) (int, bool) {
	start := use.Pos().Offset + len("use")
	if start < 0 || start > len(fi.src) || use.EndPos().Offset > len(fi.src) {
		return 0, false
	}
	i := strings.Index(string(fi.src[start:use.EndPos().Offset]), use.Path)
	if i < 0 {
		return 0, false
	}
	return start + i + len(use.Path), true
}

func hasAliasWord(src []byte, start, end int) bool {
	_, _, ok := findWord(src, start, end, "as")
	return ok
}

func importAlias(path string) string {
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if part == "" || versionPathSegment(part) {
			continue
		}
		var b strings.Builder
		for _, r := range part {
			if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(r)
			} else {
				b.WriteRune('_')
			}
		}
		name := b.String()
		for name != "" {
			r := []rune(name)[0]
			if r == '_' || unicode.IsLetter(r) {
				break
			}
			name = string([]rune(name)[1:])
		}
		if name == "" {
			return ""
		}
		return name
	}
	return ""
}

func versionPathSegment(segment string) bool {
	if len(segment) < 2 || segment[0] != 'v' {
		return false
	}
	for _, r := range segment[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	n := 0
	for _, r := range segment[1:] {
		n = n*10 + int(r-'0')
	}
	return n >= 2
}

func fixErrManual(fi *fileInfo, diagnostic Diagnostic) []TextEdit {
	offset, ok := diagnosticOffset(fi, diagnostic)
	if !ok {
		return nil
	}
	var ifStmt *ast.IfStmt
	ast.Inspect(fi.file, func(n ast.Node) bool {
		s, ok := n.(*ast.IfStmt)
		if ok && (s.Pos().Offset == offset || containsNode(s, offset)) {
			if ifStmt == nil || s.EndPos().Offset-s.Pos().Offset < ifStmt.EndPos().Offset-ifStmt.Pos().Offset {
				ifStmt = s
			}
		}
		return true
	})
	if ifStmt == nil || ifStmt.Then == nil || ifStmt.Else != nil || len(ifStmt.Then.Stmts) != 1 {
		return nil
	}
	err := errGuardIdent(ifStmt.Cond)
	if err == nil {
		return nil
	}
	ret, ok := ifStmt.Then.Stmts[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil
	}
	retID, ok := unparen(ret.Results[0]).(*ast.Ident)
	if !ok || retID.Name != err.Name {
		return nil
	}
	var block *ast.Block
	var index int
	ast.Inspect(fi.file, func(n ast.Node) bool {
		b, ok := n.(*ast.Block)
		if !ok || block != nil {
			return true
		}
		for i, stmt := range b.Stmts {
			if stmt == ifStmt && i > 0 {
				block, index = b, i
				return true
			}
		}
		return true
	})
	if block == nil || index == 0 {
		return nil
	}
	prev := block.Stmts[index-1]
	var call *ast.CallExpr
	var keep []string
	var typed bool
	var varType ast.Type
	switch stmt := prev.(type) {
	case *ast.DeclStmt:
		if stmt.Decl == nil || len(stmt.Decl.Values) != 1 {
			return nil
		}
		call = callOf(stmt.Decl.Values[0])
		if call == nil || len(stmt.Decl.Names) == 0 || stmt.Decl.Names[len(stmt.Decl.Names)-1] != err.Name {
			return nil
		}
		if call.Info == nil || call.Info.Results+1 != len(stmt.Decl.Names) {
			return nil
		}
		keep = append(keep, stmt.Decl.Names[:len(stmt.Decl.Names)-1]...)
		varType = stmt.Decl.Type
		typed = varType != nil
	case *ast.AssignStmt:
		if stmt.Op != token.Assign || len(stmt.Rhs) != 1 || len(stmt.Lhs) == 0 {
			return nil
		}
		call = callOf(stmt.Rhs[0])
		if call == nil || call.Info == nil || call.Info.Results+1 != len(stmt.Lhs) {
			return nil
		}
		last, ok := unparen(stmt.Lhs[len(stmt.Lhs)-1]).(*ast.Ident)
		if !ok || last.Name != err.Name {
			return nil
		}
		for _, expr := range stmt.Lhs[:len(stmt.Lhs)-1] {
			keep = append(keep, string(fi.src[expr.Pos().Offset:expr.EndPos().Offset]))
		}
	default:
		return nil
	}
	if call == nil || call.Info == nil || !call.Info.HasError {
		return nil
	}
	if fn := enclosingFunc(fi.file, ifStmt); fn == nil || !fn.Err {
		return nil
	}
	if typed && len(keep) != 1 {
		return nil
	}
	var b strings.Builder
	if len(keep) > 0 {
		if _, isDecl := prev.(*ast.DeclStmt); isDecl {
			b.WriteString("var ")
		}
		b.WriteString(strings.Join(keep, ", "))
		if typed {
			b.WriteString(": ")
			b.WriteString(formatType(varType))
		}
		b.WriteString(" = ")
	}
	b.WriteString(string(fi.src[call.Pos().Offset:call.EndPos().Offset]))
	b.WriteString(" or fail;")
	start := prev.Pos().Offset
	end := ifStmt.EndPos().Offset
	if start < 0 || end > len(fi.src) || start >= end {
		return nil
	}
	return []TextEdit{{Range: Range{Start: PositionAt(fi.src, start), End: PositionAt(fi.src, end)}, NewText: b.String()}}
}

func errGuardIdent(expr ast.Expr) *ast.Ident {
	bin, ok := unparen(expr).(*ast.BinaryExpr)
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

func unparen(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

func callOf(expr ast.Expr) *ast.CallExpr {
	call, _ := unparen(expr).(*ast.CallExpr)
	return call
}

func fixNilUntyped(fi *fileInfo, diagnostic Diagnostic) []TextEdit {
	offset, ok := diagnosticOffset(fi, diagnostic)
	if !ok {
		return nil
	}
	var nilLit *ast.NilLit
	ast.Inspect(fi.file, func(n ast.Node) bool {
		lit, ok := n.(*ast.NilLit)
		if ok && (lit.Pos().Offset == offset || containsNode(lit, offset)) && lit.Type == nil {
			nilLit = lit
		}
		return true
	})
	if nilLit == nil {
		return nil
	}
	typ := nilExpectedType(fi.file, nilLit)
	if typ == nil {
		return nil
	}
	text := formatType(typ)
	if text == "" {
		return nil
	}
	r := spanRange(fi.src, nilLit)
	return []TextEdit{{Range: r, NewText: "nil[" + text + "]"}}
}

func nilExpectedType(file *ast.File, lit *ast.NilLit) ast.Type {
	var typ ast.Type
	var bestWidth int
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.VarDecl:
			if n.Type != nil && n.Type.Pos().Offset <= lit.Pos().Offset && lit.EndPos().Offset <= n.EndPos().Offset {
				if width := n.EndPos().Offset - n.Pos().Offset; bestWidth == 0 || width < bestWidth {
					typ, bestWidth = n.Type, width
				}
			}
		case *ast.ReturnStmt:
			if n.Pos().Offset <= lit.Pos().Offset && lit.EndPos().Offset <= n.EndPos().Offset {
				if fn := enclosingFunc(file, n); fn != nil {
					for i, result := range n.Results {
						if result == lit && i < len(fn.Results) {
							typ = fn.Results[i]
							bestWidth = n.EndPos().Offset - n.Pos().Offset
						}
					}
				}
			}
		case *ast.CallExpr:
			if n.Info == nil || !(n.Pos().Offset <= lit.Pos().Offset && lit.EndPos().Offset <= n.EndPos().Offset) {
				return true
			}
			for i, arg := range n.Args {
				if arg == lit && i < len(n.Info.ParamTypes) {
					if width := n.EndPos().Offset - n.Pos().Offset; bestWidth == 0 || width < bestWidth {
						typ, bestWidth = n.Info.ParamTypes[i], width
					}
				}
			}
		}
		return true
	})
	return typ
}

func enclosingFunc(file *ast.File, target ast.Node) *ast.FuncDecl {
	var found *ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if ok && fn.Pos().Offset <= target.Pos().Offset && target.EndPos().Offset <= fn.EndPos().Offset {
			if found == nil || fn.EndPos().Offset-fn.Pos().Offset < found.EndPos().Offset-found.Pos().Offset {
				found = fn
			}
		}
		return true
	})
	return found
}
