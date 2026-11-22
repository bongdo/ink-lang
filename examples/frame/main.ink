use github.com/gen2brain/raylib-go/raylib as rl;
use math;
use runtime;

const W = 480;

const H = 270;

type Frame struct {
    px: u8[],
    t: f64,
};

fn shade(y: i32, x: i32, t: f64) u8 {
    var fx = f64(x) / f64(W);
    var fy = f64(y) / f64(H);
    var v = math.Sin((fx + t) * 6.2831853) + math.Cos((fy - t) * 6.2831853);
    return u8(i32((v * 0.5 + 1.0) * 127.5));
};

fn fillRow(f: *Frame, y: i32) void {
    var w = i32(W);
    var base = y * w * 4;
    for (var x = i32(0); x < w; x += 1) {
        var v = shade(y, x, f.t);
        var i = base + x * 4;
        f.px[i] = v;
        f.px[i + 1] = v;
        f.px[i + 2] = u8(255) - v;
        f.px[i + 3] = 255;
    };
};

fn main() void {
    rl.InitWindow(W, H, "ink / arena frame");
    defer rl.CloseWindow();
    rl.SetTargetFPS(60);

    var blank = rl.GenImageColor(W, H, rl.Black);
    var tex = rl.LoadTextureFromImage(blank);
    rl.UnloadImage(blank);
    defer rl.UnloadTexture(tex);

    var t0 = rl.GetTime();
    for (var frame = 0; rl.WindowShouldClose() == false; frame += 1) {
        var t = (rl.GetTime() - t0) * 0.2;
        arena {
            var f = new[Frame]();
            f.px = make[u8](W * H * 4);
            f.t = t;
            var workers = i32(runtime.GOMAXPROCS(0));
            var bands = workers * 4;
            var per = (i32(H) + bands - 1) / bands;
            par for (var b = i32(0); b < bands; b += 1) limit workers {
                var y0 = b * per;
                var y1 = min(y0 + per, i32(H));
                for (var y = y0; y < y1; y += 1) {
                    fillRow(f, y);
                };
            };
            rl.UpdateTexture(tex, f.px);
        };
        rl.BeginDrawing();
        rl.ClearBackground(rl.Black);
        rl.DrawTexture(tex, 0, 0, rl.White);
        rl.DrawFPS(8, 8);
        rl.EndDrawing();
    };
};
