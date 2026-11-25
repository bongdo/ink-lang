use fmt;
use os;
use strconv;
use ./lib;

fn usage() void! {
    fmt.Println("usage: ink run -- add SKU QTY PRICE | receive SKU QTY | value ID | top N | save PATH | load PATH; store: $STORE or store.json")?;
};

fn storePath() str {
    var p = os.Getenv("STORE");
    if (p == "") {
        return "store.json";
    };
    return p;
};

fn open(path: str) lib.Store {
    var fresh = lib.NewStore();
    var found = os.Stat(path)
        on err { return fresh; };
    _ = found;
    return lib.Load(path)
        on err { return fresh; };
};

fn main() void! {
    var args = os.Args;
    if (len(args) < 2) {
        usage()?;
        return;
    };
    var s = open(storePath());
    match (args[1]) {
        case "add" => {
            var qty = strconv.Atoi(args[3])?;
            var price = strconv.ParseFloat(args[4], 64)?;
            s.Add(args[2], i32(qty), price);
            lib.Save(storePath(), &s)?;
            fmt.Println("ok")?;
        };
        case "receive" => {
            var qty = strconv.Atoi(args[3])?;
            var id = s.Receive(args[2], i32(qty))?;
            lib.Save(storePath(), &s)?;
            fmt.Println("order", id)?;
        };
        case "value" => {
            var id = strconv.Atoi(args[2])?;
            var v = s.Value(i32(id))?;
            fmt.Println(v)?;
        };
        case "top" => {
            var n = strconv.Atoi(args[2])?;
            for (var it in s.Top(i32(n))) {
                fmt.Println(it.SKU, it.Qty)?;
            };;
        };
        case "save" => {
            lib.Save(args[2], &s)?;
        };
        case "load" => {
            var loaded = lib.Load(args[2])?;
            fmt.Println(len(loaded.Items))?;
        };
        else => {
            usage()?;
        };
    };
};
