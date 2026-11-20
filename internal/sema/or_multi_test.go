package sema

import "testing"

func TestOrMultiResult(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use errors;

fn pair(n: i32) i32, bool! {
    if (n < 0) {
        return 0, false, errors.New("neg");
    };
    return n, true;
};

fn pass(n: i32) i32, bool! {
    return pair(n) or fail;
};

fn handle(n: i32) i32, bool! {
    return pair(n) or err {
        return 0, false;
    };
};

fn must(n: i32) i32, bool {
    return pair(n) or panic("bad");
};

fn main() void! {
    var a, b = pair(1) or fail;
    a, b = pair(2) or fail;
    pair(3) or fail;
    var c, d = pair(4) or err {
        return;
    };
    var e, f = pair(5) or panic("bad");
    var g, h, err = pair(6);
    assert(err == nil);
    var i, j = pass(7) or fail;
    var k, l = handle(8) or fail;
    var m, n = must(9);
    _ = a + c + e + g + i + k + m;
    _ = b;
    _ = d;
    _ = f;
    _ = h;
    _ = j;
    _ = l;
    _ = n;
};
`})
	noErrors(t, p)
}

func TestOrMultiArity(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use errors;

fn pair() i32, bool! {
    return 1, true, errors.New("x");
};

fn one() i32! {
    return pair() or fail;
};

fn main() void! {
    var a = pair() or fail;
    var b, c, d = pair() or fail;
    var x, y, z = 0, 0, 0;
    x, y, z = pair() or fail;
};
`})
	for _, want := range []string{
		"wrong number of results: got 2, want 1",
		"wrong number of results: got 2, want 3",
		"wrong number of return values: got 2, want 1",
	} {
		if !hasError(p, want) {
			t.Fatalf("missing error %q: %v", want, p.Errors)
		}
	}
}

func TestOrMultiRejects(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn plain() i32 {
    return 1;
};

fn pair() i32, bool! {
    return 1, true, nil[error];
};

fn main() void {
    var a = plain() or panic("x");
    var b, c = pair() or panic("x");
};
`})
	if !hasError(p, "call does not return an error") {
		t.Fatalf("missing no-error rejection: %v", p.Errors)
	}
	if n := len(p.Errors); n != 1 {
		t.Fatalf("errors = %d, want 1: %v", n, p.Errors)
	}
}

func TestOrMultiNonCall(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void! {
    var x = 1;
    var a = x or fail;
};
`})
	if !hasError(p, "or expects a call") {
		t.Fatalf("missing non-call rejection: %v", p.Errors)
	}
}
