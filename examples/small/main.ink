// Ink 0.2: every construct in one file. `ink check`, `ink test`, `ink run`.
use fmt;
use strings;
use slices;
use errors;
use encoding/json;
use sync;
use ./lib;

const Answer = 42;

var packageVar = i32(7);

type UserId = u64;

type Handler = fn(i32) i32;

// Struct: fields, methods with implicit self, JSON contract blocks.
type User struct {
    Name: str,
    Pass: str,

    fn Greet() str {
        return "hi, " + Name;
    };

    wire {
        Name: "name",
    };

    admin {
        Name: "name",
        Pass: "pass",
    };
};

type Shape enum {
    Circle { r: f64 },
    Empty,
};

type Writer interface {
    Write(p: u8[]) int!;
};

type Buf struct {
    data: u8[],

    fn Write(p: u8[]) int! {
        data = append(data, p...);
        return len(p);
    };
};

type Stack[Item] struct {
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
    };
    return out;
};

fn area(s: Shape) f64 {
    return match (s) {
        case Circle { r } => 3.14159 * r * r;
        case Empty => 0.0;
    };
};

fn pair() i32, bool {
    return 1, true;
};

fn mayFail(ok: bool) i32! {
    if (!ok) {
        return errors.New("nope");
    };
    return 7;
};

fn split(n: i32) (i32, bool)! {
    if (n < 0) {
        return 0, false, errors.New("negative");
    };
    return n, true;
};

fn pass(n: i32) (i32, bool)! {
    return split(n)
        or fail;
};

fn propagate(ok: bool) i32! {
    var v = mayFail(ok)
        or fail("load");
    return v + 1;
};

fn inlineHandler() i32 {
    return mayFail(false)
        on err {
            _ = fmt.Println("inline:", err);
            return -1;
        };
};

fn wrapHandler() i32! {
    var v = mayFail(false)
        on err {
            fail(err, "step");
        };
    return v;
};

// Scope listener: catches unhandled errors of `go` in this function.
fn scopeListener() i32! {
    on err {
        _ = fmt.Println("scope:", err);
    };
    var total = i32(0);
    go {
        total = mayFail(false)
            or fail;
    };
    return total;
};

fn deferPrint() void! {
    fmt.Println("defer: block exit")
        or fail;
};

fn deferDemo() void! {
    if (true) {
        defer deferPrint();
        fmt.Println("defer: inside block")
            or fail;
    };
};

fn detached(n: i32, wg: *sync.WaitGroup) void! {
    defer wg.Done();
    fmt.Println("detached:", n)
        or fail;
};

fn describe(v: any) str {
    return match (v) {
        case i32 as n => "i32";
        case str as s => "str";
        case *User as p => "user";
        else => "other";
    };
};

fn sign(n: i32) str {
    return match (n) {
        case -1 => "neg";
        case 0 => "zero";
        else => "pos";
    };
};

fn maybe(p: *User) str {
    return match (p) {
        case nil => "nil";
        else => "set";
    };
};

test "append grows the slice" {
    var xs = i32[] {};
    xs = append(xs, 1);
    assert_eq(len(xs), 1);
};

test "empty shape has no area" {
    assert_eq(area(Shape.Empty), 0.0);
};

