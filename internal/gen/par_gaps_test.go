package gen

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func exitCode(t *testing.T, err error) int {
	t.Helper()
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("expected an exit error, got %v", err)
	}
	return ee.ExitCode()
}

func TestGoPanicVoidMain(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;

fn boom() void {
    panic("kaboom");
};

fn main() void {
    par {
        boom();
    };
    _, _ = fmt.Println("after");
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err == nil {
		t.Fatalf("expected a panic, stdout:\n%s", stdout)
	}
	if code := exitCode(t, err); code != 2 {
		t.Fatalf("exit code = %d, want 2 (a panic)\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "kaboom") {
		t.Fatalf("stderr does not contain the panic message: %s", stderr)
	}
}

func TestParDeferInside(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;
use errors;

fn stop() void! {
    return errors.New("stop");
};

fn work() void! {
    par {
        {
            defer fmt.Println("cleanup");
            stop() or fail;
            fmt.Println("unreachable") or fail;
        };
    };
    fmt.Println("unreachable") or fail;
};

fn main() void! {
    par {
        {
            defer fmt.Println("d1");
            defer fmt.Println("d2");
            fmt.Println("body") or fail;
        };
    };
    var caught = 0;
    work() on err {
        caught = 1;
    };
    fmt.Println("caught", caught) or fail;
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "body\nd2\nd1\ncleanup\ncaught 1\n")
}

func TestParForCstyleCapture(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;

fn spin() void {
    for (var k = 0; k < 1000000; k += 1) {
    };
};

fn main() void! {
    var out = int[8] {};
    var i = 0;
    par for (i = 0; i < 8; i += 1) {
        spin();
        out[i] = i * 10;
    };
    fmt.Println(i, out[0], out[3], out[7]) or fail;
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "8 0 30 70\n")
}

func TestParRangeError(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use errors;

fn check(k: i32) i32! {
    if (k == 1) {
        return 0, errors.New("key one");
    };
    if (k == 2) {
        return 0, errors.New("key two");
    };
    return k;
};

fn main() void! {
    var ks = i32[] { 1, 2 };
    var out = i32[8] {};
    par for (var i, k in ks) {
        out[k] = check(k) or fail;
    };
    var dummy = out[0] + out[1] + out[2];
    if (dummy < 0) {
        return errors.New("unreachable");
    };
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, ".Add(_s_ord")
	stdout, stderr, err := p.run(t)
	if err == nil {
		t.Fatalf("expected failure, stdout:\n%s", stdout)
	}
	if code := exitCode(t, err); code != 1 {
		t.Fatalf("exit code = %d, want 1\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "key ") {
		t.Fatalf("stderr does not contain a map iteration error: %s", stderr)
	}
}

func TestGoDetachedCallHandler(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;
use errors;
use sync;

fn job(wg: *sync.WaitGroup) void! {
    defer wg.Done();
    return errors.New("boom");
};

fn main() void! {
    var wg = sync.WaitGroup {};
    wg.Add(1);
    spawn job(&wg) on err { _ = fmt.Println("caught"); };
    wg.Wait();
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, "go func() {")
	contains(t, src, "job(")
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "caught\n")
}

func TestGoScopeListenerCatches(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;
use errors;

fn boom() void! {
    return errors.New("boom");
};

fn main() void! {
    var got = false;
    {
        on err {
            got = true;
        };
        par {
            boom() or fail;
        };
    };
    fmt.Println(got) or fail;
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "true\n")
}
