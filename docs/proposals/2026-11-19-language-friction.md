# Ink 0.2 — language friction, as one program

One runnable file: `ink check` is clean and `ink run` prints the output.
Blocks marked `// rejected:` are commented out on purpose; uncomment one and
`ink check --json` shows the rule. Each section ends with a verdict.

The point is not the demo. It is that a single realistic file needs this many
exceptions: Go `int` vs `i32`, statement-level `par`, one-root-one-writer,
advisory `within`, `defer` as a worker, `break` leaving a `match`, JSON field
names, no constraints, byte indexing, and four Go-boundary walls.

```ink
// Ink 0.2 — one program to feel the rules.
// `ink run` executes everything below. `// rejected:` blocks are commented
// out on purpose: uncomment one and `ink check --json` shows the rule.
use fmt;
use errors;
use strconv;
use strings;
use time;
use "encoding/json";

// ---------------------------------------------------------------------------
// 1. Numbers are Go's numbers.
// `i32` is int32, but `len` returns Go `int`. There is no implicit
// conversion, so the friendly literal `var total = 0` is an `int`.
// VERDICT: change — every length needs a cast; `int` is Go's, not Ink's.
// ---------------------------------------------------------------------------

type Report struct {
    Name: str,
    Sizes: i32[],
    Total: i32,
};

fn demoNumbers() void {
    var n: i32 = 7;
    var l = len("abc");
    fmt.Println("i32 and int:", n, l) or panic();
    // rejected: var bad: i32 = l;        // int is not i32
    // rejected: var bad2 = n + l;        // mismatched types
    var ok = n + i32(l);
    fmt.Println("converted:", ok) or panic();
};

// ---------------------------------------------------------------------------
// 2. `par`: the worker is the top-level statement, not the block.
// A nested block is sequential inside one worker. A local declared in one
// worker does not exist in another.
// VERDICT: keep the worker model; the sharp edge is `defer` (section 5).
// ---------------------------------------------------------------------------

fn fetch(url: str) i32 or error {
    if (strings.Contains(url, "bad")) {
        return errors.New("bad url: " + url);
    };
    time.Sleep(10ms);
    return i32(len(url));
};

fn demoParScopes() void or error {
    var left = i32(0);
    var right = i32(0);
    par {
        left = fetch("a") or return;
        right = fetch("bb") or return;
    };
    fmt.Println("plain par:", left, right) or return;
    return;
};

// rejected: one root, two writers
// par { left = fetch("a") or return; left = fetch("b") or return; };
// rejected: one writes, the other reads
// par { left = fetch("a") or return; fmt.Println(left) or return; };
// rejected: break/continue/return cannot cross the boundary
// par { break; };
// rejected: no unbounded loop inside par
// par { for () { break; }; };

// ---------------------------------------------------------------------------
// 3. `par for`: only disjoint slots, and only with the iteration variable
// in the outermost index. The body is one worker, not one worker per line.
// VERDICT: keep; the gap is maps: no disjoint keys, no range.
// ---------------------------------------------------------------------------

fn collect(urls: str[]) i32[] or error {
    var sizes = i32[] {};
    for (var i = 0; i < len(urls); i += 1) {
        sizes = append(sizes, 0);
    };
    par for (var i = 0; i < len(urls); i += 1) limit 4 {
        sizes[i] = fetch(urls[i]) or return;
    };
    return sizes;
};

fn demoSlots() i32[] or error {
    var sizes = collect(str[] { "a", "bb", "ccc", "dddd" }) or return;
    fmt.Println("par for slots:", sizes) or return;
    // rejected: a captured map, even with a disjoint key
    // var m = map[str, i32] {};
    // par for (var i = 0; i < 3; i += 1) { m[str(i)] = i; };
    // rejected: a map range in par for
    // par for (var k, v in m) { _ = k; _ = v; };
    return sizes;
};

// ---------------------------------------------------------------------------
// 4. `par` write sets are conservative: one root, one writer, no pointer,
// no `self`, no global. Two different fields of one struct are still one root.
// VERDICT: change — field-level disjointness or an explicit split; valid parallel code is rejected today.
// ---------------------------------------------------------------------------

type Counter struct {
    hits: i32,
    miss: i32,
};

