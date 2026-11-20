package gen

import "testing"

func TestTypedVarNil(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;

type Point struct {
    X: i32,
};

var gp: *Point = nil;
var ge: error = nil;

fn main() void! {
    var p: *Point = nil;
    var s: i32[] = nil;
    var m: map[str, i32] = nil;
    var h: fn(i32) i32 = nil;
    var e: error = nil;
    if (p == nil) {
        fmt.Println("p nil") or fail;
    };
    if (e == nil) {
        fmt.Println("e nil") or fail;
    };
    if (gp == nil) {
        fmt.Println("gp nil") or fail;
    };
    if (ge == nil) {
        fmt.Println("ge nil") or fail;
    };
    _ = s;
    _ = m;
    _ = h;
    return;
};
`,
	})
	src := p.source(t, "zz_ink.go")
	for _, want := range []string{
		"var p *Point = (*Point)(nil)",
		"var e error = (error)(nil)",
		"var gp *Point = (*Point)(nil)",
		"var ge error = (error)(nil)",
		"var s []int32 = ([]int32)(nil)",
		"var m map[string]int32 = (map[string]int32)(nil)",
		"var h func(int32) int32 = (func(int32) int32)(nil)",
	} {
		contains(t, src, want)
	}
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "p nil\ne nil\ngp nil\nge nil\n")
}

func TestTypedVarOr(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `type Point struct {
    X: i32,
};

fn make(ok: bool) *Point! {
    if (ok) {
        return &Point { X = 1 };
    };
    return nil;
};

fn main() void! {
    var p: *Point = make(true) or fail;
    var q: *Point = make(false) or panic("no");
    assert(p != nil);
    assert(q == nil);
    return;
};
`,
	})
	contains(t, p.source(t, "zz_ink.go"), "var p *Point = ")
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "")
}

func TestTypedVarForInit(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `fn main() void {
    var sum: i32 = 0;
    for (var i: i32 = 0; i < 3; i += 1) {
        sum += i;
    };
    assert(sum == 3);
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, "var i int32 = 0")
	contains(t, src, "for ; i < 3; i += 1 {")
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "")
}

func TestTypedVarParForInit(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `fn main() void {
    par for (var i: i32 = 0; i < 3; i += 1) {
        assert(i >= 0);
    };
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, "var i int32 = 0")
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "")
}

func TestTypedVarMultiNames(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `fn pair() i32, i32 {
    return 1, 2;
};

fn main() void {
    var a, b: i32 = pair();
    assert(a + b == 3);
};
`,
	})
	contains(t, p.source(t, "zz_ink.go"), "var a, b int32 = pair()")
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "")
}
