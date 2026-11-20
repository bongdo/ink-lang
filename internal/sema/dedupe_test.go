package sema

import (
	"strings"
	"testing"
)

func TestDuplicateUnknownTypeIsReportedOnce(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void! {
    var p: *Missing = nil;
    return;
};
`})
	count := 0
	for _, e := range p.Errors {
		if strings.Contains(e.Msg, "unknown type Missing") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("unknown type diagnostics = %d, want 1; errors: %v", count, p.Errors)
	}
}
