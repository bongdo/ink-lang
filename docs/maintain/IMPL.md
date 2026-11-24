# Ink — compiler implementation notes

Internal contract between packages. Not user documentation.

## Pipeline

```
driver → parser → sema → gen (Go source) → go build / go test
                     ↘ typecheck (check, LSP)
                     ↘ format
                     ↘ lsp
```

## Package APIs

### `internal/lexer`

```go
type Comment struct { Pos, End token.Pos; Text string; Block bool }
type Error struct { Pos token.Pos; Msg string }
func Lex(src []byte) ([]token.Token, []Comment, []Error)
```

### `internal/parser`

```go
type Error struct { Pos token.Pos; Msg string }
func Parse(path string, src []byte) (*ast.File, []Error)
```

`ast.File.Comments` is filled. `ast.File.Path` is the absolute path.
Every node has a valid `Span`; `End` is exclusive.
The parser does not resolve names and does not reject Go reserved words.

### `internal/sema`

```go
type Error struct { Path string; Pos token.Pos; Msg string; Warning bool }

type GoPkg struct {
    Path  string
    Name  string
    Types *types.Package
}

type Import struct {
    Alias string
    Path  string
    Local *Package
    Go    *GoPkg
}

type Package struct {
    Rel     string
    Dir     string
    Name    string
    IsMain  bool
    Files   []*ast.File
    Imports map[string]*Import
    Types   map[string]*ast.TypeDecl
    Funcs   map[string]*ast.FuncDecl
    Vars    map[string]*ast.VarDecl
    Decls   []ast.Decl
    GoImports []string
}

type Program struct {
    Root     string
    Module   string
    Packages map[string]*Package
    Main     *Package
    Errors   []Error
}

type Options struct {
    Root   string
    Module string
}

func Check(parsed map[string][]*ast.File, opt Options) *Program
```

`parsed` keys are slash-separated directories relative to the root; root is `"."`.
`sema` loads Go packages lazily through `go/packages` with `Dir = Root`.
The driver guarantees `go.mod` exists and Go imports are resolvable before
`Check` is called; sema returns an error if they are not.

The parser accepts only the canonical declaration forms (`type X struct`,
`type X enum`, `type X interface`, `fn f() R { ... }`); `=` before a body is a
parse error. The formatter prints the canonical form. Aliases always require
`=` (`type X = Y`). A non-local Go import path ending in `/v2`, `/v3`, ... without
an alias is a parse error.

Sema fills:

- `VarDecl.Type` for typed `var`; one type applies to every name
- `Ident.Ref`, `SelectorExpr.Ref`
- `ast.Use.GoPath`, `PackageName`, `LocalDir`, `LocalPackage`
- `ast.NamedType.Decl/Variant/GoName/PkgAlias`
- `ast.CompositeLit.Variant`, `CompositeEntry.Field`
- `ast.CallExpr.Info` (including `ResultTypes`)
- `ast.FuncLit.ParamTypes`
- `ast.FuncDecl.Results`, `GoName`
- `ast.IFaceMethod.Results`
- `ast.FuncType.Results`
- `ast.GoCallStmt.Call`, `ast.OnErrStmt.Body`
- `ast.MatchExpr.ResultType`, `IsExpr`, `SubjectType`, `TypeSwitch`
- `ast.Case.Pat.Variant`, `FieldDecls`, `Default`
- `ast.Pattern.TypePat`, `Bind`
- `ast.TestDecl.GoID`
- `ast.TypeDecl.GoName`, `ast.Variant.GoName`
- `ast.ArrayType` normalization from `GenericType` when a single argument is a
  constant expression

Sema rules added with the canonical surface:

- `const` is package-level only; a local `const` is an error (`use var`).
  `const` cannot have a type. It is a compile-time name used for array lengths
  and case patterns, not immutable runtime storage.
- Typed `var` resolves `VarDecl.Type`, applies it to bare `nil` values and
  declares every name with that one type.
- `void` is accepted only as a function result. `rejectVoid` rejects it in
  variables, fields, parameters, arguments, returns with values and composite
  elements.
