package sema

import "testing"

func TestSliceCtorType(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var xs = i32[](3);
    var ys = u8[](cap: 64);
    _ = xs;
    _ = ys;
};
`})
	noErrors(t, p)
}

func TestSliceCtorNeedsSizeOrCap(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var xs = i32[]();
    _ = xs;
};
`})
	wantError(t, p, "needs a size or a cap")
}

func TestSliceCtorCapBelowSize(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    var xs = i32[](cap: 2, size: 8);
    _ = xs;
};
`})
	wantError(t, p, "cap 2 is smaller than size 8")
}

func TestSliceCtorInArena(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    arena {
        var xs = i32[](4);
        xs[0] = 1;
    };
};
`})
	noErrors(t, p)
}

func TestSliceCtorCapInsideArena(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `fn main() void {
    arena {
        var xs = i32[](1, cap: 8);
        xs[0] = 1;
    };
};
`})
	noErrors(t, p)
}
