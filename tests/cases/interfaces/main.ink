use fmt;

type Writer interface {
    Write(p: u8[]) int!;
};

type Buffer struct {
    data: u8[],

    fn Write(p: u8[]) int! {
        data = append(data, p...);
        return len(p);
    };

    fn Len() int {
        return len(data);
    };
};

fn emit(w: Writer, text: str) int! {
    var n = w.Write(bytes(text))
        or fail;
    return n;
};

fn main() void! {
    var b = Buffer {};
    var n = emit(&b, "hello")
        or fail;
    fmt.Println(n, b.Len(), str(b.data))
        or fail;
};