- `go for` over a map is rejected; `for ()` is rejected in a `go`/`go for`
  body. Function literals remain outside the static analysis.
- A fallible call as a bare expression statement is rejected; `_ = f();` is
  the explicit discard.
- `on err { }` at the start of a block is a scope listener: non-empty, no
  `fail`/`return` inside. An inline `expr on err { ... }` is a local handler.

Sema also checks interface conformance (`internal/sema/iface.go`): method sets
for local structs, local interfaces and Go types, compared at assignments,
arguments, returns and composite elements. Hidden context is not added to the
compared Ink signature; an explicit Go `context.Context` parameter is
compared as an ordinary parameter. Signatures that cannot be named (generics,
enum variants, arrays, function values, variadic methods) are skipped and left
to `go build`; multi-result assignments/returns are also skipped.

Sema checks Ink-specific rules only. Go checks the rest of the types.

### `internal/gen`

```go
type File struct { Path string; Src []byte }
type Options struct { Root, Module string; IncludeTests bool }
func Generate(p *sema.Program, files map[string][]*ast.File, opt Options) ([]File, []error)
```

Outputs one `zz_ink.go` per local package and, if `IncludeTests`, one
`zz_ink_test.go` per package with test declarations.
Generated files are formatted with `go/format`.
Every declaration, statement and expression gets a
`//line <path>:<line>:<col>` directive where practical (at least decls,
statements, and call expressions). The path is relative to the generated file.
One-character fallback ends for sema/typecheck diagnostics are a known limit.

### `internal/typecheck`

```go
type Error struct { Path string; Pos token.Pos; Msg string }
func Check(p *sema.Program, files map[string][]*ast.File, opt Options) []Error
```

Generates Go in memory with `internal/gen`, overlays every `zz_ink.go` and
the embedded runtime into `go/packages`, and maps `//line` positions back to
`.ink` files. The load mode is `LoadImports | NeedExportFile`, not `LoadTypes`.
Results are cached in memory by a SHA-256 of generated files, `go.mod`,
`go.sum`, the runtime source, and the `Tests`/`Race` options; the last 16
results are kept. `ink check` and LSP call it; `build`, `run` and `test` still
let `go build`/`go test` find type errors. In the LSP, `didChange` submits
parse + sema to the analysis worker and schedules the typecheck after 150 ms;
a newer edit drops stale results. The cache does not include dependency
sources, hand-written `.go` files, or `GOOS`/`GOARCH`.

### `internal/format`

```go
func File(f *ast.File) []byte
```

Canonical source. Tabs are not used; indentation is four spaces.
Comments are preserved by line. The formatter drops `=` before struct/enum/
interface bodies and function bodies, keeps `=` on aliases, adds explicit
`void` to a result-less declaration and prints interface methods without `fn`.

### `internal/lsp`

```go
func Run(stdin io.Reader, stdout io.Writer, stderr io.Writer, root string) error
```

One JSON-RPC loop over stdio. It publishes parse/sema/typecheck diagnostics with
stable codes and provides hover, definition, completion, document and workspace
symbols, signature help, formatting, references/rename for project
declarations, and code actions for canonical errors. Sema is synchronous;
the generated-Go typecheck is debounced by 150 ms and stale generations are
dropped. `ink check` remains the authoritative whole-project check.

### `internal/driver`

High-level operations used by the CLI. The driver owns `go.mod`,
`go.mod`, the Go import stub, parsing, checking, generation and Go commands.

## Codegen rules

### Type mapping

| Ink | Go |
|---|---|
| `str` | `string` |
| `u8`…`u64` | `uint8`…`uint64` |
| `i8`…`i64` | `int8`…`int64` |
| `int`/`uint` | `int`/`uint` |
| `f32`/`f64` | `float32`/`float64` |
| `rune`/`byte` | `rune`/`byte` |
| `Duration` | `time.Duration` |
| `Type` | `reflect.Type` |
| `T[]` | `[]T` |
| `T[N]` | `[N]T` |
| `map[K,V]` | `map[K]V` |
| `*T` | `*T` |
| `any` | `any` |
| `error` | `error` |
| `fn(A) R` | `func(A) R` |
| `void` | no result |
| `T!` | `(T, error)` |

