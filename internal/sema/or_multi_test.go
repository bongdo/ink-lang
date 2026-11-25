package sema

import "testing"

func TestOrMultiResult(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn pair(n: i32) i32, bool! {
    check(n >= 0, "neg");
    return n, true;
};

fn pass(n: i32) i32, bool! {
    return pair(n)?;
};

fn handle(n: i32) i32, bool! {
    return pair(n) on err {
        return 0, false;
    };
};

fn must(n: i32) i32, bool! {
    return pair(n)?;
};

fn main() void! {
    var a, b = pair(1)?;
    a, b = pair(2)?;
    pair(3)?;
    var c, d = pair(4) on err {
        return;
    };
    var e, f = pair(5)?;
    var g, h, err = pair(6);
    assert(err == nil);
    var i, j = pass(7)?;
    var k, l = handle(8)?;
    var m, n = must(9)?;
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
	p := checkSrc(t, map[string]string{".": `fn pair() i32, bool! {
    check(false, "x");
    return 1, true;
};

fn one() i32! {
    return pair()?;
};

fn main() void! {
    var a = pair()?;
    var b, c, d = pair()?;
    var x, y, z = 0, 0, 0;
    x, y, z = pair()?;
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

fn main() void! {
    var a = plain()?;
    var b, c = pair()?;
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
    var a = x?;
};
`})
	if !hasError(p, "? expects a call") {
		t.Fatalf("missing non-call rejection: %v", p.Errors)
	}
}
