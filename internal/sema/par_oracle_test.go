package sema

import "testing"

func parCheck(t *testing.T, src string) *Program {
	t.Helper()
	return checkSrc(t, map[string]string{".": src})
}

func parWant(t *testing.T, src, want string) {
	t.Helper()
	p := parCheck(t, src)
	if !hasError(p, want) {
		t.Fatalf("want error containing %q, got %v", want, p.Errors)
	}
}

func parOK(t *testing.T, src string) {
	t.Helper()
	p := parCheck(t, src)
	if len(p.Errors) == 0 {
		return
	}
	noErrors(t, p)
}

func TestParMapWriteRejected(t *testing.T) {
	cases := map[string]string{
		"plain go": `fn main() void {
    var m = map[i32, i32] {};
    go {
        m[0] = 1;
    };
};
`,
		"range map": `fn main() void {
    var m = map[i32, i32] { 1 = 1 };
    go for (var k, v in m) {
        m[k] = v;
    };
};
`,
		"c-style": `fn main() void {
    var m = map[i32, i32] {};
    go for (var i = 0; i < 2; i += 1) {
        m[i] = i;
    };
};
`,
		"nested through map": `fn main() void {
    var mm = map[i32, i32[]] {};
    go for (var i = 0; i < 2; i += 1) {
        mm[i][0] = i;
    };
};
`,
		"disjoint key map": `fn main() void {
    var xs = i32[] {1, 2};
    var out = map[i32, i32] {};
    go for (var i, x in xs) {
        out[i] = x;
    };
};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			parWant(t, src, "writing to map")
		})
	}
}

func TestParPointerWriteRejected(t *testing.T) {
	cases := map[string]string{
		"field": `type C struct {
    n: i32,
};

fn main() void {
    var c = C { n = 0 };
    var p = &c;
    go {
        p.n = 1;
    };
};
`,
		"deref": `fn main() void {
    var n = 0;
    var p = &n;
    go {
        *p = 2;
    };
};
`,
		"slot": `fn main() void {
    var xs = i32[2] {};
    var p = &xs;
    go {
        p[0] = 3;
    };
};
`,
		"param": `type C struct {
    n: i32,
};

fn bump(p: *C) void {
    go {
        p.n = 1;
    };
};

fn main() void {
    var c = C { n = 0 };
    bump(&c);
};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			parWant(t, src, "captured from outside go")
		})
	}
}

func TestParAliasWriteRejected(t *testing.T) {
	cases := map[string]string{
		"alias outside": `type C struct {
    n: i32,
};

fn main() void {
    var c = C { n = 0 };
    var p = &c;
    var q = p;
    go {
        q.n = 1;
    };
};
`,
		"alias inside": `type C struct {
    n: i32,
};

fn main() void {
    var c = C { n = 0 };
    var p = &c;
    go {
        {
            var q = p;
            q.n = 1;
        };
    };
};
`,
		"chained alias inside": `type C struct {
    n: i32,
};

fn main() void {
    var c = C { n = 0 };
    var p = &c;
    go {
        {
            var q = p;
            var r = q;
            r.n = 1;
        };
    };
};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			parWant(t, src, "captured from outside go")
		})
	}
}

func TestParMethodWriteRejected(t *testing.T) {
	cases := map[string]string{
		"pointer receiver": `type C struct {
    n: i32,

    fn Bump() void {
        n += 1;
    };
};

fn main() void {
    var c = C { n = 0 };
    var p = &c;
    go {
        p.Bump();
    };
};
`,
		"self": `type C struct {
    n: i32,

    fn Bump() void {
        n += 1;
    };

    fn Run() void {
        go {
            self.Bump();
        };
    };
};

fn main() void {
    var c = C { n = 0 };
    c.Run();
};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			want := "captured from outside go"
			if name == "self" {
				want = "writing to self inside go"
			}
			parWant(t, src, want)
		})
	}
}

func TestParSharedSliceRejected(t *testing.T) {
	cases := map[string]string{
		"two slots": `fn main() void {
    var out = i32[2] {};
    go {
        out[0] = 1;
        out[1] = 2;
    };
};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			parWant(t, src, "captured from outside go")
		})
	}
}

func TestParReadWriteRejected(t *testing.T) {
	cases := map[string]string{
		"read after write": `fn id(n: i32) i32 {
    return n;
};

fn main() void {
    var a = 0;
    var b = 0;
    go {
        a = id(1);
        b = a + 1;
    };
};
`,
		"read as argument": `fn id(n: i32) i32 {
    return n;
};

fn main() void {
    var a = 0;
    var b = 0;
    go {
        a = id(1);
        b = id(a);
    };
};
`,
		"read in condition": `fn id(n: i32) i32 {
    return n;
};

fn main() void {
    var a = 0;
    var b = 0;
    go {
        a = id(1);
        if (a > 0) {
            b = 2;
        };
    };
};
`,
		"read in nested loop": `fn id(n: i32) i32 {
    return n;
};

fn main() void {
    var a = 0;
    var b = 0;
    go {
        a = id(1);
        for (var k = 0; k < a; k += 1) {
            b += k;
        };
    };
};
`,
		"slot written and read": `fn main() void {
    var out = i32[2] {};
    var n = 0;
    go {
        out[0] = 1;
        n = out[1];
    };
};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			parWant(t, src, "read by another go statement")
		})
	}
}

