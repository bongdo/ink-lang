package uml

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/bongdo/ink-lang/internal/ast"
)

type generator struct {
	root     string
	parsed   map[string][]*ast.File
	order    []string
	types    map[string]map[string]*ast.TypeDecl
	aliases  map[*ast.TypeDecl]string
	uses     map[string]map[string]string
	pkgAlias map[string]string
	rels     []string
	seen     map[string]bool
	next     int
}

func PlantUML(parsed map[string][]*ast.File, root string) string {
	g := &generator{
		root:     root,
		parsed:   parsed,
		types:    make(map[string]map[string]*ast.TypeDecl),
		aliases:  make(map[*ast.TypeDecl]string),
		uses:     make(map[string]map[string]string),
		pkgAlias: make(map[string]string),
		seen:     make(map[string]bool),
	}
	g.collect()
	g.collectRelations()

	var b strings.Builder
	b.WriteString("@startuml\n")
	for _, dir := range g.order {
		g.packageDecls(&b, dir)
	}
	for _, rel := range g.rels {
		b.WriteString(rel)
		b.WriteString("\n")
	}
	b.WriteString("@enduml\n")
	return b.String()
}

func (g *generator) collect() {
	g.order = make([]string, 0, len(g.parsed))
	for dir := range g.parsed {
		g.order = append(g.order, dir)
	}
	sort.Strings(g.order)

	for i, dir := range g.order {
		g.pkgAlias[dir] = fmt.Sprintf("p%d", i)
		g.types[dir] = make(map[string]*ast.TypeDecl)
		g.uses[dir] = make(map[string]string)
		for _, f := range g.parsed[dir] {
			for _, u := range f.Uses {
				if !u.Local {
					continue
				}
				alias := u.Alias
				if alias == "" {
					alias = path.Base(u.Path)
				}
				g.uses[dir][alias] = path.Clean(path.Join(dir, u.Path))
			}
			for _, d := range f.Decls {
				td, ok := d.(*ast.TypeDecl)
				if !ok {
					continue
				}
				g.types[dir][td.Name] = td
				g.aliases[td] = fmt.Sprintf("c%d", g.next)
				g.next++
			}
		}
	}
}

func (g *generator) packageDecls(b *strings.Builder, dir string) {
	var body strings.Builder
	for _, f := range g.parsed[dir] {
		var funcs []*ast.FuncDecl
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.TypeDecl:
				g.typeDecl(&body, d)
			case *ast.FuncDecl:
				funcs = append(funcs, d)
			}
		}
		if len(funcs) > 0 {
			g.fileDecl(&body, f, funcs)
		}
	}
	if body.Len() == 0 {
		return
	}
	name := dir
	if name == "." {
		name = g.root
	}
	fmt.Fprintf(b, "package \"%s\" as %s {\n", name, g.pkgAlias[dir])
	b.WriteString("  " + strings.ReplaceAll(strings.TrimSuffix(body.String(), "\n"), "\n", "\n  ") + "\n")
	b.WriteString("}\n")
}

func (g *generator) typeDecl(b *strings.Builder, td *ast.TypeDecl) {
	alias := g.aliases[td]
	switch td.Kind {
	case ast.TypeStruct:
		fmt.Fprintf(b, "class \"%s\" as %s {\n", typeName(td), alias)
		for _, f := range td.Fields {
			fmt.Fprintf(b, "    +%s: %s\n", f.Name, typeString(f.Type))
		}
		for _, m := range td.Methods {
			fmt.Fprintf(b, "    +%s\n", methodString(m.Name, m.TypeParams, m.Params, m.Results, m.Err))
		}
		b.WriteString("}\n")
	case ast.TypeEnum:
		fmt.Fprintf(b, "enum \"%s\" as %s {\n", typeName(td), alias)
		for _, v := range td.Variants {
			fmt.Fprintf(b, "    %s\n", variantString(v))
		}
		b.WriteString("}\n")
	case ast.TypeInterface:
		fmt.Fprintf(b, "interface \"%s\" as %s {\n", typeName(td), alias)
		for _, m := range td.IFace {
			fmt.Fprintf(b, "    +%s\n", methodString(m.Name, nil, m.Params, m.Results, m.Err))
		}
		b.WriteString("}\n")
	default:
		fmt.Fprintf(b, "class \"%s = %s\" as %s <<alias>>\n", td.Name, typeString(td.Target), alias)
	}
}

func (g *generator) fileDecl(b *strings.Builder, f *ast.File, funcs []*ast.FuncDecl) {
	alias := fmt.Sprintf("c%d", g.next)
	g.next++
	fmt.Fprintf(b, "class \"%s\" as %s <<file>> {\n", path.Base(f.Path), alias)
	for _, fn := range funcs {
		fmt.Fprintf(b, "    +%s\n", methodString(fn.Name, fn.TypeParams, fn.Params, fn.Results, fn.Err))
	}
	b.WriteString("}\n")
}

