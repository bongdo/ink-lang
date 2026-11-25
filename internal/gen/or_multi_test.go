package gen

import "testing"

func TestOrMultiResult(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;

fn pair(n: i32) i32, bool! {
    check(n >= 0, "neg");
    return n, true;
};

fn pass(n: i32) i32, bool! {
    return pair(n)?;
};

fn runPar(n: i32) i32, bool! {
    var a = i32(0);
    var b = false;
    par {
        a, b = pair(n)?;
    };
    return a, b;
};

fn main() void! {
    var a, b = pair(2)?;
    fmt.Println("var", a, b)?;
    a, b = pair(3)?;
    fmt.Println("assign", a, b)?;
    pair(4)?;
    var c, d = pass(5)?;
    fmt.Println("pass", c, d)?;
    var e, f = pair(-1) on err {
        fmt.Println("caught:", err)?;
    };
    fmt.Println("handle", e, f)?;
    var g, h = runPar(6)?;
    fmt.Println("go", g, h)?;
    var i, j = runPar(-1) on err {
        fmt.Println("go caught:", err)?;
    };
    fmt.Println("go zero", i, j)?;
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
	expect(t, stdout, "var 2 true\nassign 3 true\npass 5 true\ncaught: neg\nhandle 0 false\ngo 6 true\ngo caught: neg\ngo zero 0 false\n")
}
