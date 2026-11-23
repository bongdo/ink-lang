package gen

import "testing"

func TestGoDetachedReceiverSnapshot(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use time;

type Counter struct {
    N: i32,

    fn Bump(k: i32) void {
        N += k;
    };
};

fn main() void {
    var c = Counter { N: 1 };
    go c.Bump(2);
    time.Sleep(20ms);
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, ":= &c")
	contains(t, src, "go func()")
	if _, stderr, err := p.run(t); err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
}

func TestGoVoidGoFunction(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use time;

fn main() void {
    go time.Sleep(20ms);
};
`,
	})
	if _, stderr, err := p.run(t); err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
}

func TestDiscardFallibleCall(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;

fn load() i32! {
    return 7;
};

fn main() void! {
    _ = load();
    fmt.Println("ok") or fail;
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "ok\n")
}

func TestGoListenerCatchesVoidWorker(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;
use errors;

fn f() i32! {
    fail(errors.New("x"));
};

fn handler() void {
    on err {
        fmt.Println("logged", err)
            on err { _ = err; };
    };
    var a = i32(0);
    go {
        a = f()
            or fail;
    };
    fmt.Println("after", a)
        on err { _ = err; };
};

fn main() void {
    handler();
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "logged x\nafter 0\n")
}

func TestGoInterfaceReceiver(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;
use time;

type Notifier interface {
    Notify(msg: str) void;
};

type Sink struct {
    fn Notify(msg: str) void {
        fmt.Println("sink", msg)
            on err { _ = err; };
    };
};

fn emit(n: Notifier, msg: str) void {
    go n.Notify(msg);
};

fn main() void {
    var s = Sink {};
    emit(&s, "hi");
    time.Sleep(20ms);
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "sink hi\n")
}

func TestGoGlobalFuncSnapshot(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;
use time;

var g: fn(str) void = one;

fn one(s: str) void {
    fmt.Println("one", s)
        on err { _ = err; };
};

fn two(s: str) void {
    fmt.Println("two", s)
        on err { _ = err; };
};

fn main() void {
    go g("x");
    g = two;
    time.Sleep(20ms);
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "one x\n")
}

func TestWrapErrorWithoutFmtImport(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use errors;

fn f() void! {
    fail(errors.New("x"));
};

fn main() void! {
    f()
        or fail("ctx");
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, `_s_fmt "fmt"`)
	contains(t, src, `_s_fmt.Errorf`)
	if _, stderr, err := p.run(t); err == nil {
		t.Fatalf("expected failure, got success\nstderr:\n%s", stderr)
	}
}

func TestGoScopeListenerFailEscalates(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;
use errors;

fn boom() void! {
    return errors.New("boom");
};

fn inner() void! {
    on err { fail(err, "join"); };
    go { boom() or fail; };
};

fn main() void! {
    inner() on err { fmt.Println(err) or fail; };
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "join: boom\n")
}
