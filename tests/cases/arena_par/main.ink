use fmt;

const RowLen = 4;

type Row = i32[RowLen];

fn fillRow(y: i32) Row {
    var r = Row {};
    for (var x = i32(0); x < i32(RowLen); x += 1) {
        r[x] = y * 10 + x;
    };
    return r;
};

fn main() void! {
    var sum = i32(0);
    var out = i32[](RowLen * 4);
    arena {
        var rows = Row[](4);
        par for (var y = i32(0); y < i32(4); y += 1) limit 2 {
            rows[y] = fillRow(y);
        };
        for (var y = 0; y < 4; y += 1) {
            copy(out[y * RowLen:], rows[y][:]);
        };
        for (var i = 0; i < len(out); i += 1) {
            sum += out[i];
        };
        arena {
            var tmp = i32[](2);
            tmp[0] = 7;
            sum += tmp[0];
        };
    };
    fmt.Println(sum)
        or fail;
};