func (g *generator) collectRelations() {
	for _, dir := range g.order {
		targets := make([]string, 0, len(g.uses[dir]))
		for _, target := range g.uses[dir] {
			targets = append(targets, target)
		}
		sort.Strings(targets)
		for _, target := range targets {
			if target == dir || g.parsed[target] == nil {
				continue
			}
			g.addLine(g.pkgAlias[dir] + " ..> " + g.pkgAlias[target])
		}
	}
	for _, dir := range g.order {
		for _, f := range g.parsed[dir] {
			for _, d := range f.Decls {
				td, ok := d.(*ast.TypeDecl)
				if !ok {
					continue
				}
				if td.Kind == ast.TypeAlias {
					g.addRelation(dir, td, td.Target)
					continue
				}
				for _, fld := range td.Fields {
					g.addRelation(dir, td, fld.Type)
				}
				for _, v := range td.Variants {
					for _, fld := range v.Fields {
						g.addRelation(dir, td, fld.Type)
					}
				}
			}
		}
	}
}

func (g *generator) addRelation(dir string, td *ast.TypeDecl, t ast.Type) {
	src := g.aliases[td]
	for _, dst := range g.typeRefs(dir, t) {
		if dst == src {
			continue
		}
		g.addLine(src + " --> " + dst)
	}
}

func (g *generator) addLine(line string) {
	if g.seen[line] {
		return
	}
	g.seen[line] = true
	g.rels = append(g.rels, line)
}

func (g *generator) typeRefs(dir string, t ast.Type) []string {
	switch t := t.(type) {
	case *ast.NamedType:
		if alias := g.typeRef(dir, t); alias != "" {
			return []string{alias}
		}
	case *ast.SliceType:
		return g.typeRefs(dir, t.Elem)
	case *ast.ArrayType:
		return g.typeRefs(dir, t.Elem)
	case *ast.PointerType:
		return g.typeRefs(dir, t.Elem)
	case *ast.MapType:
		return append(g.typeRefs(dir, t.Key), g.typeRefs(dir, t.Value)...)
	case *ast.GenericType:
		refs := g.typeRefs(dir, t.Base)
		for _, arg := range t.Args {
			refs = append(refs, g.typeRefs(dir, arg)...)
		}
		return refs
	}
	return nil
}

func (g *generator) typeRef(dir string, n *ast.NamedType) string {
	target := dir
	if n.Package != "" {
		pkg, ok := g.uses[dir][n.Package]
		if !ok {
			return ""
		}
		target = pkg
	}
	td := g.types[target][n.Name]
	if td == nil {
		return ""
	}
	return g.aliases[td]
}

func typeName(td *ast.TypeDecl) string {
	if len(td.TypeParams) == 0 {
		return td.Name
	}
	var b strings.Builder
	b.WriteString(td.Name)
	b.WriteString("[")
	for i, p := range td.TypeParams {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
	}
	b.WriteString("]")
	return b.String()
}

func variantString(v *ast.Variant) string {
	if len(v.Fields) == 0 {
		return v.Name
	}
	var b strings.Builder
	b.WriteString(v.Name)
	b.WriteString("(")
	for i, f := range v.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(f.Name)
		b.WriteString(": ")
		b.WriteString(typeString(f.Type))
	}
	b.WriteString(")")
	return b.String()
}

func methodString(name string, tparams []*ast.TypeParam, params []*ast.Param, results []ast.Type, err bool) string {
	var b strings.Builder
	b.WriteString(name)
	if len(tparams) > 0 {
		b.WriteString("[")
		for i, p := range tparams {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Name)
		}
		b.WriteString("]")
	}
	b.WriteString("(")
	for i, p := range params {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name)
		b.WriteString(": ")
		b.WriteString(typeString(p.Type))
	}
	b.WriteString(")")
	r := resultList(results, err)
	if r == "" {
		r = "void"
	} else if r == "!" {
		r = "void!"
	}
	b.WriteString(": ")
	b.WriteString(r)
	return b.String()
}

func resultList(results []ast.Type, err bool) string {
	var s string
	switch len(results) {
	case 0:
	case 1:
		s = typeString(results[0])
	default:
		var b strings.Builder
		b.WriteString("(")
		for i, t := range results {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(typeString(t))
		}
		b.WriteString(")")
		s = b.String()
	}
	if err {
		s += "!"
	}
	return s
}

func typeString(t ast.Type) string {
	switch t := t.(type) {
	case *ast.NamedType:
		if t.Package != "" {
			return t.Package + "." + t.Name
		}
		return t.Name
	case *ast.SliceType:
		return typeString(t.Elem) + "[]"
	case *ast.ArrayType:
		return typeString(t.Elem) + "[" + exprString(t.Len) + "]"
	case *ast.GenericType:
		var b strings.Builder
		b.WriteString(typeString(t.Base))
		b.WriteString("[")
		for i, a := range t.Args {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(typeString(a))
		}
		b.WriteString("]")
		return b.String()
	case *ast.MapType:
		return "map[" + typeString(t.Key) + ", " + typeString(t.Value) + "]"
	case *ast.PointerType:
		return "*" + typeString(t.Elem)
	case *ast.FuncType:
		var b strings.Builder
		b.WriteString("fn(")
		for i, p := range t.Params {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(typeString(p))
		}
		b.WriteString(")")
		if r := resultList(t.Results, t.Err); r != "" {
			b.WriteString(" ")
			b.WriteString(r)
		}
		return b.String()
	case *ast.InterfaceType:
		return "interface"
	case *ast.StructType:
		return "struct"
	case *ast.GoType:
		return t.Text
	}
	return "?"
}

func exprString(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.BasicLit:
		return e.Value
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Name
	case *ast.ParenExpr:
		return "(" + exprString(e.X) + ")"
	}
	return "N"
}
