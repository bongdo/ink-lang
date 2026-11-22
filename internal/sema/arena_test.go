package sema

import "testing"

func TestArenaAllowsParOnPreallocated(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn fill(xs: i32[], y: i32) void {
    xs[y] = y;
};

fn f() void {
    arena {
        var xs = make[i32](16);
        par for (var y = 0; y < 16; y += 1) limit 4 {
            fill(xs, y);
        };
    };
};

fn main() void {};
`})
	noErrors(t, p)
}

func TestArenaRejectsAllocationInsidePar(t *testing.T) {
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
	wantError(t, p, "arena allocation inside par")
}

func TestArenaAllowsWorkerArena(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f() void {
    arena {
        var outer = make[i32](4);
        par {
            arena {
                var tmp = make[i32](4);
                tmp[0] = outer[0];
            };
        };
    };
};

fn main() void {};
`})
	noErrors(t, p)
}

func TestArenaAllowsNested(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f() i32 {
    var n = i32(0);
    arena {
        var xs = make[i32](4);
        arena {
            var ys = make[i32](4);
            ys[1] = 5;
            n = ys[1];
        };
        xs[0] = n;
    };
    return n;
};

fn main() void {};
`})
	noErrors(t, p)
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

func TestArenaArgBorrows(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Point struct { X: i32 };

fn consume(p: *Point) void { _ = p; };

fn f() void {
    arena {
        var p = new[Point]();
        consume(p);
    };
};

fn main() void {};
`})
	noErrors(t, p)
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