### Packages and imports

- Generated Go package name = sanitized directory base (`-` → `_`), root is
  `main`.
- Local import `use ./ed as ed;` → `<module>/ed` with alias `ed`.
- Go import keeps its path, alias from the `use` declaration or the real Go
  package name.
- Imports are merged per package; conflicting aliases are a sema error.
- Generated support package: `<module>/.ink/runtime`, imported only when
  needed (defer stack, asserts, `has`, `ParErrors`).

### Main wrapper

`fn main() void!` generates:

```go
func main() {
    if err := _s_main(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

The real body is named `_s_main` when a wrapper is needed. A `void` main gets
a plain wrapper.

### Context

- `context.Context` is an ordinary Ink parameter and maps to Go
  `context.Context` 1:1, in any position.
- There is no `NeedsCtx`, no `_s_Name_ctx` twin and no public wrapper.
- `main` starts without a context; a handler that needs one derives it with
  `context.WithTimeout` and passes it explicitly.
- `go` creates no context and does not observe cancellation.

### `defer`

Block-scoped defer is implemented with a per-function stack:

```go
var _s_d runtime.DeferStack
defer _s_d.RunTo(0)
```

- At block entry: `_s_mark := _s_d.Mark()`.
- `defer f(args);` → evaluate receiver and args into temps, then
  `_s_d.Push(func() { f(a1, a2) })`.
- At normal block exit: `_s_d.RunTo(_s_mark)`.
- On `return`: `_s_d.RunTo(0)` before the return.
- On `break`/`continue`: run to the mark of the loop body being left.
- On `or fail`: same as return/panic.
- Each `go`/`go for` worker declares its own `_s_d` inside the goroutine and
  runs `RunTo(0)` at worker exit. It never touches the enclosing function's
  stack.

### Errors (`or`, `on err`)

- `var x = f() or fail;` → `_s_v, _s_err := f(); if _s_err != nil { cleanup; return zero, _s_err }; x := _s_v`
- For a `void!` function: `return _s_err`.
- For `T!`: zero value `*new(T)`; a nil result uses `(T)(nil)`.
- Multiple non-error results: `return a, b, nil`; `return a, b` appends nil;
  `return f()` is a raw pass-through when the arities match.
- `expr on err { ... }` → `_s_v, _s_err := f(); if _s_err != nil { err := _s_err; ... }`.
- A scope listener emits `_s_onerr := func(err error) { ... }` at block entry;
  `go` joins and detached calls use it. A handler that falls through leaves
  the bound value at its zero value.
- Direct call with no assignment and only an error result:
  `if _s_err := f(); _s_err != nil { ... }`.
- `CallInfo.Results`, `ResultTypes` and `HasError` drive the shape.
- `or` supports multiple non-error results: `var a, b = f() or fail;`
  generates `a, b` from `CallInfo.ResultTypes[:Results]`. A multi-result call
  is still not a tuple value and cannot be nested in another expression.

### Multiple results

- `FuncDecl.Results`, `FuncType.Results`, `IFaceMethod.Results`,
  `FuncLit.Results` hold non-error results; `Err` adds the trailing error.
- Declarations use `fn f() (A, B)!`; function types group with
  `fn(A) (B, C)`.
- `var a, b = f()` and `a, b = f()` are valid; `CallInfo.ResultTypes` lets sema
  type the names. A multi-result call is not a tuple value.

### Enums

```go
type Shape interface{ _s_shape() }

type ShapeCircle struct{ Radius float64 }
func (ShapeCircle) _s_shape() {}

