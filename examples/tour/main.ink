// Ink 0.2 syntax tour.
// Valid program: `ink check`, `ink run`, `ink test`.
// Problem constructs are in ../problems/main.ink.
use strings;
use slices;
use ./lib;

// Constants are compile-time values; `const` can size an array.
const Answer = 42;
const Name = "ink";
const N = 3;

var packageVar = i32(7);

// Aliases do not create a new Go type.
alias UserId = u64;
alias Handler = fn(i32) i32;
alias PairFn = fn(i32) (str, bool);

// Struct: fields and methods live in the type body; `self` is implicit.
struct Point {
    x: i32,
    y: i32,

    fn Move(dx: i32, dy: i32) void {
        x += dx;
        y += dy;
    };

    fn Sum() i32 {
        return x + y;
    };
};

// Enum is a sealed sum type. `match` must be exhaustive.
enum Shape {
    Circle { radius: f64 },
    Rect { w: f64, h: f64 },
    Empty,
};

fn area(s: Shape) f64 {
    return match (s) {
        case Circle { radius } => 3.14159 * radius * radius;
        case Rect { w, h } => w * h;
        case Empty => 0.0;
    };
};

// Interfaces are structural; methods are signatures, without `fn`.
interface Writer {
    Write(p: u8[]) int!;
};

struct Buffer {
    data: u8[],

    fn Write(p: u8[]) int! {
        data = append(data, p...);
        return len(p);
    };
};

fn emit(w: Writer, text: str) int! {
    return w.Write(bytes(text));
};

// Generics: type parameters are written in `[...]`.
struct Stack[Item] {
    data: Item[],

    fn Push(x: Item) void {
        data = append(data, x);
    };

    fn Pop() Item {
        var x = data[len(data) - 1];
        data = data[:len(data) - 1];
        return x;
    };
};

fn Map[In, Out](xs: In[], f: fn(In) Out) Out[] {
    var out = Out[] {};
    for (var x in xs) {
        out = append(out, f(x));
    };;
    return out;
};

// Multiple results and fallible results.
fn pair() i32, bool {
    return 1, true;
};

fn mayFail(ok: bool) i32! {
    check(ok, "requested failure");
    return 7;
};

fn split(n: i32) (i32, bool)! {
    check(n >= 0, "negative");
    return n, true;
};

fn propagate(ok: bool) i32! {
    var v = mayFail(ok)?;
    return v + 1;
};

fn passthrough(n: i32) (i32, bool)! {
    return split(n)?;
};

fn applyPair(f: PairFn, n: i32) str, bool {
    return f(n);
};

// `defer` is block-scoped: this runs at the end of the `if` block.
fn deferPrint() void {
    print("defer: block exit");
};

fn deferDemo() void {
    if (true) {
        defer deferPrint();
        print("defer: inside block");
    };
    print("defer: after block");
};

fn handleDemo() i32 {
    var v = mayFail(false)
        on err {
            print("on err: handled", err);
            return -1;
        };
    return v;
};

fn panicDemo() i32 {
    var v = mayFail(true)
        on err { panic("unreachable"); };
    return v;
};

// `match` on `any` uses type patterns.
fn describe(v: any) str {
    return match (v) {
        case i32 as n => "i32";
        case str as s => "str";
        case *Point as p => "point";
        else => "other";
    };
};

// Literal patterns include negative numbers and nil.
fn sign(n: i32) str {
    return match (n) {
        case -1 => "negative";
        case 0 => "zero";
        else => "positive";
    };
};

fn maybe(p: *Point) str {
    return match (p) {
        case nil => "nil";
        else => "set";
    };
};

// Test blocks are run by `ink test`, not by `ink run`.
test "append grows the slice" {
    var xs = i32[] {};
    xs = append(xs, 1);
    assert_eq(len(xs), 1);
};

test "min and max" {
    assert(min(1, 2) == 1);
    assert_eq(max(1, 2), 2);
};

