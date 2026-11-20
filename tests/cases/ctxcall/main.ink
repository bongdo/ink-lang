use fmt;
use os/exec;

fn main() void! {
    var cmd = exec.CommandContext("true");
    var err = cmd.Run();
    if (err == nil[error]) {
        fmt.Println("ok")
            or fail;
    } else {
        fmt.Println("failed")
            or fail;
    };
};
