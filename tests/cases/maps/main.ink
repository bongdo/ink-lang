use fmt;

struct P {
    x: i32,
};

struct Bag {
    m: map[str, i32],
};

fn bump(m: map[str, i32]) void {
    m["a"] += 1;
};

fn main() void! {
    var a = map[str, i32] { "a": 1 };
    var b = a;
    b["a"] = 9;
    b["b"] = 2;
    fmt.Println("alias", a["a"], len(a))?;

    bump(a);
    fmt.Println("param", a["a"])?;

    var missing = a["zz"];
    var v, ok = a["a"];
    var _, absent = a["zz"];
    fmt.Println("lookup", missing, v, ok, absent)?;

    fmt.Println("has", has(a, "b"), has(a, "zz"))?;
    delete(a, "b");
    fmt.Println("delete", has(a, "b"), len(a))?;

    var keys = str[] {};
    var sum = i32(0);
    for (var k, v in a) {
        keys = append(keys, k);
        sum += v;
    };;
    fmt.Println("iter", len(keys), sum)?;

    var ps = map[str, P] { "p": P { x: 1 } };
    var p = ps["p"];
    p.x = 5;
    fmt.Println("struct copy", ps["p"].x, p.x)?;
    ps["p"] = p;
    fmt.Println("store", ps["p"].x)?;

    var bag = Bag { m: a };
    var bag2 = bag;
    bag2.m["a"] = 100;
    fmt.Println("field alias", bag.m["a"])?;

    var nilmap: map[str, i32] = nil;
    fmt.Println("nil read", len(nilmap), nilmap["x"], has(nilmap, "x"))?;
    delete(nilmap, "x");
    fmt.Println("nil delete", len(nilmap))?;
};