fn main() void! {
    // Constants, variables, aliases, conversions.
    var answer = Answer;
    var user = UserId(7);
    var name = strings.ToUpper(Name);
    print(answer, user, name, packageVar);

    // Struct literal, methods, implicit self.
    var p = Point { x: 1, y: 2 };
    p.Move(3, 4);
    print(p.Sum());

    // Pointers: `&`, `*`, auto-deref, write through pointer.
    var pp = &p;
    pp.x = 10;
    var snapshot = *pp;
    *pp = Point { x: 20, y: 30 };
    print(p.x, snapshot.x);

    // Enum construction and exhaustive match expression.
    var shapes = Shape[] {
        Shape.Circle { radius: 2.0 },
        Shape.Rect { w: 2.0, h: 3.0 },
        Shape.Empty,
    };
    var areas = f64[] {};
    for (var s in shapes) {
        areas = append(areas, area(s));
    };;
    print(areas);

    // Match statement with block arms.
    match (shapes[0]) {
        case Circle { radius } => {
            print("circle", radius);
        };
        case Rect { w, h } => {
            print("rect", w, h);
        };
        case Empty => {
            print("empty");
        };
    };

    // Interface and structural implementation.
    var b = Buffer {};
    var n = emit(&b, "hello")?;
    print(n, str(b.data));

    // Generics: inference and explicit type arguments.
    var stack = Stack[i32] {};
    stack.Push(1);
    stack.Push(2);
    print(stack.Pop(), stack.Pop());
    var doubled = Map[i32, i32](i32[] { 1, 2, 3 }, fn(x: i32) i32 => x * 2);
    print(doubled);

    // Multiple results, fallible multiple results, `or` forms.
    var a, ok = pair();
    print(a, ok);

    var x, y = split(7)?;
    print(x, y);

    var x2, y2 = passthrough(8)?;
    print(x2, y2);

    var v = propagate(true)?;
    print(v);

    var handled = handleDemo();
    print(handled);

    var panicked = panicDemo();
    print(panicked);

    // `on err` handles the error here and keeps the previous value.
    var fallback = i32(0);
    fallback = mayFail(false)
        on err { print("fallback", err); };
    print(fallback);

    // Block-scoped defer.
    deferDemo();

    // if / else if / else, break, continue.
    var total = i32(0);
    for (var i = 0; i < 10; i += 1) {
        if (i == 3) {
            continue;
        };
        if (i == 8) {
            break;
        };
        total += i32(i);
    };;
    print(total);

    // Range over slice with index, map with key/value, string with rune.
    var xs = i32[] { 10, 20, 30 };
    for (var i, x in xs) {
        total += i32(i) + x;
    };;
    var m = map[str, i32] { "a": 1, "bb": 2 };
    for (var k, v in m) {
        total += v + i32(len(k));
    };;
    for (var r in "ab") {
        total += i32(r);
    };;
    print(total);

    // Infinite loop with a local break.
    var loops = 0;
    for () {
        loops += 1;
        if (loops == 3) {
            break;
        };
    };;
    print(loops);

    // Slices, arrays, local import constant, map builtins.
    var arr = Point[N] {};
    var imported = Point[lib.N] {};
    var slice = xs[1:3];
    var buf = u8[8] {};
    var copied = copy(buf[:], bytes("hi"));
    var capValue = cap(xs);
    var present = has(m, "a");
    delete(m, "a");
    print(len(arr), len(imported), len(slice), copied, capValue, present);
    print(lib.Double(21));

    // Conversions and builtins.
    var text = str(runes("hé"));
    var raw = bytes("hi");
    var low = min(3, 5);
    var high = max(3, 5);
    print(text, len(raw), low, high);

    // Typed vars give bare nil a type. Without an expected type, use nil[T]
    // (see maybe(nil[*Point]) below).
    var np: *Point = nil;
    var ns: i32[] = nil;
    var nm: map[str, i32] = nil;
    var nf: fn(i32) i32 = nil;
    var ne: error = nil;
    print(np == nil, ns == nil, nm == nil, nf == nil, ne == nil);

    // Function values and type aliases.
    var h = fn(x: i32) i32 => x + 1;
    var apply = fn(f: Handler, v: i32) i32 => f(v);
    print(apply(h, 41));

    var pf = fn(n: i32) str, bool {
        return "pair", n > 0;
    };
    var ps, pok = applyPair(pf, 1);
    print(ps, pok);

    // Type patterns on `any`.
    print(describe(i32(1)), describe("s"), describe(&p), describe(true));

    // Negative and nil patterns.
    print(sign(-1), sign(0), sign(1));
    print(maybe(nil[*Point]), maybe(&p));

    // Go interop with a generic stdlib callback.
    var nums = i32[] { 3, 1, 2 };
    slices.SortFunc(nums, fn(a: i32, b: i32) int => int(a - b));
    print(nums);

    // par: each top-level statement is a worker.
    var left = i32(0);
    var right = i32(0);
    par {
        left = mayFail(true)?;
        right = mayFail(true)?;
    };
    print(left, right);

    // par for: disjoint slots, explicit limit.
    var out = i32[4] {};
    par for (var i, x in i32[] { 1, 2, 3, 4 }) limit 2 {
        out[i] = x * 2;
    };;
    print(out);

    // A nested block scopes its own defer and locals.
    {
        var w = mayFail(true)?;
        print("scope", w);
    };

    return;
};
