package sema

import (
	"strings"
	"testing"
)

func lineOf(src, sub string) int {
	for i, line := range strings.Split(src, "\n") {
		if strings.Contains(line, sub) {
			return i + 1
		}
	}
	return 0
}

func errorWith(p *Program, sub string) *Error {
	for i := range p.Errors {
		if strings.Contains(p.Errors[i].Msg, sub) {
			return &p.Errors[i]
		}
	}
	return nil
}

func wantError(t *testing.T, p *Program, sub string) *Error {
	t.Helper()
	e := errorWith(p, sub)
	if e == nil {
		t.Fatalf("want error %q, got %v", sub, p.Errors)
	}
	return e
}

func TestIfaceConformanceOK(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `interface Writer {
    Write(p: u8[]) int!;
};

struct Buffer {
    data: u8[],

    fn Write(p: u8[]) int! {
        return len(p);
    };
};

alias AliasWriter = Writer;

fn emit(w: Writer, text: str) int! {
    return w.Write(bytes(text))?;
};

fn forward(w: AliasWriter, text: str) int! {
    return emit(w, text)?;
};

fn mk() Writer {
    var b = Buffer {};
    return &b;
};

fn main() void! {
    var b = Buffer {};
    var n = emit(&b, "x")?;
    var n2 = forward(&b, "y")?;
    var w = Writer(&b);
    var m = mk();
    _ = n;
    _ = n2;
    _ = w;
    _ = m;
};
`})
	noErrors(t, p)
}

func TestIfaceMissingMethod(t *testing.T) {
	src := `interface Writer {
    Write(p: u8[]) int!;
};

struct Empty {
    fn Len() int {
        return 0;
    };
};

fn emit(w: Writer) int! {
    return 0;
};

fn main() void {
    var e = Empty {};
    emit(&e);
};
`
	p := checkSrc(t, map[string]string{".": src})
	e := wantError(t, p, "does not implement Writer")
	if !strings.Contains(e.Msg, "missing method Write") {
		t.Fatalf("message = %q", e.Msg)
	}
	if want := lineOf(src, "emit(&e)"); e.Pos.Line != want {
		t.Fatalf("error at line %d, want %d", e.Pos.Line, want)
	}
}

func TestIfaceWrongParam(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `interface Writer {
    Write(p: u8[]) int!;
};

struct Bad {
    fn Write(p: str) int! {
        return len(p);
    };
};

fn emit(w: Writer) int! {
    return 0;
};

fn main() void {
    var b = Bad {};
    emit(&b);
};
`})
	wantError(t, p, "method Write has parameter 1 of type string, want []uint8")
}

func TestIfaceWrongResult(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `interface Writer {
    Write(p: u8[]) int!;
};

struct Bad {
    fn Write(p: u8[]) bool {
        return true;
    };
};

fn emit(w: Writer) int! {
    return 0;
};

fn main() void {
    var b = Bad {};
    emit(&b);
};
`})
	wantError(t, p, "method Write has result 1 of type bool, want int")
}

func TestIfaceMissingErrorResult(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `interface Writer {
    Write(p: u8[]) int!;
};

struct Bad {
    fn Write(p: u8[]) int {
        return len(p);
    };
};

fn emit(w: Writer) int! {
    return 0;
};

fn main() void {
    var b = Bad {};
    emit(&b);
};
`})
	wantError(t, p, "method Write is missing an error result")
}

func TestIfaceValueReceiver(t *testing.T) {
	src := `interface Writer {
    Write(p: u8[]) int!;
};

struct Buffer {
    data: u8[],

    fn Write(p: u8[]) int! {
        return len(p);
    };
};

fn emit(w: Writer) int! {
    return 0;
};

fn main() void {
    var b = Buffer {};
    _, _ = emit(b);
    var w = Writer(b);
    _ = w;
};
`
	p := checkSrc(t, map[string]string{".": src})
	e := wantError(t, p, "does not implement Writer")
	if !strings.Contains(e.Msg, "missing method Write") {
		t.Fatalf("message = %q", e.Msg)
	}
	if !strings.Contains(e.Msg, "pass a pointer") {
		t.Fatalf("message = %q", e.Msg)
	}
	if want := lineOf(src, "emit(b)"); e.Pos.Line != want {
		t.Fatalf("error at line %d, want %d", e.Pos.Line, want)
	}
	if n := len(p.Errors); n != 2 {
		t.Fatalf("errors = %d, want 2: %v", n, p.Errors)
	}
}

