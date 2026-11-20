package sema

import "testing"

func TestMatchBlockArmsAsValueRejected(t *testing.T) {
	cases := map[string]string{
		"return": `fn f(n: i32) str {
    return match (n) { case 0 => "zero"; else => { return "big"; }; };
};
`,
		"var": `fn f(n: i32) str {
    var s = match (n) { case 0 => "zero"; else => { return "big"; }; };
    return s;
};
`,
		"assign": `fn f(n: i32) str {
    var s = "";
    s = match (n) { case 0 => "zero"; else => { return "big"; }; };
    return s;
};
`,
		"call argument": `fn id(s: str) str {
    return s;
};

fn f(n: i32) str {
    return id(match (n) { case 0 => "zero"; else => { return "big"; }; });
};
`,
		"nested expression": `fn f(n: i32) str {
    return match (n) {
    case 0 => match (n) { case 1 => "one"; else => { return "big"; }; };
    else => "other";
    };
};
`,
		"expression statement": `fn id(s: str) str {
    return s;
};

fn f(n: i32) void {
    id(match (n) { case 0 => "zero"; else => { return; }; });
};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			p := checkSrc(t, map[string]string{".": src})
			if !hasError(p, "match with block arms cannot be used as an expression") {
				t.Fatalf("errors = %v", p.Errors)
			}
		})
	}
}

func TestMatchBlockArmsInOrHandlerAllowed(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use errors;

fn load() str! {
    return errors.New("x");
};

fn f() str! {
    var s = load() or err {
        match (1) {
        case 1 => { return "one"; };
        else => { return "other"; };
        };
    };
    return s;
};

fn main() void {};
`})
	noErrors(t, p)
}

func TestMatchBlockArmsInPackageVarRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `var x = match (1) {
    case 1 => "one";
    else => { "two"; };
};

fn main() void {};
`})
	if !hasError(p, "match with block arms cannot be used as an expression") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestMatchBlockArmsAsStatementAllowed(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f(n: i32) str {
    match (n) {
    case 0 => { return "zero"; };
    else => { return "big"; };
    };
    return "other";
};

fn main() void {};
`})
	noErrors(t, p)
}

func TestMatchExpressionArmsAllowed(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f(n: i32) str {
    return match (n) { case 0 => "zero"; else => "big"; };
};

fn main() void {};
`})
	noErrors(t, p)
}

func TestMatchNilPattern(t *testing.T) {
	cases := map[string]string{
		"pointer": `fn f(p: *i32) str {
    return match (p) { case nil => "nil"; else => "set"; };
};

fn main() void {};
`,
		"slice": `fn f(p: i32[]) str {
    return match (p) { case nil => "nil"; else => "set"; };
};

fn main() void {};
`,
		"map": `fn f(p: map[i32, i32]) str {
    return match (p) { case nil => "nil"; else => "set"; };
};

fn main() void {};
`,
		"func": `fn f(p: fn() void) str {
    return match (p) { case nil => "nil"; else => "set"; };
};

fn main() void {};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			p := checkSrc(t, map[string]string{".": src})
			noErrors(t, p)
		})
	}
}

func TestMatchNilPatternRejectedOnValue(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f(n: i32) str {
    return match (n) { case nil => "nil"; else => "set"; };
};
`})
	if !hasError(p, "nil is not valid for i32") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestMatchNilPatternOnAnyRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f(v: any) str {
    return match (v) { case nil => "nil"; else => "set"; };
};
`})
	if !hasError(p, "case nil is not supported on any or interface") {
		t.Fatalf("errors = %v", p.Errors)
	}
}

func TestMatchNilPatternOnEnumRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type E enum { A, B };

fn f(e: E) str {
    return match (e) { case nil => "nil"; else => "set"; };
};
`})
	if !hasError(p, "case nil is not supported on enum E") {
		t.Fatalf("errors = %v", p.Errors)
	}
}
