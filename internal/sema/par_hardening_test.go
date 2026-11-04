package sema

import "testing"

func TestParForMapRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var m = map[i32, i32] { 1 = 1 };
    par for (var k, v in m) {
        _ = k;
        _ = v;
    };
};
`})
	wantError(t, p, "map iteration is not allowed in par for; collect keys into a slice first")
}

func TestForUnboundedInsideParRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    par {
        for () {
            break;
        };
    };
};
`})
	wantError(t, p, "for () is not allowed inside par; use a bounded loop")
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
    par {
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
