use os;
use ./lib;

fn seed() lib.Store {
    var s = lib.NewStore();
    s.Add("A", 10, 2.0);
    s.Add("B", 10, 10.0);
    s.Add("C", 5, 4.0);
    s.Add("D", 1, 1.0);
    return s;
};

fn mustReceive(s: *lib.Store, sku: str, qty: i32) i32 {
    return s.Receive(sku, qty)
        on err { panic(err); };
};

test "receive decrements stock" {
    var s = seed();
    var id = mustReceive(&s, "A", 3);
    assert_eq(id, 1);
    assert_eq(s.Items["A"].Qty, 7);
};

test "cannot oversell" {
    var s = seed();
    var failed = false;
    s.Receive("A", 20)
        on err { failed = true; };
    assert(failed);
    assert_eq(s.Items["A"].Qty, 10);
};

test "discount applies once over ten units" {
    var s = seed();
    var id = mustReceive(&s, "B", 10);
    var v = s.Value(id)
        on err { panic(err); };
    assert_eq(v, 90.0);
};

test "value of small order has no discount" {
    var s = seed();
    var id = mustReceive(&s, "C", 2);
    var v = s.Value(id)
        on err { panic(err); };
    assert_eq(v, 8.0);
};

test "top returns exactly n items, largest first" {
    var s = seed();
    var top = s.Top(2);
    assert_eq(len(top), 2);
    assert_eq(top[0].Qty, 10);
    assert_eq(top[1].Qty, 10);
};

test "load rejects corrupt json" {
    _ = os.WriteFile("corrupt.json", bytes("{not json"), 0o644);
    defer os.Remove("corrupt.json");
    var failed = false;
    var s = lib.Load("corrupt.json")
        on err { failed = true; };
    _ = s;
    assert(failed);
};

test "load keeps the order counter" {
    var path = "store.json";
    defer os.Remove(path);
    var s = seed();
    var first = mustReceive(&s, "A", 1);
    lib.Save(path, &s)
        on err { panic(err); };
    var loaded = lib.Load(path)
        on err { panic(err); };
    var second = mustReceive(&loaded, "A", 1);
    assert_eq(first, 1);
    assert_eq(second, 2);
};
