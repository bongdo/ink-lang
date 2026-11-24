package sema

import "testing"

func TestArenaAllowsParOnPreallocated(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f() void {
    arena {
        var xs = i32[](16);
        par for (var y = i32(0); y < i32(16); y += 1) limit 4 {
            xs[y] = y * 2;
        };
    };
};

fn main() void {};
`})
	noErrors(t, p)
}

func TestArenaRejectsMethodCall(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Point { X: i32 };

struct Mover {
    fn Move(p: *Point) void { p.X = 1; };
};

fn f() void {
    arena {
        var m = new[Mover]();
        var p = new[Point]();
        m.Move(p);
    };
};

fn main() void {};
`})
	wantError(t, p, "method call on an arena value")
}

func TestArenaRejectsClosureEscape(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Point { X: i32 };

fn f() void {
    var out = fn() i32 { return 0; };
    arena {
        var p = new[Point]();
        out = fn() i32 { return p.X; };
    };
};

fn main() void {};
`})
	wantError(t, p, "arena value escapes the arena block")
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
        var outer = i32[](4);
        par {
            arena {
                var tmp = i32[](4);
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
        var xs = i32[](4);
        arena {
            var ys = i32[](4);
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
	p := checkSrc(t, map[string]string{".": `struct Point { X: i32 };

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
	p := checkSrc(t, map[string]string{".": `struct Point { X: i32 };

fn leak() *Point {
    arena {
        return new[Point]();
    };
};

fn main() void {};
`})
	wantError(t, p, "arena value escapes the arena block")
}

func TestArenaRejectsArg(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Point { X: i32 };

fn consume(p: *Point) void { _ = p; };

fn f() void {
    arena {
        var p = new[Point]();
        consume(p);
    };
};

fn main() void {};
`})
	wantError(t, p, "arena value cannot leave the arena block")
}

func TestArenaAllowsPureBuiltins(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f() i32 {
    var n = i32(0);
    arena {
        var xs = i32[](4, cap: 8);
        n = len(xs) + cap(xs);
    };
    return n;
};

fn main() void {};
`})
	noErrors(t, p)
}

func TestArenaRejectsStructWithPointer(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Point { X: i32 };

struct Holder {
    P: *Point,
};

fn consume(h: Holder) void { _ = h; };

fn f() void {
    arena {
        var hs = Holder[](2);
        var h = hs[0];
        consume(h);
    };
};

fn main() void {};
`})
	wantError(t, p, "arena value cannot leave the arena block")
}

func TestArenaAllowsScalarStructCopy(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Vec { X: i32 };

fn consume(v: Vec) void { _ = v; };

fn f() void {
    arena {
        var vs = Vec[](2);
        var v = vs[0];
        consume(v);
    };
};

fn main() void {};
`})
	noErrors(t, p)
}

func TestArenaAddrRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Point { X: i32 };

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
	p := checkSrc(t, map[string]string{".": `struct Point { X: i32 };

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

func TestArenaRejectsCopyOfPointers(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Point { X: i32 };
alias PointPtr = *Point;

fn f() void {
    var out = PointPtr[](2);
    arena {
        var xs = PointPtr[](2);
        copy(out, xs);
    };
};

fn main() void {};
`})
	wantError(t, p, "arena value cannot leave the arena block")
}

func TestArenaRejectsMatchBinding(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Point { X: i32 };

fn consume(p: *Point) void { _ = p; };

fn f() void {
    arena {
        var p = new[Point]();
        var v: any = p;
        match (v) {
            case *Point as q => {
                consume(q);
            };
            else => {};
        };
    };
};

fn main() void {};
`})
	wantError(t, p, "arena value cannot leave the arena block")
}
