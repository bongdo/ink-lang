package sema

import (
	"strings"
	"testing"
)

func TestNilMapWriteWarns(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var m: map[str, i32] = nil;
    m["a"] = 1;
};
`})
	found := false
	for _, e := range p.Errors {
		if e.Warning && strings.Contains(e.Msg, "nil map") {
			found = true
		}
	}
	if !found {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestNilMapWriteAllowsLiteralAndReassign(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var a = map[str, i32] {};
    a["x"] = 1;
    var b: map[str, i32] = nil;
    b = map[str, i32] {};
    b["y"] = 2;
    _ = a;
    _ = b;
};
`})
	noErrors(t, p)
}

func TestMapCtorNilValue(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Holder struct {
    Items: map[str, i32],
};

fn take(m: map[str, i32]) void {
    _ = m;
};

fn main() void {
    var m = map[str, i32](nil);
    var h = Holder { Items: map[str, i32](nil) };
    take(map[str, i32](nil));
    _ = m;
    _ = h;
};
`})
	noErrors(t, p)
}
