package gen

import (
	"strings"
	"testing"
)

func TestGoDetachedReceiverSnapshot(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use time;

struct Counter {
    N: i32,

    fn Bump(k: i32) void {
        N += k;
    };
};

fn main() void {
    var c = Counter { N: 1 };
    spawn c.Bump(2);
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
    spawn time.Sleep(20ms);
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
    fmt.Println("ok")?;
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

fn f() i32! {
    check(false, "x");
};

fn handler() void {
    on err {
        fmt.Println("logged", err)
            on err { _ = err; };
    };
    var a = i32(0);
    par {
        a = f()?;
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

interface Notifier {
    Notify(msg: str) void;
};

struct Sink {
    fn Notify(msg: str) void {
        fmt.Println("sink", msg)
            on err { _ = err; };
    };
};

fn emit(n: Notifier, msg: str) void {
    spawn n.Notify(msg);
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
    spawn g("x");
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

func TestGoScopeListenerHandles(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use errors;

fn boom() void! {
    return errors.New("boom");
};

fn inner() void! {
    on err { print("joined:", err); };
    par { boom()?; };
};

fn main() void! {
    inner()?;
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "joined: boom\n")
}

func TestTryCheckAndPrint(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `fn f(n: i32) i32! {
    check(n >= 0, "negative");
    return n;
};

fn g(n: i32) i32! {
    return f(n)? + 1;
};

fn main() void! {
    print("ok:", g(1)?);
    print("bad:", g(-1)?);
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err == nil {
		t.Fatalf("expected failure, stdout:\n%s", stdout)
	}
	expect(t, stdout, "ok: 2\n")
	if !strings.Contains(stderr, "negative") {
		t.Fatalf("stderr = %q", stderr)
	}
	if code := exitCode(t, err); code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}
