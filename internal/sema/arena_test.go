package sema

import "testing"

func TestArenaRejectsPar(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f() void {
    arena a {
        par {
            var p = a.New[i32]();
            _ = p;
        };
    };
};

fn main() void {};
`})
	wantError(t, p, "par is not allowed inside arena")
}
