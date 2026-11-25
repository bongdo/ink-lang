use fmt;

interface Writer {
    Write(p: u8[]) int!;
};

struct Buffer {
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
    var n = w.Write(bytes(text))?;
    return n;
};

fn main() void! {
    var b = Buffer {};
    var n = emit(&b, "hello")?;
    fmt.Println(n, b.Len(), str(b.data))?;
};
