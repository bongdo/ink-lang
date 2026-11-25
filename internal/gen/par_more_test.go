package gen

import (
	"strings"
	"testing"
)

func TestGoDeferPanicCrashes(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `fn boom() void {
    panic("defer-panic");
};

fn stop() void! {
    check(false, "original");
};

fn main() void! {
    par {
        {
            defer boom();
            stop()?;
        };
    };
};
`,
	})
	_, stderr, err := p.run(t)
	if err == nil {
		t.Fatal("expected failure")
	}
	if !strings.Contains(stderr, "defer-panic") {
		t.Fatalf("defer panic lost:\n%s", stderr)
	}
	if strings.Contains(stderr, "original") {
		t.Fatalf("original error should not win:\n%s", stderr)
	}
}
