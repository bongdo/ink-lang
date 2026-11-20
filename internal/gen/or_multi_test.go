package gen

import "testing"

func TestOrMultiResult(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;
use errors;

fn pair(n: i32) i32, bool! {
    if (n < 0) {
        return 0, false, errors.New("neg");
    };
    return n, true;
};

fn pass(n: i32) i32, bool! {
    return pair(n) or fail;
};

fn runPar(n: i32) i32, bool! {
    var a = i32(0);
    var b = false;
    par {
        a, b = pair(n) or fail;
    };
    return a, b;
};

fn runPanic() void! {
    par {
        var _, _ = pair(-1) or panic("wrapped");
    };
};

fn main() void! {
    var a, b = pair(2) or fail;
    fmt.Println("var", a, b) or fail;
    a, b = pair(3) or fail;
    fmt.Println("assign", a, b) or fail;
    pair(4) or fail;
    var c, d = pass(5) or fail;
    fmt.Println("pass", c, d) or fail;
    var e, f = pair(-1) or err {
        fmt.Println("caught:", err) or fail;
    };
    fmt.Println("handle", e, f) or fail;
    var g, h = runPar(6) or fail;
    fmt.Println("par", g, h) or fail;
    var i, j = runPar(-1) or err {
        fmt.Println("par caught:", err) or fail;
    };
    fmt.Println("par zero", i, j) or fail;
    var caught = false;
    runPanic() or err { caught = true; };
    fmt.Println("panic", caught) or fail;
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, "_, _, _s_err")
	contains(t, src, "*new(int32), *new(bool), _s_err")
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("panic reached stderr: %s", stderr)
	}
	expect(t, stdout, "var 2 true\nassign 3 true\npass 5 true\ncaught: neg\nhandle 0 false\npar 6 true\npar caught: neg\npar zero 0 false\npanic true\n")
}
