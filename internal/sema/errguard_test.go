package sema

import "testing"

func TestManualErrReturnRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn wrap(err: error) void! {
    if (err != nil) {
        return err;
    };
};

fn wrapZero(err: error) i32! {
    if (err != nil) {
        return 0, err;
    };
    return 0;
};

fn wrapReversed(err: error) void! {
    if (nil != err) {
        return err;
    };
};

fn main() void {};
`})
	got := 0
	for _, e := range p.Errors {
		if contains(e.Msg, "use or fail instead of if err != nil { return err }") {
			got++
		}
	}
	if got != 3 || len(p.Errors) != 3 {
		t.Fatalf("manual propagation errors = %d (all %d), want 3: %v", got, len(p.Errors), p.Errors)
	}
}

func TestErrGuardAllowed(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use fmt;
use errors;

fn success(err: error) void! {
    if (err == nil) {
        return;
    };
    return err;
};

fn inspect(err: error) void! {
    if (err != nil) {
        fmt.Println("caught:", err) or fail;
        return err;
    };
};

fn other(err: error, other: error) void! {
    if (err != nil) {
        return other;
    };
};

fn notNil(p: *i32) *i32 {
    if (p != nil) {
        return p;
    };
    return nil;
};

fn isCheck(err: error) void! {
    if (errors.Is(err, err)) {
        return nil;
    };
};

fn main() void {};
`})
	noErrors(t, p)
}

func TestStringIsNotError(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn f() void! {
    fail("usage");
};

fn main() void {};
`})
	wantError(t, p, "cannot use str as error")
}
