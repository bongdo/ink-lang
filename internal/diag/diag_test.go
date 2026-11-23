package diag

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		msg  string
		code string
	}{
		{"import paths are written without quotes: use fmt;", "import-simple"},
		{"fallible results are marked with '!': write T! instead of T or error", "result-bang"},
		{"invalid operation: total += u.Age (mismatched types int and int32)", "go-int"},
		{"no ':=' in Ink; declare with var", "colon-equals"},
		{"files have no package clause; one directory is one package", "go-package"},
		{"import is not an Ink declaration; Ink imports with use", "go-import"},
		{"functions are declared with fn, not func", "go-func"},
		{"switch does not exist; use if or match", "go-switch"},
		{"chan does not exist; keep channels in a Go adapter", "go-chan"},
		{"select does not exist; use go or a Go adapter", "go-select"},
		{"par was renamed to go", "par-removed"},
		{"within was removed; use context.WithTimeout", "within-removed"},
		{"or panic was removed; use on err { panic(...) } or or fail", "or-panic"},
		{"use 'on err { ... }' instead of 'or err { ... }'", "or-err"},
		{"goto does not exist; use for, break and continue", "go-goto"},
		{"fallthrough does not exist; give each match arm its own body", "go-fallthrough"},
		{`versioned import path "charm.land/bubbletea/v2" requires an alias: use charm.land/bubbletea/v2 as <name>;`, "import-alias"},
		{"type definitions are declared without '='", "decl-equals"},
		{"interface methods are declared without 'fn'", "iface-fn"},
		{"block comments are not supported; use //", "comment-block"},
		{"const is only allowed at package level; use var", "const-local"},
		{"use or fail instead of if err != nil { return err }", "err-manual"},
		{"cannot infer type of nil; use nil[T] or an expected type", "nil-untyped"},
		{"void is not a value", "void-value"},
		{"map iteration is not allowed in go for; collect keys into a slice first", "go-map-range"},
		{"writing to map m inside go", "go-map-write"},
		{"writing to p captured from outside go", "go-capture-write"},
		{"for () is not allowed inside go; use a bounded loop", "go-loop"},
		{"break cannot leave a go block", "go-break"},
		{"expected ';', found end of file", "syntax"},
		{"undefined: foo", "undefined"},
		{"something else", "semantic"},
	}
	for _, tc := range cases {
		if got := Classify(tc.msg).Code; got != tc.code {
			t.Errorf("Classify(%q) = %q, want %q", tc.msg, got, tc.code)
		}
	}
}

func TestEveryCanonicalCodeHasHint(t *testing.T) {
	for _, r := range rules {
		if r.Hint == "" {
			t.Errorf("%s has no hint", r.Code)
		}
	}
}

func TestTypecheckFallback(t *testing.T) {
	if got := Typecheck("cannot use 1 (untyped int constant) as string value").Code; got != "go-type" {
		t.Fatalf("got %q", got)
	}
	if got := Typecheck("undefined: foo").Code; got != "undefined" {
		t.Fatalf("got %q", got)
	}
}