func TestIfaceReturnPosition(t *testing.T) {
	src := `interface Writer {
    Write(p: u8[]) int!;
};

struct Buffer {
    data: u8[],

    fn Write(p: u8[]) int! {
        return len(p);
    };
};

fn mk() Writer {
    var b = Buffer {};
    return b;
};

fn main() void {
    _ = mk();
};
`
	p := checkSrc(t, map[string]string{".": src})
	e := wantError(t, p, "does not implement Writer")
	if want := lineOf(src, "return b;"); e.Pos.Line != want {
		t.Fatalf("error at line %d, want %d", e.Pos.Line, want)
	}
}

func TestIfaceGoNoHiddenCtx(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `interface Writer {
    Write(p: u8[]) int!;
};

struct Good {
    fn Write(p: u8[]) int! {
        par { };
        return len(p);
    };
};

fn emit(w: Writer) int! {
    return w.Write(bytes("x"))?;
};

fn main() void! {
    var g = Good {};
    var n = emit(&g)?;
    _ = n;
};
`})
	noErrors(t, p)
	if d := p.Main.Types["Good"]; d == nil || len(d.Methods) == 0 || d.Methods[0].NeedsCtx {
		t.Fatal("go must not add a context")
	}
}

func TestIfaceCompositeAndAssignment(t *testing.T) {
	src := `interface Writer {
    Write(p: u8[]) int!;
};

struct Buffer {
    data: u8[],

    fn Write(p: u8[]) int! {
        return len(p);
    };
};

struct Holder {
    w: Writer,
};

fn main() void {
    var b = Buffer {};
    var h = Holder { w = &b };
    var hs = Writer[] { &b };
    var hm = map[str, Writer] { "x" = &b };
    var bad = Holder { w = b };
    var bads = Writer[] { b };
    var badm = map[str, Writer] { "x" = b };
    h.w = b;
    _ = hs;
    _ = hm;
    _ = bad;
    _ = bads;
    _ = badm;
};
`
	p := checkSrc(t, map[string]string{".": src})
	if n := len(p.Errors); n != 4 {
		t.Fatalf("errors = %d, want 4: %v", n, p.Errors)
	}
	e := wantError(t, p, "pass a pointer")
	if want := lineOf(src, "var bad = Holder"); e.Pos.Line != want {
		t.Fatalf("error at line %d, want %d", e.Pos.Line, want)
	}
}

func TestIfaceUnexportedMethod(t *testing.T) {
	p := checkSrc(t, map[string]string{
		".": `use ./lib as lib;

struct Impl {
    fn write() void {
    };
};

interface hidden {
    write();
};

fn run(h: hidden) void {
};

fn main() void {
    var i = Impl {};
    run(&i);
    lib.Run(&i);
};
`,
		"lib": `interface Hidden {
    write();
};

fn Run(h: Hidden) void {
};
`,
	})
	wantError(t, p, "does not implement Hidden: method write is not exported")
}

func TestIfaceCrossPackage(t *testing.T) {
	p := checkSrc(t, map[string]string{
		".": `use ./lib as lib;

struct Impl {
    fn Push(m: lib.Msg) int! {
        return 0;
    };
};

fn main() void {
    var i = Impl {};
    lib.Use(&i);
};
`,
		"lib": `struct Msg {
    n: i32,
};

interface Sink {
    Push(m: Msg) int!;
};

fn Use(s: Sink) void {
};
`,
	})
	noErrors(t, p)
}

func TestIfaceGoReader(t *testing.T) {
	good := `use io;

struct Source {
    fn Read(p: u8[]) int! {
        return len(p);
    };
};

fn consume(r: io.Reader) int {
    return 1;
};

fn main() void {
    var s = Source {};
    var n = consume(&s);
    _ = n;
};
`
	p := checkSrc(t, map[string]string{".": good})
	noErrors(t, p)
}

func TestIfaceGoReaderMismatch(t *testing.T) {
	p := checkSrc(t, map[string]string{".": `use io;

struct Source {
    fn Read(p: u8[]) i32 {
        return len(p);
    };
};

fn consume(r: io.Reader) int {
    return 1;
};

fn main() void {
    var s = Source {};
    consume(&s);
};
`})
	wantError(t, p, "does not implement io.Reader: method Read has result 1 of type int32, want int")
}
