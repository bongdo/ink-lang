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
			name: "within returns",
			src: `fn f() i32 {
    within 1s {
        return 1;
    };
};

fn main() void {
    _ = f();
};
`,
			want: false,
		},
		{
			name: "error only",
			src: `fn f() void or error {
};

fn main() void or error {
    f() or return;
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
