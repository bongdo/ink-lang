use github.com/gen2brain/raylib-go/raylib as rl;
use math;
use runtime;

const W = 480;

const H = 270;

const RowLen = W * 4;

type Row = u8[RowLen];

fn shadeRow(y: i32, t: f64) Row {
    var row = Row {};
    for (var x = i32(0); x < i32(W); x += 1) {
        var fx = f64(x) / f64(W);
        var fy = f64(y) / f64(H);
        var v = math.Sin((fx + t) * 6.2831853) + math.Cos((fy - t) * 6.2831853);
        var b = u8(i32((v * 0.5 + 1.0) * 127.5));
        var i = x * 4;
        row[i] = b;
        row[i + 1] = b;
        row[i + 2] = u8(255) - b;
        row[i + 3] = 255;
    };
    return row;
};

fn main() void {
    rl.InitWindow(W, H, "ink / arena frame");
    defer rl.CloseWindow();
    rl.SetTargetFPS(60);

    var blank = rl.GenImageColor(W, H, rl.Black);
    var tex = rl.LoadTextureFromImage(blank);
    rl.UnloadImage(blank);
    defer rl.UnloadTexture(tex);

    var px = make[u8](RowLen * H);
    var t0 = rl.GetTime();
    var workers = i32(runtime.GOMAXPROCS(0));

    for (var frame = 0; rl.WindowShouldClose() == false; frame += 1) {
        var t = (rl.GetTime() - t0) * 0.2;
        arena {
            var rows = make[Row](H);
            go for (var y = i32(0); y < i32(H); y += 1) limit workers {
                rows[y] = shadeRow(y, t);
            };
            for (var y = 0; y < H; y += 1) {
                copy(px[y * RowLen:], rows[y][:]);
            };
        };
        rl.UpdateTexture(tex, px);
        rl.BeginDrawing();
        rl.ClearBackground(rl.Black);
        rl.DrawTexture(tex, 0, 0, rl.White);
        rl.DrawFPS(8, 8);
        rl.EndDrawing();
    };
};
