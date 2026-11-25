package sema

import "testing"

func hasError(p *Program, want string) bool {
	for _, e := range p.Errors {
		if contains(e.Msg, want) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return len(sub) == 0
}

func TestMatchAnyRequiresElse(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `
fn f(v: any) str {
    return match (v) {
    case i32 as n => "i";
    };
};
fn main() void {};
`})
	if !hasError(p, "requires else") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestTypePatternOnConcrete(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `
fn f(n: i32) str {
    return match (n) {
    case i32 as x => "i";
    else => "e";
    };
};
fn main() void {};
`})
	if !hasError(p, "type patterns are only allowed") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestNilNotNilable(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `
fn f() i32 {
    return nil;
};
fn main() void {};
`})
	if !hasError(p, "nil is not valid") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestMultiResultArity(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `
fn f() i32, i32 {
    return 1;
};
fn main() void {};
`})
	if !hasError(p, "wrong number of return values") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestMultiResultInference(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `
fn f() i32, bool {
    return 1, true;
};
fn main() void {
    var a, b = f();
    a = a;
    b = b;
};
`})
	noErrors(t, p)
}

func TestTypePatternBinding(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `
fn f(v: any) str {
    return match (v) {
    case i32 as n => "i";
    else => "e";
    };
};
fn main() void {};
`})
	noErrors(t, p)
}

func TestBareNilExpected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `
fn f() error {
    return nil;
};
fn main() void {
    var e = f();
    if (e == nil) {};
};
`})
	noErrors(t, p)
}

func TestGoCallRules(t *testing.T) {
	cases := map[string]struct {
		src  string
		want string
	}{
		"returns value": {
			src: `
fn f() i32 { return 1; };
fn main() void { spawn f(); };
`,
			want: "spawn call must not return a value",
		},
		"not a call": {
			src: `
fn main() void { spawn 1; };
`,
			want: "spawn expects a call",
		},
		"fallible without handler": {
			src: `
fn f() void! { return; };
fn main() void! { spawn f(); };
`,
			want: "spawn call can fail",
		},
		"or fail": {
			src: `
fn f() void! { return; };
fn main() void! { spawn f()?; };
`,
			want: "spawn takes on err",
		},
		"handler on a void call": {
			src: `
fn f() void { return; };
fn main() void { spawn f() on err { _ = err; }; };
`,
			want: "on err on a call that cannot fail",
		},
		"handler captures a local": {
			src: `
fn f() void! { return; };
fn main() void! {
    var x = 1;
    spawn f() on err { _ = x; };
};
`,
			want: "spawn on err cannot capture locals",
		},
		"handler check": {
			src: `
fn f() void! { return; };
fn main() void! { spawn f() on err { check(false, "x"); }; };
`,
			want: "check cannot leave a spawn task",
		},
		"handler returns": {
			src: `
fn f() void! { return; };
fn main() void! { spawn f() on err { return; }; };
`,
			want: "return cannot leave a spawn task",
		},
		"handler empty": {
			src: `
fn f() void! { return; };
fn main() void! { spawn f() on err {}; };
`,
			want: "on err must do something",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := checkSrc(t, map[string]string{".": c.src})
			if !hasError(p, c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, p.Errors)
			}
		})
	}
}

func TestOnErrListenerRules(t *testing.T) {
	cases := map[string]struct {
		src  string
		want string
	}{
		"empty": {
			src:  `fn main() void { on err {}; };`,
			want: "on err must do something",
		},
		"not first": {
			src: `
fn main() void {
    var x = 1;
    on err { _ = x; };
};
`,
			want: "on err must be the first statement",
		},
		"return": {
			src: `
fn f() void! { on err { return; }; };
fn main() void {};
`,
			want: "return cannot leave an on err listener",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := checkSrc(t, map[string]string{".": c.src})
			if !hasError(p, c.want) {
				t.Fatalf("want error containing %q, got %v", c.want, p.Errors)
			}
		})
	}
}

func TestOnErrListenerCatchesGoError(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn boom() void! { check(false, "x"); };
fn main() void! {
    on err { panic(err); };
    par { boom()?; };
};
`})
	noErrors(t, p)
}

func TestDroppedFallibleRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `
fn f() i32! { return 1; };
fn main() void { f(); };
`})
	if !hasError(p, "fallible result is dropped") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestCheckNeedsFallibleFunction(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f() void { on err { check(false, "x"); }; };
fn main() void {};
`})
	if !hasError(p, "check in a function that cannot fail") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestErrorConstructorRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `
fn f() void! {
    var e = error("x");
    _ = e;
};
fn main() void! { f()?; };
`})
	if !hasError(p, "error is a type, not a constructor") {
		t.Fatalf("errors = %v", p.Errors)
	}
}
