use fmt;
use errors;

fn stop() void! {
    return errors.New("stop");
};

fn work() void! {
    par {
        {
            defer fmt.Println("cleanup");
            stop()
                or fail;
            fmt.Println("unreachable")
                or fail;
        };
    };
    fmt.Println("unreachable")
        or fail;
};

fn main() void! {
    par {
        {
            defer fmt.Println("d1");
            defer fmt.Println("d2");
            fmt.Println("body")
                or fail;
        };
    };
    var caught = 0;
    work()
        or err {
            caught = 1;
        };
    fmt.Println("caught", caught)
        or fail;
};
