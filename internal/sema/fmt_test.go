package sema

import "testing"

func TestFmtPrintlnRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;

fn main() void {
    fmt.Println("x");
};
`})
	wantError(t, p, "fmt.Println is replaced by print")
}

func TestFmtSprintRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;

fn main() void {
    print(fmt.Sprint(1));
};
`})
	wantError(t, p, "fmt.Sprint is replaced by sprint")
}

func TestFmtSprintfAllowed(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;

fn main() void {
    print(fmt.Sprintf("%d", 1));
};
`})
	noErrors(t, p)
}
