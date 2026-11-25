use fmt;
use errors;

fn stop() void! {
    return errors.New("boom");
};

fn withDefer() void! {
    defer fmt.Println("defer ran");
    fmt.Println("body ran")?;
};

fn run1() void! {
    par {
        withDefer()?;
    };
};

fn run2() void! {
    par for (var i = 0; i < 1; i += 1) {
        defer fmt.Println("for defer");
        fmt.Println("for body")?;
    };;
};

fn run3() void! {
    par {
        stop()?;
    };
};

fn run4() void! {
    par {
        {
            defer fmt.Println("inner defer");
            stop()?;
        };
    };
};

fn main() void! {
    var c1 = false;
    var c2 = false;
    var c3 = false;
    var c4 = false;
    run1()
        on err {
            c1 = true;
        };
    run2()
        on err {
            c2 = true;
        };
    run3()
        on err {
            c3 = true;
        };
    run4()
        on err {
            c4 = true;
        };
    fmt.Println(c1, c2, c3, c4)?;
};