fn demoRoots() void {
    var c = Counter { hits = 0, miss = 0 };
    // rejected: two writers on one root, even for different fields
    // par { c.hits = 1; c.miss = 2; };
    par {
        c.hits = 1;
    };
    fmt.Println("one root, one writer:", c.hits) or panic();
    // rejected: a write through a captured pointer
    // var p = &c; par { p.hits = 2; };
    // rejected: a non-pure method on a captured value struct is a write
    // par { c.Bump(); };
};

// ---------------------------------------------------------------------------
// 5. `defer` at the top level of a `par` body is its own worker.
// It runs when that worker ends, not when the siblings finish.
// VERDICT: change — a `defer` that silently becomes its own worker is a trap, not a feature.
// ---------------------------------------------------------------------------

fn note(s: str) void {
    fmt.Println(s) or panic();
};

fn demoDeferInPar() void {
    par {
        defer note("defer: own worker, runs immediately");
        {
            time.Sleep(30ms);
            note("body: still working after the defer ran");
        };
    };
};

// ---------------------------------------------------------------------------
// 6. C-style `par for (i = 0; ...)` reads a per-iteration copy of `i`.
// After the loop, `i` holds what the post left, not the last body value.
// VERDICT: odd — the post value survives and the copy is invisible at the call site.
// ---------------------------------------------------------------------------

fn demoCstyle() void {
    var i = 0;
    par for (i = 0; i < 3; i += 1) limit 1 {
        note("C-style body sees a copy");
    };
    fmt.Println("after the loop i =", i) or panic();
    // rejected: a direct write to the captured i in the body
    // par for (i = 0; i < 3; i += 1) { i = 0; };
};

// ---------------------------------------------------------------------------
// 7. `within` is advisory: it cancels the context, but a callee that ignores
// the context finishes anyway. Same for `par`: the deadline does not kill a
// sleeping goroutine.
// VERDICT: change — the name promises a deadline; only context-aware callees observe it.
// ---------------------------------------------------------------------------

fn demoWithin(sizes: i32[]) void {
    var start = time.Now();
    within 1ms {
        var n = len(sizes);
        time.Sleep(20ms);
        fmt.Println("within 1ms, body finished, n =", n) or panic();
    };
    fmt.Println("elapsed:", time.Since(start)) or panic();
};

// Hidden context is not a value: it cannot be stored, passed, or put in a
// channel (there are no channels). It resets to `context.Background()` at a
// package boundary, an Ink interface call, a function value and a package
// initializer. A `within` deadline does not cross any of them.
// VERDICT: the biggest semantic hole — a deadline that silently stops at a package boundary.
// rejected: use context;
// rejected: var ctx context.Context = ...;

// ---------------------------------------------------------------------------
// 8. `or` is statement-level only. It cannot be nested in an expression, a
// multi-RHS, or a `for` post.
// VERDICT: keep statement-level; the cost is no nesting and no `for` post.
// ---------------------------------------------------------------------------

fn mayFail(ok: bool) i32 or error {
    if (!ok) {
        return errors.New("requested failure");
    };
    return 7;
};

fn demoErrors() i32 {
    var v = mayFail(false) or err {
        fmt.Println("or err:", err) or panic();
        return -1;
    };
    // rejected: var x = mayFail(true) or return + 1;
    // rejected: f(g() or return);
    // rejected: if (err != nil) { return err; }
    return v;
};

// `or panic` inside a worker is an error at the worker's ordinal, not a Go
// panic. Only `par` turns it into a return or a re-panic.

fn demoErrorOrder() void {
    var sizes = collect(str[] { "bad0", "bad1" }) or err {
        fmt.Println("winner is the lowest ordinal, not the first failure:", err) or panic();
        return;
    };
    fmt.Println("never:", len(sizes)) or panic();
};

// ---------------------------------------------------------------------------
// 9. `break` in a `match` arm targets the enclosing loop, not the match.
// VERDICT: change — `break` should target the match or be rejected; today it leaves the loop.
// ---------------------------------------------------------------------------

fn demoBreakInMatch() void {
    var seen = 0;
    for (var i = 0; i < 5; i += 1) {
        match (i) {
            case 2 => {
                break;
            };
            else => {
                seen += 1;
            };
        };
    };
    fmt.Println("break left the loop, seen =", seen) or panic();
};

// ---------------------------------------------------------------------------
// 10. JSON uses Go field names. There are no struct tags, so a public API
// emits `Name`, not `name`, and a lowercase field never reaches JSON.
// VERDICT: change — decide tags or a naming convention before a public API depends on it.
// ---------------------------------------------------------------------------

