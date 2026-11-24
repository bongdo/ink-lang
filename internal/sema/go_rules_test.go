package sema

import "testing"

func TestGoCallReadVisibleToPar(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn job(n: i32) void { _ = n; };

fn f() void {
    var a = i32(0);
    go {
        a = 1;
        go job(a);
    };
};

fn main() void {};
`})
	wantError(t, p, "read by another go statement: a")
}

func TestGoLimitMustBePositive(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    go for (var i = 0; i < 4; i += 1) limit 0 {
        _ = i;
    };
};
`})
	wantError(t, p, "limit must be positive")
}

func TestArenaAddrOfElementEscapes(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Point struct { X: i32 };

fn work(p: *Point) void { _ = p; };

fn main() void {
    arena {
        var ps = Point[](4);
        work(&ps[0]);
    };
};
`})
	wantError(t, p, "arena value cannot leave the arena block")
}

func TestGoVoidOrFailWithListener(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;
use errors;

fn f() i32! { fail(errors.New("x")); };

fn handler() void {
    on err {
        fmt.Println("logged", err)
            on err { _ = err; };
    };
    var a = i32(0);
    go {
        a = f()
            or fail;
    };
    fmt.Println(a)
        on err { _ = err; };
};

fn main() void {};
`})
	noErrors(t, p)
}

func TestListenerFuncLiteralMayReturn(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;

fn main() void {
    on err {
        var f = fn(x: i32) i32 { return x; };
        fmt.Println(f(1), err)
            on err { _ = err; };
    };
};
`})
	noErrors(t, p)
}

func TestArenaReceiverInDetachedGo(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Point struct {
    X: i32,

    fn Touch() void { X += 1; };
};

fn main() void {
    arena {
        var ps = Point[](4);
        go ps[0].Touch();
    };
};
`})
	wantError(t, p, "arena value cannot leave the arena block")
}