fn main() void! {
    // const, var, alias, conversion, import, package var.
    fmt.Println(Answer, UserId(7), strings.ToUpper("ink"), packageVar)
        or fail;

    // Contract views as JSON; the default wire view hides Pass.
    var u = User { Name: "a", Pass: "secret" };
    var wired = json.Marshal(u)
        or fail;
    var admin = json.Marshal(u.admin())
        or fail;
    fmt.Println(u.Greet(), str(wired), str(admin))
        or fail;

    // Pointers: address, auto-deref, write through, copy.
    var p = User { Name: "b", Pass: "" };
    var pp = &p;
    pp.Name = "c";
    var snapshot = *pp;
    fmt.Println(p.Name, snapshot.Name)
        or fail;

    // Enum, match expression, match statement with block arms.
    var shapes = Shape[] { Shape.Circle { r: 2.0 }, Shape.Empty };
    fmt.Println(area(shapes[0]), area(shapes[1]))
        or fail;
    match (shapes[0]) {
        case Circle { r } => {
            fmt.Println("circle", r)
                or fail;
        };
        case Empty => {
            fmt.Println("empty")
                or fail;
        };
    };

    // Interface and structural implementation.
    var b = Buf {};
    var w: Writer = &b;
    var written = w.Write(bytes("hey"))
        or fail;
    fmt.Println(written, str(b.data))
        or fail;

    // Generics: inference and explicit type arguments.
    var stack = Stack[i32] {};
    stack.Push(1);
    stack.Push(2);
    fmt.Println(stack.Pop(), stack.Pop())
        or fail;
    fmt.Println(Map[i32, i32](i32[] { 1, 2, 3 }, fn(x: i32) i32 => x * 2))
        or fail;

    // Multiple results and fallible results.
    var a, ok = pair();
    var x, y = pass(7)
        or fail;
    var v = propagate(true)
        or fail;
    fmt.Println(a, ok, x, y, v)
        or fail;

    // Inline handler leaves zero; another handler wraps and fails.
    fmt.Println(inlineHandler())
        or fail;
    var wrapped = wrapHandler()
        on err {
            fmt.Println("handled:", err)
                or fail;
        };
    fmt.Println("wrapped:", wrapped)
        or fail;

    // Scope listener on go errors.
    var listened = scopeListener()
        or fail;
    fmt.Println("listened:", listened)
        or fail;

    // Deliberate discard of a fallible call.
    _ = mayFail(false);

    // Block-scoped defer.
    deferDemo()
        or fail;

    // if / else if / else, break, continue, infinite loop.
    var total = i32(0);
    for (var i = 0; i < 10; i += 1) {
        if (i == 3) {
            continue;
        };
        if (i == 8) {
            break;
        };
        total += i32(i);
    };
    var loops = 0;
    for () {
        loops += 1;
        if (loops == 3) {
            break;
        };
    };
    fmt.Println(total, loops)
        or fail;

    // Range over slice, map, string.
    var xs = i32[] { 10, 20, 30 };
    for (var i, e in xs) {
        total += i32(i) + e;
    };
    var m = map[str, i32] { "a": 1, "bb": 2 };
    for (var k, mv in m) {
        total += mv + i32(len(k));
    };
    for (var r in "ab") {
        total += i32(r);
    };
    fmt.Println(total)
        or fail;

    // Slices, arrays sized by a const and by an import, builtins.
    var arr = i32[lib.N] {};
    var slice = xs[1:3];
    var buf = u8[8] {};
    var copied = copy(buf[:], bytes("hi"));
    var present = has(m, "a");
    delete(m, "a");
    fmt.Println(len(arr), len(slice), copied, cap(xs), present, min(3, 5), max(3, 5))
        or fail;
    fmt.Println(lib.Double(21))
        or fail;

    // String bytes vs runes.
    var raw = "hé";
    fmt.Println(raw[1], str(runes(raw)), len(raw))
        or fail;

    // Typed vars give bare nil a type; nil[T] when there is none.
    var np: *User = nil;
    var ns: i32[] = nil;
    var nm: map[str, i32] = nil;
    var nf: Handler = nil;
    fmt.Println(np == nil, ns == nil, nm == nil, nf == nil, maybe(nil[*User]))
        or fail;

    // Function values and aliases.
    var h = fn(n: i32) i32 => n + 1;
    var apply = fn(f: Handler, n: i32) i32 => f(n);
    fmt.Println(apply(h, 41))
        or fail;

    // Patterns: type, literal, nil.
    fmt.Println(describe(i32(1)), describe("s"), describe(&p), describe(true))
        or fail;
    fmt.Println(sign(-1), sign(0), sign(1), maybe(&p))
        or fail;

    // Go interop: callback, sort, explicit generics.
    var nums = i32[] { 3, 1, 2 };
    slices.SortFunc(nums, fn(a: i32, b: i32) int => int(a - b));
    fmt.Println(nums)
        or fail;

    // go: one worker per top-level statement, joins at the block.
    var left = i32(0);
    var right = i32(0);
    go {
        left = mayFail(true)
            or fail;
        right = mayFail(true)
            or fail;
    };
    // go for over disjoint slots with an explicit limit.
    var out = i32[4] {};
    go for (var i, n in i32[] { 1, 2, 3, 4 }) limit 2 {
        out[i] = n * 2;
    };
    fmt.Println(left, right, out)
        or fail;

    // Detached task: program-owned, its own error handler, explicit join.
    var wg = sync.WaitGroup {};
    wg.Add(1);
    go detached(3, &wg)
        on err {
            _ = fmt.Println("task:", err);
        };
    wg.Wait();

    // Arena: scratch memory reused, nothing escapes the block.
    var arenaSum = i32(0);
    arena {
        var ap = new[User]();
        ap.Name = "arena";
        var made = i32[](2);
        made[0] = 4;
        made[1] = 5;
        arenaSum = i32(len(ap.Name)) + made[0] + made[1];
    };
    fmt.Println(arenaSum)
        or fail;

    // Nested block scopes its own defer and locals.
    {
        var scoped = mayFail(true)
            or fail;
        fmt.Println("scope", scoped)
            or fail;
    };

    return;
};