fn report(name: str, sizes: i32[]) Report {
    var total = i32(0);
    for (var s in sizes) {
        total += s;
    };
    return Report { Name = name, Sizes = sizes, Total = total };
};

fn printJSON(r: Report) void or error {
    var b = json.Marshal(r) or return;
    fmt.Println(str(b)) or return;
    return;
};

// ---------------------------------------------------------------------------
// 11. Enums are sealed: no zero value, no `Match` without every variant,
// and variant names are unique across the package.
// VERDICT: keep, but no zero value means every enum must be constructed before use.
// ---------------------------------------------------------------------------

type Mode enum {
    Fast,
    Safe { checks: i32 },
};

fn describeMode(m: Mode) str {
    return match (m) {
        case Fast => "fast";
        case Safe { checks } => "safe";
    };
};

// rejected: var m: Mode;             // var needs an initializer; an enum has no zero value
// rejected: match (m) { case Fast => "fast"; };
// rejected: another enum with variant Fast;

// ---------------------------------------------------------------------------
// 12. Generics have no constraints: every type parameter behaves like `any`.
// A `Stack[T]` cannot require `T: comparable`; the body decides what works.
// VERDICT: change — without constraints the error moves from the declaration to the use site.
// ---------------------------------------------------------------------------

type Stack[T] struct {
    data: T[],

    fn Push(x: T) void {
        data = append(data, x);
    };

    fn Pop() T {
        var x = data[len(data) - 1];
        data = data[:len(data) - 1];
        return x;
    };
};

// ---------------------------------------------------------------------------
// 13. `s[i]` is a byte; `runes(s)` is the way to get code points.
// VERDICT: keep Go-compatible; this is the first bug in every port from Go.
// ---------------------------------------------------------------------------

fn demoBytes() void {
    var word = "héllo";
    fmt.Println("word[1] is a byte:", word[1]) or panic();
    fmt.Println("runes(word)[1] is a rune:", runes(word)[1]) or panic();
};

// ---------------------------------------------------------------------------
// 14. `nil` needs an expected type; `nil[T]` is the escape hatch.
// `const` is package-level only. `var` always has an initializer.
// VERDICT: odd — `var` always needs an initializer, `nil` needs a type; two rules for one declaration.
// ---------------------------------------------------------------------------

fn maybeNil(r: *Report) str {
    return match (r) {
        case nil => "nil";
        else => "set";
    };
};

fn demoNilConst() void {
    var p: *Report = nil;
    fmt.Println("typed nil:", maybeNil(p)) or panic();
    fmt.Println("explicit nil[T]:", maybeNil(nil[*Report])) or panic();
    // rejected: var q = nil;
    // rejected: const Local = 3;
    // rejected: x := 1;
};

// ---------------------------------------------------------------------------
// 15. The Go boundary: multiple results work, `(T, bool)` stays an ordinary
// result (`or` is for `error` only), channels and `context.Context` do not
// cross, and `//go:embed` is absent.
// VERDICT: four walls — channels, context, tags, embed; decide each instead of leaving them half-open.
// ---------------------------------------------------------------------------

fn demoGoBoundary() void {
    var before, after, found = strings.Cut("key=value", "=");
    fmt.Println("Cut:", before, after, found) or panic();
    var n = strconv.Atoi("42") or panic();
    fmt.Println("Atoi:", n) or panic();
    // rejected: go worker();
    // rejected: var c chan i32;
    // rejected: select {};
    // rejected: //go:embed templates/*
};

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

fn main() void or error {
    demoNumbers();

    demoParScopes() or return;
    var sizes = demoSlots() or return;
    demoRoots();
    demoDeferInPar();
    demoCstyle();
    fmt.Println("errors:", demoErrors()) or return;
    demoErrorOrder();
    demoBreakInMatch();
    demoWithin(sizes);

    var r = report("batch", sizes);
    printJSON(r) or return;
    fmt.Println("enum:", describeMode(Mode.Safe { checks = 3 })) or return;

    var stack = Stack[str] {};
    stack.Push("a");
    stack.Push("b");
    fmt.Println("stack:", stack.Pop(), stack.Pop()) or return;

    fmt.Println("match on any:", describeAny(i32(1)), describeAny("s"), describeAny(true)) or return;
    demoBytes();
    demoNilConst();
    demoGoBoundary();
    return;
};

fn describeAny(v: any) str {
    return match (v) {
        case i32 as n => "i32";
        case str as s => "str";
        else => "other";
    };
};
```
