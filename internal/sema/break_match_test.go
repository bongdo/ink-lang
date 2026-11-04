package sema

import "testing"

func TestBreakContinueInsideMatch(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    for (var i = 0; i < 5; i += 1) {
        match (i) {
        case 1 => {
            continue;
        };
        case 3 => {
            break;
        };
        else => {
            _ = i;
        };
        };
    };
};
`})
	noErrors(t, p)
}

func TestBreakInMatchOutsideLoopRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    match (1) {
    else => {
        break;
    };
    };
};
`})
	wantError(t, p, "break outside a loop")
}

func TestBreakInMatchInsideFuncLitRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    for () {
        var f = fn() void {
            match (1) {
            else => {
                break;
            };
            };
        };
        _ = f;
        break;
    };
};
`})
	wantError(t, p, "break outside a loop")
}
