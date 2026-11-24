package sema

import "testing"

func TestMissingReturn(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "falls through",
			src: `fn f() i32 {
};

fn main() void {
    _ = f();
};
`,
			want: true,
		},
		{
			name: "panic",
			src: `fn f() i32 {
    panic("x");
};

fn main() void {
    _ = f();
};
`,
			want: false,
		},
		{
			name: "if else",
			src: `fn f(n: i32) i32 {
    if (n > 0) {
        return 1;
    } else {
        return 2;
    };
};

fn main() void {
    _ = f(1);
};
`,
			want: false,
		},
		{
			name: "if without else",
			src: `fn f(n: i32) i32 {
    if (n > 0) {
        return 1;
    };
};

fn main() void {
    _ = f(1);
};
`,
			want: true,
		},
		{
			name: "match all cases return",
			src: `type E enum {
    A,
    B,
};

fn f(e: E) i32 {
    match (e) {
    case A => return 1;
    case B => return 2;
    };
};

fn main() void {
    _ = f(E.A);
};
`,
			want: false,
		},
		{
			name: "match one case falls through",
			src: `type E enum {
    A,
    B,
};

fn f(e: E) i32 {
    match (e) {
    case A => return 1;
    case B => f(e);
    };
};

fn main() void {
    _ = f(E.A);
};
`,
			want: true,
		},
		{
			name: "match panic expression and block return",
			src: `type E enum {
    A,
    B,
};

fn m(e: E) i32 {
    match (e) {
    case A => panic("bad");
    case B => { return 2; };
    };
};

fn main() void {
    _ = m(E.A);
};
`,
			want: false,
		},
		{
			name: "match all panic expressions",
			src: `type E enum {
    A,
    B,
};

fn m(e: E) i32 {
    match (e) {
    case A => panic("bad");
    case B => panic("bad");
    };
};

fn main() void {
    _ = m(E.A);
};
`,
			want: false,
		},
		{
			name: "infinite for",
			src: `fn f() i32 {
    for () {
    };
};

fn main() void {
    _ = f();
};
`,
			want: false,
		},
		{
			name: "infinite for with break",
			src: `fn f(ok: bool) i32 {
    for () {
        if (ok) {
            break;
        };
    };
};

fn main() void {
    _ = f(true);
};
`,
			want: true,
		},
		{
			name: "literal falls through",
			src: `fn main() void {
    var f = fn(n: i32) i32 {
        if (n > 0) {
            return 1;
        };
    };
    _ = f(1);
};
`,
			want: true,
		},
		{
			name: "literal all paths return",
			src: `fn main() void {
    var f = fn(n: i32) i32 {
        if (n > 0) {
            return 1;
        } else {
            return 2;
        };
    };
    _ = f(1);
};
`,
			want: false,
		},
		{
			name: "arrow literal",
			src: `fn main() void {
    var f = fn(n: i32) i32 => n + 1;
    _ = f(1);
};
`,
			want: false,
		},
		{
			name: "void literal",
			src: `fn main() void {
    var f = fn() void {
    };
    f();
};
`,
			want: false,
		},
		{
			name: "callback literal falls through",
			src: `use slices;

fn main() void {
    var xs = i32[] { 2, 1 };
    slices.SortFunc(xs, fn(a: i32, b: i32) int {
        if (a < b) {
            return -1;
        };
    });
};
`,
			want: true,
		},
		{
			name: "error only",
			src: `fn f() void! {
};

fn main() void! {
    f() or fail;
};
`,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := checkSrc(t, map[string]string{".": tc.src})
			if got := hasError(p, "missing return"); got != tc.want {
				t.Fatalf("missing return = %v, want %v; errors: %v", got, tc.want, p.Errors)
			}
		})
	}
}
