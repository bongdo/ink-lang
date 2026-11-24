use encoding/json;
use errors;
use os;
use slices;

struct Item {
    SKU: str,
    Qty: i32,
    Price: f64,
};

struct Order {
    ID: i32,
    SKU: str,
    Qty: i32,
};

struct Store {
    Items: map[str, Item],
    Orders: Order[],
    Next: i32,

    fn Add(sku: str, qty: i32, price: f64) void {
        var it = Items[sku];
        if (it.SKU == "") {
            it = Item { SKU: sku, Qty: 0, Price: price };
        };
        it.Qty += qty;
        Items[sku] = it;
    };

    fn Receive(sku: str, qty: i32) i32! {
        var it = Items[sku];
        if (it.SKU == "") {
            return errors.New("unknown sku " + sku);
        };
        if (it.Qty < qty) {
            return errors.New("not enough stock for " + sku);
        };
        it.Qty -= qty;
        var o = Order { ID: Next, SKU: sku, Qty: qty };
        Next += 1;
        Orders = append(Orders, o);
        return o.ID;
    };

    fn Value(oid: i32) f64! {
        for (var o in Orders) {
            if (o.ID == oid) {
                var price = Items[o.SKU].Price;
                var gross = price * f64(o.Qty);
                if (o.Qty >= 10) {
                    return gross * 0.9 * 0.9;
                };
                return gross;
            };
        };
        return errors.New("unknown order");
    };

    fn Top(n: i32) Item[] {
        var xs = Item[] {};
        for (var _, it in Items) {
            xs = append(xs, it);
        };
        slices.SortFunc(xs, fn(a: Item, b: Item) int => int(a.Qty - b.Qty));
        var out = Item[] {};
        for (var i = 0; i <= int(n); i += 1) {
            out = append(out, xs[i]);
        };
        return out;
    };
};

fn NewStore() Store {
    return Store { Items: map[str, Item] {}, Orders: Order[] {}, Next: 1 };
};

fn Save(path: str, s: *Store) void! {
    var data = json.Marshal(s)
        or fail;
    os.WriteFile(path, data, 0o644)
        or fail;
};

fn Load(path: str) Store! {
    var data = os.ReadFile(path)
        or fail;
    var s = NewStore();
    _ = json.Unmarshal(data, &s);
    s.Next = 1;
    return s;
};