func TestParWriteSetsAllowed(t *testing.T) {
	cases := map[string]string{
		"distinct writers": `fn id(n: i32) i32 {
    return n;
};

fn main() void {
    var a = 0;
    var b = 0;
    go {
        a = id(1);
        b = id(2);
    };
};
`,
		"readers only": `fn main() void {
    var a = 1;
    var b = 0;
    var c = 0;
    go {
        b = a + 1;
        c = a + 2;
    };
};
`,
		"single writer": `fn id(n: i32) i32 {
    return n;
};

fn main() void {
    var a = 0;
    go {
        a = id(1);
    };
};
`,
		"single statement reads and writes": `fn main() void {
    var a = 0;
    go {
        a += 1;
    };
};
`,
		"single slice slot": `fn main() void {
    var out = i32[1] {};
    go {
        out[0] = 1;
    };
};
`,
		"value struct method": `type C struct {
    n: i32,

    fn Bump() void {
        n += 1;
    };
};

fn main() void {
    var c = C { n = 0 };
    go {
        c.Bump();
    };
};
`,
		"disjoint slots": `fn id(n: i32) i32 {
    return n;
};

fn main() void {
    var xs = i32[] {1, 2, 3};
    var out = i32[3] {};
    go for (var i, x in xs) {
        out[i] = id(x);
    };
};
`,
		"map reads": `fn main() void {
    var m = map[i32, i32] { 1 = 2, 2 = 3 };
    var ks = i32[] {1, 2};
    var out = i32[4] {};
    go for (var i, k in ks) {
        out[i] = m[k];
    };
};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			parOK(t, src)
		})
	}
}

func TestParAllowsDefer(t *testing.T) {
	cases := map[string]string{
		"go": `fn note() void {
};

fn main() void {
    go {
        defer note();
        note();
    };
};
`,
		"go for": `fn note() void {
};

fn main() void {
    go for (var i = 0; i < 2; i += 1) {
        defer note();
        note();
    };
};
`,
		"with defer and or fail": `fn note() void {
};

fn load() i32! {
    return 1;
};

fn main() void! {
    var a = 0;
    go {
        defer note();
        a = load() or fail;
    };
    a += 1;
};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			parOK(t, src)
		})
	}
}

func TestParForCstyleIterVar(t *testing.T) {
	parOK(t, `fn main() void {
    var out = i32[4] {};
    var i = 0;
    go for (i = 0; i < 4; i += 1) {
        out[i] = i * 2;
    };
};
`)
	parWant(t, `fn main() void {
    var i = 0;
    go for (i = 0; i < 4; i += 1) {
        i = i + 1;
    };
};
`, "captured from outside go")
}

func TestParBoundaryStillRejected(t *testing.T) {
	parWant(t, `fn main() void {
    go {
        return;
    };
};
`, "cannot leave a go block")
	parWant(t, `fn main() void {
    var out = i32[2] {};
    go for (var i = 0; i < 2; i += 1) {
        out[i] = i;
        break;
    };
};
`, "break cannot leave a go block")
	parWant(t, `fn main() void {
    var out = i32[2] {};
    go for (var i = 0; i < 2; i += 1) {
        out[i] = i;
        continue;
    };
};
`, "continue cannot leave a go block")
}

func TestParBreakContinueNestedLoopAllowed(t *testing.T) {
	cases := map[string]string{
		"go block": `fn main() void {
    go {
        for (var i = 0; i < 4; i += 1) {
            if (i == 2) {
                break;
            };
            if (i == 1) {
                continue;
            };
        };
    };
};
`,
		"go for": `fn main() void {
    go for (var i = 0; i < 4; i += 1) {
        for (var j = 0; j < 4; j += 1) {
            if (j == 2) {
                break;
            };
            if (j == 1) {
                continue;
            };
        };
    };
};
`,
		"loop around go": `fn main() void {
    for (var i = 0; i < 2; i += 1) {
        go {
            for (var j = 0; j < 4; j += 1) {
                if (j == 2) {
                    break;
                };
                if (j == 1) {
                    continue;
                };
            };
        };
    };
};
`,
		"go for inside loop": `fn main() void {
    for (var i = 0; i < 2; i += 1) {
        go for (var j = 0; j < 4; j += 1) {
            for (var k = 0; k < 4; k += 1) {
                if (k == 2) {
                    break;
                };
                if (k == 1) {
                    continue;
                };
            };
        };
    };
};
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			parOK(t, src)
		})
	}
}

func TestParBreakContinueCrossingRejected(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "plain go break",
			src: `fn main() void {
    go {
        break;
    };
};
`,
			want: "break cannot leave a go block",
		},
		{
			name: "plain go continue",
			src: `fn main() void {
    go {
        continue;
    };
};
`,
			want: "continue cannot leave a go block",
		},
		{
			name: "loop break out of go",
			src: `fn main() void {
    for (var i = 0; i < 2; i += 1) {
        go {
            break;
        };
    };
};
`,
			want: "break cannot leave a go block",
		},
		{
			name: "loop continue out of go",
			src: `fn main() void {
    for (var i = 0; i < 2; i += 1) {
        go {
            continue;
        };
    };
};
`,
			want: "continue cannot leave a go block",
		},
		{
			name: "break out of nested go",
			src: `fn main() void {
    go {
        for (var i = 0; i < 2; i += 1) {
            go {
                break;
            };
        };
    };
};
`,
			want: "break cannot leave a go block",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parWant(t, tc.src, tc.want)
		})
	}
}
