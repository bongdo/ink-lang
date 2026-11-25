package sema

import "testing"

func TestGoCallReadVisibleToPar(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn job(n: i32) void { _ = n; };

fn f() void {
    var a = i32(0);
    par {
        a = 1;
        spawn job(a);
    };
};

fn main() void {};
`})
	wantError(t, p, "read by another par statement: a")
}

func TestGoLimitMustBePositive(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    par for (var i = 0; i < 4; i += 1) limit 0 {
        _ = i;
    };
};
`})
	wantError(t, p, "limit must be positive")
}

func TestArenaAddrOfElementEscapes(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `struct Point { X: i32 };

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

func TestGoVoidWithListener(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;

fn f() i32! { check(false, "x"); };

fn handler() void {
    on err {
        fmt.Println("logged", err)
            on err { _ = err; };
    };
    var a = i32(0);
    par {
        a = f()?;
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
	p := checkSrc(t, map[string]string{".": `struct Point {
    X: i32,

    fn Touch() void { X += 1; };
};

fn main() void {
    arena {
        var ps = Point[](4);
        spawn ps[0].Touch();
    };
};
`})
	wantError(t, p, "arena value cannot leave the arena block")
}
