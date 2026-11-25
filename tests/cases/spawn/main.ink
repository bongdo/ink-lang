use fmt;
use sync;
use time;

fn save(id: i32, wg: *sync.WaitGroup) void {
    defer wg.Done();
    _ = fmt.Println("saved", id);
};

fn failTask(wg: *sync.WaitGroup) void! {
    defer wg.Done();
    check(false, "boom");
};

fn main() void {
    var wg = sync.WaitGroup {};
    wg.Add(1);
    spawn save(7, &wg);
    wg.Wait();
    wg.Add(1);
    spawn failTask(&wg)
        on err {
            _ = fmt.Println("task:", err);
        };
    wg.Wait();
    time.Sleep(20ms);
};