type ShapeRect struct{ W, H float64 }
func (ShapeRect) _s_shape() {}
```

- Construction `Shape.Circle { radius: 2.0 }` → `&ShapeCircle{Radius: 2.0}`.
- `match` → type switch over `*ShapeCircle` etc.
- Variant field bindings become locals in the case clause.
- Exported variant types use the same case as the variant name.

### Structs

- Methods: `func (self *T) Name(args) ...`.
- Bare field identifiers resolved with `RefField` are emitted as `self.Field`.
- Struct literal: `T{Field: value}`.
- Typed `var` emits the declared type; `const` emits Go `const`.

### Loops and `match`

- `break`/`continue` inside a `match` case target the enclosing loop.
- `loopNeedsLabel` labels a loop when its body contains a match with
  `break`/`continue`; the statement is emitted as `break _s_loop` or
  `continue _s_loop`. Plain `break` outside a match stays unlabeled.

### `go`

```go
var _s_wg sync.WaitGroup
var _s_pe _s_rt.ParErrors
_s_wg.Add(N)
go func() {
    defer _s_wg.Done()
    var _s_d _s_rt.DeferStack
    defer _s_d.RunTo(0)
    ...
}()
_s_wg.Wait()
if _s_err := _s_pe.First(); _s_err != nil {
    _s_onerr(_s_err)        // or return zero, _s_err
}
```

- Each top-level statement is its own goroutine and its own scope; `ord` is
  its index. A local declared in one worker is not visible in another.
- No cancelable context is created and no recovery boundary is installed.
  Workers run to completion; a panic is a Go panic and aborts the process.
- Each goroutine has its own `DeferStack` and runs it to zero at worker exit,
  before reporting an error.
- `or fail` drains the worker's `DeferStack` first, then adds at the worker's
  ord and returns from the goroutine.
- At the join, the winner goes to the block's `_s_onerr` if present; otherwise
  an error-returning function returns it. A `void` function without a listener
  and a fallible branch is a sema error.
- Writes to globals are rejected in both `go` and `go for`: a global is
  shared with the whole program, not only with the other workers.

### `go for`

- Sema has already rejected `return`/`break`/`continue` that leave the body,
  a map range, and `for ()` in the body.
- `limit N` default `runtime.GOMAXPROCS(0)`.
- Each iteration runs in its own goroutine with its own `DeferStack`.
- Iteration variables are captured per iteration. A C-style init with
  assignment (`go for (i = 0; ...)`) captures `_s_iN := i` per iteration and
  rewrites body references to the copy; the condition and post use the outer
  variable. A direct write to `i` in the body is rejected.
- Errors are stored by iteration ordinal; lowest index wins.
- `for (var x in xs)` uses the range index as ordinal.
- Map ranges are rejected by sema, so Ink source never reports ord `-1`.
  The runtime keeps the `-1` path (first-completed wins, after all
  non-negative indexes) for compatibility.

### `go f(args)`

```go
_a1 := arg1
go func() {
    if err := f(_a1); err != nil {
        // handler body from `on err { ... }`
    }
}()
```

- Arguments are evaluated into temps before the goroutine starts; the call
  returns immediately.
- A fallible call without a handler is rejected by sema, so the `nil` branch is
  the only place an error can go. `panic(err)` is emitted only for a handler
  that is missing, which sema already rejects.
- There is no wait and no handle: the task is owned by the program.

### Tests

- `test "name" { ... };` → `func TestName(t *testing.T)` in
  `zz_ink_test.go`, with unique suffix on duplicates.
- `assert(cond)` → `runtime.Assert(cond, "file:line")`.
- `assert_eq(a, b)` → `runtime.AssertEq(a, b, "file:line")`.

## Deep recursion

Ink does not guarantee tail calls. Deep recursion can overflow the Go stack
the same way Go code does.

## Generated runtime

`runtime/` is a normal Go package that is embedded into the compiler and copied
to `.ink/runtime` in the target project.

Required API:

```go
type DeferStack struct{ ... }
func (d *DeferStack) Mark() int
func (d *DeferStack) Push(fn func())
func (d *DeferStack) RunTo(mark int)
func Assert(ok bool, pos string)
func AssertEq[T comparable](got, want T, pos string)
func Has[K comparable, V any](m map[K]V, k K) bool

type ParErrors struct{ ... }
func (p *ParErrors) Add(index int, err error)
func (p *ParErrors) First() error
```

`ParErrors.First()` returns the lowest non-negative index. Entries with `-1`
(legacy map iterations; source-level map ranges are rejected) are considered
only when no non-negative entry exists, and then the earliest `Add` wins.
