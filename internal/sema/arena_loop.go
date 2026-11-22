package sema

import (
	"go/types"
	"runtime"
	"strconv"

	"github.com/bongdo/ink/internal/ast"
	"github.com/bongdo/ink/internal/token"
)

// shortcut: mirrors Go's userArenaChunkBytesMax; update if the runtime changes it.
const arenaChunkBytes = 8 << 20

func (c *checker) warnArenaLoop(s *ast.ArenaStmt) {
	if c.loopDepth == 0 || c.inPar {
		return
	}
	size, ok := c.arenaBytes(s.Body)
	if !ok || size <= 0 {
		return
	}
	chunks := (size + arenaChunkBytes - 1) / arenaChunkBytes
	if chunks <= 1 {
		c.p.warnf(c.fc.file.Path, s.Pos(), "arena in a loop allocates %d bytes per iteration; allocate the buffer before the loop and reuse it", size)
		return
	}
	c.p.warnf(c.fc.file.Path, s.Pos(), "arena in a loop allocates %d bytes per iteration over %d chunks; Go reuses a freed chunk only after a GC", size, chunks)
}

func (c *checker) arenaBytes(body *ast.Block) (int64, bool) {
	if body == nil {
		return 0, false
	}
	var total int64
	known := true
	ast.Inspect(body, func(n ast.Node) bool {
		if !known {
			return false
		}
		switch n := n.(type) {
		case *ast.ArenaStmt:
			return false
		case *ast.ForStmt:
			if blockAllocates(n.Body) {
				known = false
			}
			return false
		case *ast.CallExpr:
			if n.Info == nil || n.Info.Kind != ast.CallBuiltin || (n.Info.Builtin != "new" && n.Info.Builtin != "make") {
				return true
			}
			size, ok := c.allocBytes(n)
			if !ok {
				known = false
				return false
			}
			total += size
		}
		return true
	})
	return total, known
}

func blockAllocates(body *ast.Block) bool {
	if body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.ArenaStmt:
			return false
		case *ast.CallExpr:
			if n.Info != nil && n.Info.Kind == ast.CallBuiltin && (n.Info.Builtin == "new" || n.Info.Builtin == "make") {
				found = true
			}
		}
		return true
	})
	return found
}

func (c *checker) allocBytes(call *ast.CallExpr) (int64, bool) {
	if len(call.TypeArgs) != 1 {
		return 0, false
	}
	elem := c.sizeOf(call.TypeArgs[0])
	if elem <= 0 {
		return 0, false
	}
	if call.Info.Builtin == "new" {
		return elem, true
	}
	if len(call.Args) == 0 {
		return 0, false
	}
	count := len(call.Args) - 1
	n, ok := c.constInt(call.Args[count])
	if !ok || n < 0 {
		return 0, false
	}
	return elem * n, true
}

func (c *checker) sizeOf(t ast.Type) int64 {
	if gt := c.goTypeOf(t); gt != nil {
		if n := c.goSize(gt); n > 0 {
			return n
		}
	}
	switch t := t.(type) {
	case *ast.NamedType:
		if t.Package == "" {
			if n, ok := builtinSize[t.Name]; ok {
				return n
			}
		}
		if t.Decl != nil && t.Decl.Kind == ast.TypeAlias && t.Decl.Target != nil {
			return c.sizeOf(t.Decl.Target)
		}
		if t.Decl != nil && t.Decl.Kind == ast.TypeStruct {
			var total, align int64 = 0, 1
			for _, f := range t.Decl.Fields {
				size := c.sizeOf(f.Type)
				if size <= 0 {
					return 0
				}
				fa := c.alignOf(f.Type)
				total = roundUp(total, fa) + size
				if fa > align {
					align = fa
				}
			}
			return roundUp(total, align)
		}
	case *ast.ArrayType:
		n, ok := c.constInt(t.Len)
		if !ok {
			return 0
		}
		size := c.sizeOf(t.Elem)
		if size <= 0 {
			return 0
		}
		return n * size
	case *ast.PointerType:
		return c.wordSize()
	case *ast.SliceType:
		return 3 * c.wordSize()
	case *ast.MapType:
		return c.wordSize()
	case *ast.FuncType:
		return c.wordSize()
	}
	return 0
}

