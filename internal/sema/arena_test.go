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

func TestArenaEscapeRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Point struct { X: i32 };

fn leak() *Point {
    var out: *Point = nil;
    arena {
        out = new[Point]();
    };
    return out;
};

fn main() void {};
`})
	wantError(t, p, "arena value escapes the arena block")
}

func TestArenaReturnRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Point struct { X: i32 };

fn leak() *Point {
    arena {
        return new[Point]();
    };
};

fn main() void {};
`})
	wantError(t, p, "arena value escapes the arena block")
}

func TestArenaArgRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Point struct { X: i32 };

fn consume(p: *Point) void { _ = p; };

fn leak() void {
    arena {
        var p = new[Point]();
        consume(p);
    };
};

fn main() void {};
`})
	wantError(t, p, "arena value passed to a function")
}

func TestArenaAddrRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Point struct { X: i32 };

fn f() void {
    arena {
        var p = &Point{};
        _ = p;
    };
};

fn main() void {};
`})
	wantError(t, p, "&T{} allocates outside the arena")
}

func TestArenaScalarCopyAllowed(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Point struct { X: i32 };

fn f() i32 {
    var n = i32(0);
    arena {
        var p = new[Point]();
        p.X = 7;
        n = p.X;
    };
    return n;
};

fn main() void {};
`})
	noErrors(t, p)
}
