package sema

import "testing"

func TestParForMapRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var m = map[i32, i32] { 1 = 1 };
    go for (var k, v in m) {
        _ = k;
        _ = v;
    };
};
`})
	wantError(t, p, "map iteration is not allowed in go for; collect keys into a slice first")
}

func TestForUnboundedInsideParRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    go {
        for () {
            break;
        };
    };
};
`})
	wantError(t, p, "for () is not allowed inside go; use a bounded loop")
}

func TestForUnboundedOutsideParAllowed(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    for () {
        break;
    };
};
`})
	noErrors(t, p)
}

func TestForUnboundedInFuncLitInsideParAllowed(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    go {
        {
            var f = fn() void {
                for () {
                    break;
                };
            };
            f();
        };
    };
};
`})
	noErrors(t, p)
}
