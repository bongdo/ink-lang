package sema

import "testing"

func TestArenaRejectsPar(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f() void {
    arena {
        par {
            var p = new[i32]();
            _ = p;
        };
    };
};

fn main() void {};
`})
	wantError(t, p, "par is not allowed inside arena")
}
