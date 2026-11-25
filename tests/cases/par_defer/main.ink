use fmt;
use errors;

fn stop() void! {
    return errors.New("stop");
};

fn work() void! {
    par {
        {
            defer fmt.Println("cleanup");
            stop()?;
            fmt.Println("unreachable")?;
        };
    };
    fmt.Println("unreachable")?;
};

fn main() void! {
    par {
        {
            defer fmt.Println("d1");
            defer fmt.Println("d2");
            fmt.Println("body")?;
        };
    };
    var caught = 0;
    work()
        on err {
            caught = 1;
        };
    fmt.Println("caught", caught)?;
};
