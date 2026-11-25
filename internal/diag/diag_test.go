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
		{"select does not exist; use par or a Go adapter", "go-select"},
		{"go is not an Ink word; parallel work is par { ... } / par for, a detached task is spawn f(...)", "go-removed"},
		{"contract: field \"ID\" is not in User", "contract"},
		{"within was removed; use context.WithTimeout", "within-removed"},
		{"or panic was removed; use on err { panic(...) } or or fail", "or-panic"},
		{"use 'on err { ... }' instead of 'or err { ... }'", "or-err"},
		{"goto does not exist; use for, break and continue", "go-goto"},
		{"fallthrough does not exist; give each match arm its own body", "go-fallthrough"},
		{`versioned import path "charm.land/bubbletea/v2" requires an alias: use charm.land/bubbletea/v2 as <name>;`, "import-alias"},
		{"type User is declared without '='", "decl-equals"},
		{"`type` is not an Ink word; declare struct User { ... }", "type-removed"},
		{"interface methods are declared without 'fn'", "iface-fn"},
		{"block comments are not supported; use //", "comment-block"},
		{"const is only allowed at package level; use var", "const-local"},
		{"use ? instead of if err != nil { return err }", "err-manual"},
		{"import errors is not available in Ink; use ? and on err", "go-errors"},
		{"import sync is not available in Ink; use par and spawn", "go-sync"},
		{"fmt.Errorf is not available in Ink; use check(cond, \"msg\")", "fmt-errorf"},
		{"cannot infer type of nil; use nil[T] or an expected type", "nil-untyped"},
		{"void is not a value", "void-value"},
		{"map iteration is not allowed in par for; collect keys into a slice first", "go-map-range"},
		{"writing to map m inside par", "go-map-write"},
		{"writing to p captured from outside par", "go-capture-write"},
		{"for () is not allowed inside par; use a bounded loop", "go-loop"},
		{"break cannot leave a par block", "go-break"},
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

func TestInkWords(t *testing.T) {
	got := InkWords("cannot use x (variable of type []byte) as int32 value, string of one rune")
	want := "cannot use x (variable of type u8[]) as i32 value, str of one rune"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
