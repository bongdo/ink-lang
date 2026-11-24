// Ink 0.2 known problems.
// This file is intentionally invalid: `ink check` should report each
// problem block below. Each block names the current error or limitation.
//
// Syntax-level problems are commented out so the rest of the file still
// parses; uncomment one block at a time to see the diagnostic.
use fmt;
use strconv;

type Foo struct {
    x: i32,
};

type Stack[T] struct {
    data: T[],
};

type E enum {
    A,
    B,
};

fn f() i32! {
    return 1;
};

// 1. const is only allowed at package level.
//    Expected: const is only allowed at package level; use var
fn arrayLocalConst() void {
    const N = 3;
    var a = Foo[N] {};
    _ = a;
};

// 2. A generic type with a value-looking argument is still read as an array.
//    Expected: generic type Stack requires type arguments.
fn genericValueArg() void {
    var a = Stack[3] {};
    _ = a;
};

// 3. A match with a block arm cannot be used as a value.
//    Expected: match with block arms cannot be used as an expression.
fn matchBlockValue() str {
    return match (1) {
        case 1 => "one";
        else => {
            return "other";
        };
    };
};

// 4. Bare nil has no type without an expected type.
//    Expected: cannot infer type of nil; use nil[T] or an expected type.
fn nilNoType() void {
    var p = nil;
    _ = p;
};

// 5. Manual propagation is rejected; use `or fail`.
//    Expected: use or fail instead of if err != nil { return err }.
fn manualPropagation() i32! {
    var v, err = f();
    if (err != nil) {
        return err;
    };
    return v;
};

// 6. A map write inside par is always rejected (G5).
//    Expected: writing to map m inside par.
fn mapParWrite() void {
    var m = map[i32, i32] {};
    par for (var i = 0; i < 3; i += 1) {
        m[i] = i;
    };
};

// 7. par for cannot iterate a map; collect keys into a slice first.
//    Expected: map iteration is not allowed in par for; collect keys into a slice first.
fn goForMap() void {
    var m = map[i32, i32] {};
    par for (var k, v in m) {
        _ = k;
        _ = v;
    };
};

// 8. A write through a captured pointer inside par is rejected (G6).
//    Expected: writing to p captured from outside par.
fn pointerGoWrite() void {
    var p = &Foo { x: 1 };
    par {
        p.x = 2;
    };
};

// 9. break/continue cannot cross a par boundary.
//    Expected: break cannot leave a par block.
fn breakGo() void {
    for (var i = 0; i < 3; i += 1) {
        par {
            break;
        };
    };
};

// 10. An unbounded loop inside par cannot be proven to terminate.
//     Expected: for () is not allowed inside par; use a bounded loop.
fn goEmptyFor() void {
    par {
        for () {
            break;
        };
    };
};

// 11. case nil is not supported on any/interface (use else).
//     Expected: case nil is not supported on any or interface; use else.
fn caseNilAny(v: any) void {
    match (v) {
        case nil => {};
        else => {};
    };
};

// 12. case nil is not supported on an enum.
//     Expected: case nil is not supported on enum E.
fn caseNilEnum(e: E) void {
    match (e) {
        case nil => {};
        else => {};
    };
};

// 13. A constant from a Go package cannot size an array.
//     Expected: constant strconv.IntSize must be declared in a local package.
fn goConstArray() void {
    var a = Foo[strconv.IntSize] {};
    _ = a;
};

// 14. void is only allowed as a function result, not as a value.
//     Expected: void is not a value.
fn note() void {
    return;
};

fn voidValue() void {
    var x = note();
    _ = x;
};

// 15. Interface methods must not use `fn`.
//     Expected: interface methods are declared without 'fn'.
// type BadWriter interface {
//     fn Write(p: u8[]) int!;
// };

// 16. Quoted simple imports are not canonical.
//     Expected: simple import must be bare: use fmt;.
// use "fmt";  // quotes are rejected: write use fmt;

// 17. `or` is statement-level only.
//     Expected: or must be the whole initializer, right side, statement or return value.
// fn nestedOr() i32! {
//     var x = f() or fail + 1;
//     return x;
// };

// 18. A non-void function can fall off its end.
//     Expected: missing return in function returning i32.
fn missingReturn() i32 {};

// 19. A typed var must not report an unknown pointer element twice.
//     Expected: unknown type Missing (one diagnostic).
fn duplicateDiagnostic() void {
    var p: *Missing = nil;
    _ = p;
};

fn main() void {
    _, _ = fmt.Println("problems: run `ink check` for diagnostics", strconv.IntSize);
};
