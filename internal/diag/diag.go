package diag

import (
	"regexp"
	"strings"
)

var typeWords = regexp.MustCompile(`\b(int32|int64|uint8|uint16|uint32|uint64|float32|float64|string)\b`)

func InkWords(msg string) string {
	msg = strings.ReplaceAll(msg, "[]byte", "u8[]")
	msg = strings.ReplaceAll(msg, "[]rune", "rune[]")
	return typeWords.ReplaceAllStringFunc(msg, func(word string) string {
		switch word {
		case "int32":
			return "i32"
		case "int64":
			return "i64"
		case "uint8":
			return "u8"
		case "uint16":
			return "u16"
		case "uint32":
			return "u32"
		case "uint64":
			return "u64"
		case "float32":
			return "f32"
		case "float64":
			return "f64"
		case "string":
			return "str"
		}
		return word
	})
}

type Info struct {
	Code string
	Hint string
}

var rules = []struct {
	Code    string
	Match   string
	Hint    string
	Warning bool
}{
	{"colon-equals", "no ':='", "var x = value;", false},
	{"go-package", "no package clause", "omit the package clause; one directory is one package", false},
	{"go-import", "import is not", "use fmt;", false},
	{"go-func", "functions are declared with fn", "fn main() void { ... };", false},
	{"go-switch", "switch does not exist", "use if or match", false},
	{"go-chan", "chan does not exist", "keep channels in a Go adapter", false},
	{"par-removed", "par was renamed to go", "use go { ... };", false},
	{"within-removed", "within was removed", "use context.WithTimeout", false},
	{"or-panic", "or panic was removed", "use on err { panic(...) } or or fail", false},
	{"or-err", "instead of 'or err", "use on err { ... };", false},
	{"go-select", "select does not exist", "use go or a Go adapter", false},
	{"go-goto", "goto does not exist", "use for, break and continue", false},
	{"go-fallthrough", "fallthrough does not exist", "give each match arm its own body", false},
	{"import-simple", "written without quotes", "use fmt;", false},
	{"import-alias", "requires an alias", "use charm.land/bubbletea/v2 as tea;", false},
	{"unused-import", "is not used", "remove the import, use it, or write use pkg as _; for a side effect", false},
	{"decl-equals", "declared without '='", "type User struct { ... }; fn f() void { ... };", false},
	{"iface-fn", "interface methods are declared without 'fn'", "Write(p: u8[]) int!;", false},
	{"comment-block", "block comments are not supported", "// comment", false},
	{"const-local", "const is only allowed at package level", "var N = 3;", false},
	{"const-type", "const cannot have a type", "const N = 3;", false},
	{"err-manual", "use or fail instead", "var v = f() or fail;", false},
	{"go-task-err", "go call can fail", "go f() on err { log(err) };", false},
	{"error-ctor", "error is a type, not a constructor", "write errors.New(\"msg\")", false},
	{"nil-untyped", "cannot infer type of nil", "var p: *T = nil; or nil[T]", false},
	{"void-value", "void is not a value", "void is a result only; use a value-returning call", false},
	{"result-bang", "fallible results are marked", "write T! instead of T or error", false},
	{"go-int", "mismatched types int32 and int", "both sides need one width: declare the variable as i32 (var n: i32 = 0) or convert with int(x)", false},
	{"byte-arg", "as byte value", "the parameter is a byte: pass 102 or u8('f')", false},
	{"byte-arg", "as uint8 value", "the parameter is a byte: pass 102 or u8('f')", false},
	{"type-removed", "usize was removed", "use uint", false},
	{"type-removed", "isize was removed", "use int", false},
	{"go-int", "mismatched types int and int32", "both sides need one width: declare the variable as i32 (var n: i32 = 0) or convert with i32(x)", false},
	{"or-return", "or return", "use or fail; it returns the error to the caller", false},
	{"match-block-value", "match with block arms cannot be used as an expression", "use match as a statement or give every arm a value", false},
	{"match-nil-any", "case nil is not supported on any or interface", "else => ...", false},
	{"match-nil-enum", "case nil is not supported on enum", "match on the enum without case nil", false},
	{"go-map-write", "writing to map", "write to a local slice, then merge sequentially", false},
	{"go-map-range", "map iteration is not allowed in go for", "collect keys into a slice first", false},
	{"map-addressable", "cannot assign to struct field", "copy out, mutate, write back: var p = m[k]; p.F = v; m[k] = p;", false},
	{"map-addressable", "map index expression", "copy out, mutate, write back: var p = m[k]; p.F = v; m[k] = p;", false},
	{"map-addressable", "cannot call pointer method", "copy out, mutate, write back: var p = m[k]; p.F(); m[k] = p;", false},
	{"map-addressable", "field of a map element", "copy out, mutate, write back: var p = m[k]; p.F = v; m[k] = p;", false},
	{"map-addressable", "address of a map element", "copy out, mutate, write back: var p = m[k]; p.F = v; m[k] = p;", false},
	{"make-removed", "make was removed", "write the slice type: i32[](3) or u8[](cap: 4096)", false},
	{"slice-arg", "unknown slice argument", "use size: or cap:", false},
	{"slice-arg", "given twice", "pass one size and one cap", false},
	{"slice-cap", "is smaller than size", "cap must be at least the size", false},
	{"nil-map-write", "write to a nil map", "initialize it first: var m = map[K,V] {}", true},
	{"init-cycle", "initialization cycle", "break the cycle: initialize in main or inside a function", false},
	{"conformance", "does not implement", "implement every method with the same signature; methods use pointer receivers", false},
	{"conformance-unchecked", "conformance not checked", "go build is the backstop for this assignment", true},
	{"err-return", "return in on err turns the failure into success", "use fail(err) to propagate, or return a value only if success is intended", true},
	{"reserved", "is a reserved word", "rename it; the word is part of the syntax", false},
	{"or-expr", "or is statement-level", "split it: var v = f() or fail; then pass v", false},
	{"for-cond", "condition-only loop", "write for (; cond; ): for (; rows.Next(); ) { ... }", false},
	{"import-path", "is not in std", "use the full Go path: path/filepath, net/http, encoding/json, database/sql", false},
	{"var-value", "var needs a value", "every var has a value: var n = 0; var p: *T = nil;", false},
	{"method-outside-type", "declared at file scope", "declare it inside the type body: type T struct { fn M(...) ... };", false},
	{"copy-lock", "copies lock value", "use a pointer: var srv = &http.Server { ... }", false},
	{"str-number", "string of one rune", "format numbers with strconv.Itoa, strconv.FormatFloat or fmt.Sprint", false},
	{"go-capture-write", "captured from outside go", "copy the value into a local and collect the result", false},
	{"go-break", "cannot leave a go block", "return a value or set a local flag", false},
	{"go-loop", "for () is not allowed inside go", "use a bounded loop", false},
	{"array-const", "must be declared in a local package", "use a const from a local package", false},
	{"generic-args", "requires type arguments", "Stack[i32]", false},
	{"missing-return", "missing return", "end every path with return;", false},
	{"unused-import", "imported and not used", "remove the import, use it, or write use pkg as _; for a side effect", false},
	{"undefined", "undefined:", "declare or import the name", false},
	{"unused-var", "declared and not used", "use the variable or replace it with _", false},
	{"contract", "contract:", "declare wire { Field: \"name\"; } inside a struct", false},
}

func Classify(msg string) Info {
	lower := strings.ToLower(msg)
	for _, r := range rules {
		if strings.Contains(lower, strings.ToLower(r.Match)) {
			return Info{Code: r.Code, Hint: r.Hint}
		}
	}
	switch {
	case strings.Contains(lower, "expected") || strings.Contains(lower, "unexpected") ||
		strings.Contains(lower, "unterminated") || strings.Contains(lower, "invalid"):
		return Info{Code: "syntax"}
	}
	return Info{Code: "semantic"}
}

func Typecheck(msg string) Info {
	info := Classify(msg)
	if info.Code == "semantic" || info.Code == "syntax" {
		info.Code = "go-type"
	}
	if info.Code == "go-type" && info.Hint == "" {
		info.Hint = "the generated-Go typecheck rejected this: []byte is u8[], string is str, int32 is i32; convert or index at the boundary"
	}
	return info
}

func Vet(msg string) Info {
	info := Classify(msg)
	if info.Code == "semantic" {
		info.Code = "go-vet"
	}
	return info
}

func Syntax(msg string) Info {
	info := Classify(msg)
	if info.Code == "semantic" {
		info.Code = "syntax"
	}
	return info
}
