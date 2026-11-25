package sema

import "testing"

func TestTryPropagationRules(t *testing.T) {
	cases := map[string]struct {
		src  string
		want string
	}{
		"void function": {
			src:  `fn f() void! { return; }; fn main() void { f()?; };`,
			want: "? in a function that cannot fail",
		},
		"non-call": {
			src:  `fn main() void! { var x = 1; var y = x?; _ = y; };`,
			want: "? expects a call",
		},
		"task handler": {
			src:  `fn f() void! { return; }; fn main() void! { spawn f() on err { f()?; }; };`,
			want: "? cannot leave a spawn task",
		},
		"plain call": {
			src:  `fn p() i32 { return 1; }; fn main() void! { var y = p()?; };`,
			want: "call does not return an error",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := checkSrc(t, map[string]string{".": c.src})
			if !hasError(p, c.want) {
				t.Fatalf("want %q, got %v", c.want, p.Errors)
			}
		})
	}
}

func TestTryNestedExpression(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;
fn f() i32! { return 1; };
fn main() void! {
    print(fmt.Sprint(f()?));
};
`})
	noErrors(t, p)
}

func TestCheckAndPrint(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn try(n: i32) i32! {
    check(n > 0, "positive");
    return n;
};
fn main() void! {
    print("value", try(1)?);
};
`})
	noErrors(t, p)
}
