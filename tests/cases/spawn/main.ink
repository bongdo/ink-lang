use time;

fn save(id: i32) void {
    print("saved", id);
};

fn failTask() void! {
    check(false, "boom");
};

fn main() void {
    par {
        save(7);
    };
    spawn failTask()
        on err {
            print("task:", err);
        };
    time.Sleep(50ms);
};
