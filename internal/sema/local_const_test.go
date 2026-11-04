package sema

import "testing"

func TestLocalConstRejected(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    const N = 3;
    _ = N;
};
`})
	wantError(t, p, "const is only allowed at package level; use var")
}

func TestPackageConstArrayLength(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `type Foo struct {
    x: i32,
};

const N = 3;

fn main() void {
    var a = Foo[N] {};
    _ = a;
};
`})
	noErrors(t, p)
}

func TestPackageConstMatchPattern(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `const Small = 10;

fn classify(n: i32) str {
    return match (n) {
    case Small => "small";
    else => "big";
    };
};

fn main() void {
    var s = classify(10);
    _ = s;
};
`})
	noErrors(t, p)
}