func (c *checker) alignOf(t ast.Type) int64 {
	word := c.wordSize()
	if word <= 0 {
		word = 8
	}
	if gt := c.goTypeOf(t); gt != nil {
		sizes := types.SizesFor("gc", runtime.GOARCH)
		if sizes == nil {
			sizes = types.SizesFor("gc", "amd64")
		}
		if sizes != nil {
			if a := sizes.Alignof(gt); a > 0 {
				return int64(a)
			}
		}
	}
	switch t := t.(type) {
	case *ast.ArrayType:
		return c.alignOf(t.Elem)
	case *ast.NamedType:
		if t.Decl != nil && t.Decl.Kind == ast.TypeStruct {
			var align int64 = 1
			for _, f := range t.Decl.Fields {
				if a := c.alignOf(f.Type); a > align {
					align = a
				}
			}
			if align > word {
				return word
			}
			return align
		}
		if t.Package == "" {
			if _, ok := builtinSize[t.Name]; ok && builtinSize[t.Name] > word {
				return word
			}
		}
	}
	if size := c.sizeOf(t); size > 0 && size < word {
		return size
	}
	return word
}

func (c *checker) wordSize() int64 {
	sizes := types.SizesFor("gc", runtime.GOARCH)
	if sizes == nil {
		sizes = types.SizesFor("gc", "amd64")
	}
	if sizes == nil {
		return 8
	}
	return sizes.Sizeof(types.Typ[types.Uintptr])
}

func (c *checker) goSize(t types.Type) int64 {
	sizes := types.SizesFor("gc", runtime.GOARCH)
	if sizes == nil {
		sizes = types.SizesFor("gc", "amd64")
	}
	if sizes == nil {
		return 0
	}
	return sizes.Sizeof(t)
}

func roundUp(n, align int64) int64 {
	if align <= 1 {
		return n
	}
	return (n + align - 1) / align * align
}

var builtinSize = map[string]int64{
	"bool": 1, "u8": 1, "i8": 1, "byte": 1,
	"i16": 2, "u16": 2,
	"i32": 4, "u32": 4, "f32": 4, "rune": 4,
	"i64": 8, "u64": 8, "f64": 8, "int": 8, "uint": 8, "uintptr": 8,
	"str": 16, "any": 16, "error": 16,
}

func (c *checker) constInt(e ast.Expr) (int64, bool) {
	switch e := unparen(e).(type) {
	case *ast.BasicLit:
		if e.Kind != token.Int {
			return 0, false
		}
		v, err := strconv.ParseInt(e.Value, 0, 64)
		if err != nil {
			return 0, false
		}
		return v, true
	case *ast.Ident:
		sym := c.lookup(e.Name)
		if sym == nil {
			return 0, false
		}
		decl, ok := sym.decl.(*ast.VarDecl)
		if !ok || !decl.Const || len(decl.Values) != 1 {
			return 0, false
		}
		return c.constInt(decl.Values[0])
	case *ast.UnaryExpr:
		switch e.Op {
		case token.Minus:
			v, ok := c.constInt(e.X)
			return -v, ok
		case token.Plus:
			return c.constInt(e.X)
		}
	case *ast.BinaryExpr:
		x, ok := c.constInt(e.X)
		if !ok {
			return 0, false
		}
		y, ok := c.constInt(e.Y)
		if !ok {
			return 0, false
		}
		switch e.Op {
		case token.Plus:
			return x + y, true
		case token.Minus:
			return x - y, true
		case token.Star:
			return x * y, true
		case token.Slash:
			if y != 0 {
				return x / y, true
			}
		case token.Percent:
			if y != 0 {
				return x % y, true
			}
		case token.Shl:
			if y >= 0 && y < 63 {
				return x << uint(y), true
			}
		case token.Shr:
			if y >= 0 && y < 63 {
				return x >> uint(y), true
			}
		}
	}
	return 0, false
}
