package sema

import (
	"strings"
	"testing"
)

func TestBlankImport(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use net/http/pprof as _;

fn main() void {
};
`})
	noErrors(t, p)
}

func TestBlankImportLocal(t *testing.T) {
	p := checkSrc(t, map[string]string{
		".": `use ./lib as _;

fn main() void {
};
`,
		"lib": `fn Helper() void {
};
`,
	})
	if len(p.Errors) != 1 {
		t.Fatalf("expected exactly one error, got %v", p.Errors)
	}
	if msg := p.Errors[0].Msg; !strings.Contains(msg, "blank import must be a Go package") {
		t.Fatalf("unexpected message: %s", msg)
	}
}

func TestBannedImports(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{`use sync;

fn main() void {
};
`, "import sync is not available in Ink; use par and spawn"},
		{`use sync/atomic;

fn main() void {
};
`, "import sync/atomic is not available in Ink; use par and spawn"},
		{`use errors;

fn main() void {
};
`, "import errors is not available in Ink; use ? and on err"},
	}
	for _, c := range cases {
		p := checkSrc(t, map[string]string{".": c.src})
		e := wantError(t, p, c.want)
		if e.Pos.Line != 1 {
			t.Fatalf("error at line %d, want 1: %s", e.Pos.Line, e.Msg)
		}
	}
}

func TestFmtErrorfBanned(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;

fn main() void {
    _ = fmt.Errorf("x");
};
`})
	wantError(t, p, "fmt.Errorf is not available in Ink; use check")
}

func TestGeneratedTypeCollision(t *testing.T) {
	cases := []struct {
		src  string
		line int
	}{
		{`enum Color {
    Red,
};

struct ColorRed {
    X: i32,
};

fn main() void {
};
`, 5},
		{`struct ColorRed {
    X: i32,
};

enum Color {
    Red,
};

fn main() void {
};
`, 6},
	}
	for _, c := range cases {
		for i := 0; i < 5; i++ {
			p := checkSrc(t, map[string]string{".": c.src})
			if len(p.Errors) != 1 {
				t.Fatalf("expected exactly one error, got %v", p.Errors)
			}
			e := p.Errors[0]
			if !strings.Contains(e.Msg, "ColorRed") || !strings.Contains(e.Msg, "Red") {
				t.Fatalf("message does not name both declarations: %s", e.Msg)
			}
			if e.Pos.Line != c.line {
				t.Fatalf("error reported at line %d, want %d: %s", e.Pos.Line, c.line, e.Msg)
			}
		}
	}
}
