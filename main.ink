struct Counter { n: i32, fn Inc() void { n += 1; }; };
var store = Counter { n: 0 };
fn main() void {
    serve store;
    store = Counter { n: 0 };
    par {
        for (var i = 0; i < 10; i += 1) { store.Inc(); };;
        for (var i = 0; i < 10; i += 1) { store.Inc(); };;
    };
};
