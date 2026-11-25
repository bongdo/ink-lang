// Ink 0.2: every construct in one file. `ink check`, `ink test`, `ink run`.
use strings;
use slices;
use errors;
use encoding/json;
use sync;
use ./lib;

const Answer = 42;

var packageVar = i32(7);

alias UserId = u64;

alias Handler = fn(i32) i32;

// Struct: fields, methods with implicit self, JSON contract blocks.
struct User {
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

enum Shape {
    Circle { r: f64 },
    Empty,
};

interface Writer {
    Write(p: u8[]) int!;
};

struct Buf {
    data: u8[],

    fn Write(p: u8[]) int! {
        data = append(data, p...);
        return len(p);
    };
};

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
    return split(n)?;
};

fn propagate(ok: bool) i32! {
    var v = mayFail(ok)?;
    return v + 1;
};

fn inlineHandler() i32 {
    return mayFail(false)
        on err {
            print("inline:", err);
            return -1;
        };
};

fn wrapHandler() i32! {
    var v = mayFail(false)?;
    return v;
};

// Scope listener: catches unhandled errors of `par` in this function.
fn scopeListener() i32! {
    on err {
        print("scope:", err);
    };
    var total = i32(0);
    par {
        total = mayFail(false)?;
    };
    return total;
};

fn deferPrint() void! {
    print("defer: block exit");
};

fn deferDemo() void! {
    if (true) {
        defer deferPrint();
        print("defer: inside block");
    };
};

fn detached(n: i32, wg: *sync.WaitGroup) void! {
    defer wg.Done();
    print("detached:", n);
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
    print(Answer, UserId(7), strings.ToUpper("ink"), packageVar);

    // Contract views as JSON; the default wire view hides Pass.
    var u = User { Name: "a", Pass: "secret" };
    var wired = json.Marshal(u)?;
    var admin = json.Marshal(u.admin())?;
    print(u.Greet(), str(wired), str(admin));

    // Pointers: address, auto-deref, write through, copy.
    var p = User { Name: "b", Pass: "" };
    var pp = &p;
    pp.Name = "c";
    var snapshot = *pp;
    print(p.Name, snapshot.Name);

    // Enum, match expression, match statement with block arms.
    var shapes = Shape[] { Shape.Circle { r: 2.0 }, Shape.Empty };
    print(area(shapes[0]), area(shapes[1]));
    match (shapes[0]) {
        case Circle { r } => {
            print("circle", r);
        };
        case Empty => {
            print("empty");
        };
    };

    // Interface and structural implementation.
    var b = Buf {};
    var w: Writer = &b;
    var written = w.Write(bytes("hey"))?;
    print(written, str(b.data));

    // Generics: inference and explicit type arguments.
    var stack = Stack[i32] {};
    stack.Push(1);
    stack.Push(2);
    print(stack.Pop(), stack.Pop());
    print(Map[i32, i32](i32[] { 1, 2, 3 }, fn(x: i32) i32 => x * 2));

    // Multiple results and fallible results.
    var a, ok = pair();
    var x, y = pass(7)?;
    var v = propagate(true)?;
    print(a, ok, x, y, v);

    // Inline handler leaves zero; another handler wraps and fails.
    print(inlineHandler());
    var wrapped = wrapHandler()
        on err {
            print("handled:", err);
        };
    print("wrapped:", wrapped);

    // Scope listener on par errors.
    var listened = scopeListener()?;
    print("listened:", listened);

    // Deliberate discard of a fallible call.
    _ = mayFail(false);

    // Block-scoped defer.
    deferDemo()?;

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
    };;
    var loops = 0;
    for () {
        loops += 1;
        if (loops == 3) {
            break;
        };
    };;
    print(total, loops);

    // Range over slice, map, string.
    var xs = i32[] { 10, 20, 30 };
    for (var i, e in xs) {
        total += i32(i) + e;
    };;
    var m = map[str, i32] { "a": 1, "bb": 2 };
    for (var k, mv in m) {
        total += mv + i32(len(k));
    };;
    for (var r in "ab") {
        total += i32(r);
    };;
    print(total);

    // Slices, arrays sized by a const and by an import, builtins.
    var arr = i32[lib.N] {};
    var slice = xs[1:3];
    var buf = u8[8] {};
    var copied = copy(buf[:], bytes("hi"));
    var present = has(m, "a");
    delete(m, "a");
    print(len(arr), len(slice), copied, cap(xs), present, min(3, 5), max(3, 5));
    print(lib.Double(21));

    // String bytes vs runes.
    var raw = "hé";
    print(raw[1], str(runes(raw)), len(raw));

    // Typed vars give bare nil a type; nil[T] when there is none.
    var np: *User = nil;
    var ns: i32[] = nil;
    var nm: map[str, i32] = nil;
    var nf: Handler = nil;
    print(np == nil, ns == nil, nm == nil, nf == nil, maybe(nil[*User]));

    // Function values and aliases.
    var h = fn(n: i32) i32 => n + 1;
    var apply = fn(f: Handler, n: i32) i32 => f(n);
    print(apply(h, 41));

    // Patterns: type, literal, nil.
    print(describe(i32(1)), describe("s"), describe(&p), describe(true));
    print(sign(-1), sign(0), sign(1), maybe(&p));

    // Go interop: callback, sort, explicit generics.
    var nums = i32[] { 3, 1, 2 };
    slices.SortFunc(nums, fn(a: i32, b: i32) int => int(a - b));
    print(nums);

    // par: one worker per top-level statement, joins at the block.
    var left = i32(0);
    var right = i32(0);
    par {
        left = mayFail(true)?;
        right = mayFail(true)?;
    };
    // par for over disjoint slots with an explicit limit.
    var out = i32[4] {};
    par for (var i, n in i32[] { 1, 2, 3, 4 }) limit 2 {
        out[i] = n * 2;
    };;
    print(left, right, out);

    // Detached task: program-owned, its own error handler, explicit join.
    var wg = sync.WaitGroup {};
    wg.Add(1);
    spawn detached(3, &wg)
        on err {
            print("task:", err);
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
    print(arenaSum);

    // Nested block scopes its own defer and locals.
    {
        var scoped = mayFail(true)?;
        print("scope", scoped);
    };

    return;
};
