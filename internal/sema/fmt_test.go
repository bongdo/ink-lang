package sema

import (
	"strings"
	"testing"
)

func wantWarning(t *testing.T, p *Program, sub string) {
	t.Helper()
	for _, e := range p.Errors {
		if e.Warning && strings.Contains(e.Msg, sub) {
			return
		}
	}
	t.Fatalf("want warning %q, got %v", sub, p.Errors)
}

func TestFmtPrintlnWarns(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;

fn main() void {
    fmt.Println("x");
};
`})
	wantWarning(t, p, "fmt.Println is replaced by print")
}

func TestFmtSprintWarns(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;

fn main() void {
    print(fmt.Sprint(1));
};
`})
	wantWarning(t, p, "fmt.Sprint is replaced by sprint")
}

func TestFmtSprintfAllowed(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;

fn main() void {
    print(fmt.Sprintf("%d", 1));
};
`})
	noErrors(t, p)
}
