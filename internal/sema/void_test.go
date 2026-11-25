package sema

import "testing"

func TestVoidRejectedInValuePositions(t *testing.T) {
	cases := map[string]string{
		"local var": `fn note() void {};

fn main() void {
    var x = note();
    _ = x;
};
`,
		"package var": `var x = note();

fn note() void {};

fn main() void {};
`,
		"assignment": `fn note() void {};

fn main() void {
    var x = 0;
    x = note();
};
`,
		"call argument": `fn note() void {};

fn take(x: i32) void {};

fn main() void {
    take(note());
};
`,
		"return value": `fn note() void {};

fn run() void! {
    return note();
};

fn main() void {};
`,
		"struct field": `struct S {
    x: void,
};

fn main() void {};
`,
		"param": `fn take(x: void) void {};

fn main() void {};
`,
		"slice element": `fn note() void {};

fn main() void {
    var xs = i32[] { note() };
    _ = xs;
};
`,
		"struct literal field": `struct S {
    x: i32,
};

fn note() void {};

fn main() void {
    var s = S { x = note() };
    _ = s;
};
`,
		"map value": `fn note() void {};

fn main() void {
    var m = map[str, i32] { "a" = note() };
    _ = m;
};
`,
		"void slice type": `fn main() void {
    var xs = void[] {};
    _ = xs;
};
`,
		"generic void arg": `struct Box[T] {
    v: T,
};

fn main() void {
    var b = Box[void] {};
    _ = b;
};
`,
		"func lit param": `fn main() void {
    var f = fn(x: void) void {
    };
    _ = f;
};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			p := checkSrc(t, map[string]string{".": src})
			wantError(t, p, "void")
		})
	}
}

func TestVoidAllowedAsResultOnly(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn note() void {
    return;
};

fn stop() void! {
    return;
};

fn wrap() void! {
    return stop();
};

fn main() void! {
    note();
    stop()?;
};
`})
	noErrors(t, p)
}
