struct Counter {
    n: i32,

    fn Add(d: i32) void {
        n += d;
    };

    fn Value() i32 {
        return n;
    };
};

fn addTwice(c: *Counter, d: i32) void {
    c.Add(d);
    c.Add(d);
};

fn main() void! {
    var c = Counter { n: 0 };
    serve c;

    c.Add(1);
    addTwice(&c, 2);

    par {
        {
            c.Add(10);
        };
        {
            c.Add(20);
        };
    };

    print(c.Value());
};
