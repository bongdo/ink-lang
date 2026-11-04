package gen

import "testing"

func TestOrMultiResult(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;
use errors;

fn pair(n: i32) i32, bool or error {
    if (n < 0) {
        return 0, false, errors.New("neg");
    };
    return n, true;
};

fn pass(n: i32) i32, bool or error {
    return pair(n) or return;
};

fn runPar(n: i32) i32, bool or error {
    var a = i32(0);
    var b = false;
    par {
        a, b = pair(n) or return;
    };
    return a, b;
};

fn runPanic() void or error {
    par {
        var _, _ = pair(-1) or panic("wrapped");
    };
};

fn main() void or error {
    var a, b = pair(2) or return;
    fmt.Println("var", a, b) or return;
    a, b = pair(3) or return;
    fmt.Println("assign", a, b) or return;
    pair(4) or return;
    var c, d = pass(5) or return;
    fmt.Println("pass", c, d) or return;
    var e, f = pair(-1) or err {
        fmt.Println("caught:", err) or return;
    };
    fmt.Println("handle", e, f) or return;
    var g, h = runPar(6) or return;
    fmt.Println("par", g, h) or return;
    var i, j = runPar(-1) or err {
        fmt.Println("par caught:", err) or return;
    };
    fmt.Println("par zero", i, j) or return;
    var caught = false;
    runPanic() or err { caught = true; };
    fmt.Println("panic", caught) or return;
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
