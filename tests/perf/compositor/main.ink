use fmt;
use os;
use sort;
use runtime;
use time;
use github.com/swaywm/go-wlroots/wlroots as wl;
use github.com/swaywm/go-wlroots/xkb;

fn envInt(name: str, def: int) int {
    var s = os.Getenv(name);
    if (s == "") {
        return def;
    };
    var n = 0;
    for (var i = 0; i < len(s); i += 1) {
        var ch = s[i];
        if (ch < 48 || ch > 57) {
            return def;
        };
        n = n * 10 + int(ch) - 48;
    };
    return n;
};

const ModePass = 0;

const ModeMove = 1;

const ModeResize = 2;

type View struct {
    tl: wl.XDGTopLevel,
    xdg: wl.XDGSurface,
    mapped: bool,
};

type Server struct {
    display: wl.Display,
    backend: wl.Backend,
    renderer: wl.Renderer,
    allocator: wl.Allocator,
    scene: wl.Scene,
    sceneLayout: wl.SceneOutputLayout,
    xdgShell: wl.XDGShell,
    outputLayout: wl.OutputLayout,
    seat: wl.Seat,
    cursor: wl.Cursor,
    cursorMgr: wl.XCursorManager,
    views: (*View)[],
    keyboards: int,
    frames: int,
    work: f64[],
    mode: int,
    grabbed: *View,
    grabX: f64,
    grabY: f64,
    grabBox: wl.GeoBox,
    edges: wl.Edges,

    fn find(xdg: wl.XDGSurface) *View {
        for (var v in self.views) {
            if (v.xdg == xdg) {
                return v;
            };
        };
        return nil;
    };

    fn addView(v: *View) void {
        var next: (*View)[] = nil;
        next = append(next, v);
        for (var x in self.views) {
            next = append(next, x);
        };
        self.views = next;
    };

    fn dropView(v: *View) void {
        var keep: (*View)[] = nil;
        for (var x in self.views) {
            if (x != v) {
                keep = append(keep, x);
            };
        };
        self.views = keep;
    };

    fn front() *View {
        if (len(self.views) == 0) {
            return nil;
        };
        return self.views[0];
    };

    fn focus(v: *View, sx: f64, sy: f64) void {
        if (v == nil) {
            return;
        };
        var prev = self.seat.KeyboardState().FocusedSurface();
        var cur = v.xdg.Surface();
        if (prev == cur) {
            return;
        };
        _ = self.deactivate(prev);
        v.tl.Base().SceneTree().Node().RaiseToTop();
        self.dropView(v);
        self.addView(v);
        v.tl.SetActivated(true);
        if (self.keyboards > 0) {
            self.seat.NotifyKeyboardEnter(v.xdg.Surface(), self.seat.Keyboard());
        };
        _ = sx;
        _ = sy;
    };

    fn deactivate(surf: wl.Surface) void! {
        if (surf.Nil()) {
            return;
        };
        var pt = surf.XDGTopLevel()
            or fail;
        pt.SetActivated(false);
    };

    fn resetMode() void {
        self.mode = ModePass;
        self.grabbed = nil;
    };

    fn viewAt(x: f64, y: f64) *View, f64, f64 {
        var node, sx, sy = self.scene.Tree().Node().At(x, y);
        if (node.Nil() || node.Type() != wl.SceneNodeBuffer) {
            return nil, sx, sy;
        };
        var ss = node.SceneBuffer().SceneSurface();
        if (ss.Nil()) {
            return nil, sx, sy;
        };
        var xdg = ss.Surface().XDGSurface();
        if (xdg.Nil()) {
            return nil, sx, sy;
        };
        return self.find(xdg), sx, sy;
    };

    fn handleNewOutput(o: wl.Output) void {
        _ = o.InitRender(self.allocator, self.renderer);
        var st = wl.NewOutputState();
        st.StateInit();
        st.StateSetEnabled(true);
        var modes = o.Modes();
        var w = envInt("INK_OUT_W", 0);
        var h = envInt("INK_OUT_H", 0);
        var hz = envInt("INK_OUT_HZ", 0);
        if (w > 0 && h > 0 && hz > 0) {
            st.SetCustomMode(w, h, i32(hz * 1000));
        } else if (len(modes) > 0) {
            st.SetMode(modes[0]);
        };
        _ = o.CommitState(st);
        st.Finish();
        o.OnFrame(fn(out: wl.Output) {
            _ = self.handleFrame(out);
        });
        o.OnRequestState(fn(out: wl.Output, s2: wl.OutputState) {
            out.CommitState(s2);
        });
        var lo = self.outputLayout.AddOutputAuto(o);
        var so = self.scene.NewOutput(o);
        self.sceneLayout.AddOutput(lo, so);
        _ = o.SetTitle(fmt.Sprintf("ink-tinywl - %s", o.Name()));
    };

    fn handleFrame(o: wl.Output) void! {
        var t0 = time.Now();
        var so = self.scene.SceneOutput(o)
            or fail;
        so.Commit();
        so.SendFrameDone(time.Now());
        var us = f64(time.Since(t0).Nanoseconds()) / 1000.0;
        self.work = append(self.work, us);
        self.frames += 1;
        if (self.frames % 300 == 0) {
            self.dumpWork();
        };
    };

    fn dumpWork() void {
        var n = len(self.work);
        if (n == 0) {
            return;
        };
        var ys = f64[](n);
        copy(ys, self.work);
        sort.Float64s(ys);
        var sum = 0.0;
        for (var x in ys) {
            sum += x;
        };
        var line = fmt.Sprintf("INKSTAT frames=%d views=%d work_us_mean=%.1f work_us_p50=%.1f work_us_p95=%.1f work_us_p99=%.1f work_us_max=%.1f", n, len(self.views), sum / f64(n), ys[n / 2], ys[(n * 95) / 100], ys[(n * 99) / 100], ys[n - 1]);
        _ = fmt.Println(line);
        self.work = nil;
    };

    fn handleNewTopLevel(tl: wl.XDGTopLevel) void {
        var xdg = tl.Base();
        var v = &View { tl: tl, xdg: xdg, mapped: false };
        self.views = append(self.views, v);
        xdg.SetData(self.scene.Tree().NewXDGSurface(xdg));
        xdg.OnMap(fn(s: wl.XDGSurface) {
            self.mapView(s);
        });
        xdg.OnUnmap(fn(s: wl.XDGSurface) {
            self.unmapView(s);
        });
        xdg.OnCommit(fn(s: wl.XDGSurface) {
            if (s.InitialCommit()) {
                s.ScheduleConfigure();
            };
        });
        tl.OnRequestMove(fn(c: wl.SeatClient, serial: u32) {
            self.beginInteractive(v, ModeMove, 0);
        });
        tl.OnRequestResize(fn(c: wl.SeatClient, serial: u32, e: wl.Edges) {
            self.beginInteractive(v, ModeResize, e);
        });
    };

    fn handleNewPopup(p: wl.XDGPopup) void {
        var xdg = p.Base();
        var parent = p.Parent();
        if (parent.Nil()) {
            return;
        };
        xdg.SetData(parent.XDGSurface().SceneTree().NewXDGSurface(xdg));
        xdg.OnCommit(fn(s: wl.XDGSurface) {
            if (s.InitialCommit()) {
                s.ScheduleConfigure();
            };
        });
    };

    fn mapView(xdg: wl.XDGSurface) void {
        var v = self.find(xdg);
        if (v == nil) {
            return;
        };
        v.mapped = true;
        self.focus(v, 0.0, 0.0);
    };

    fn unmapView(xdg: wl.XDGSurface) void {
        var v = self.find(xdg);
        if (v == nil) {
            return;
        };
        if (self.grabbed == v) {
            self.resetMode();
        };
        v.mapped = false;
        self.dropView(v);
    };

    fn handleNewPointer(dev: wl.InputDevice) void {
        self.cursor.AttachInputDevice(dev);
    };

    fn handleNewKeyboard(dev: wl.InputDevice) void {
        var kb = dev.Keyboard();
        var ctx = xkb.NewContext(xkb.KeySymFlagNoFlags);
        var km = ctx.KeyMap();
        kb.SetKeymap(km);
        km.Destroy();
        ctx.Destroy();
        kb.SetRepeatInfo(25, 600);
        self.seat.SetKeyboard(dev);
        kb.OnModifiers(fn(k: wl.Keyboard) {
            self.seat.SetKeyboard(dev);
            self.seat.NotifyKeyboardModifiers(k);
        });
        kb.OnKey(fn(k: wl.Keyboard, t: u32, code: u32, update: bool, state: wl.KeyState) {
            self.handleKey(k, t, code, state);
        });
        self.keyboards += 1;
    };

    fn handleNewInput(dev: wl.InputDevice) void {
        if (dev.Type() == wl.InputDeviceTypePointer) {
            self.handleNewPointer(dev);
        };
        if (dev.Type() == wl.InputDeviceTypeKeyboard) {
            self.handleNewKeyboard(dev);
        };
        var caps = wl.SeatCapabilityPointer;
        if (self.keyboards > 0) {
            caps = caps | wl.SeatCapabilityKeyboard;
        };
        self.seat.SetCapabilities(caps);
    };

    fn handleKey(kb: wl.Keyboard, t: u32, code: u32, state: wl.KeyState) void {
        var syms = kb.XKBState().Syms(xkb.KeyCode(code + 8));
        var handled = false;
        var mods = kb.Modifiers();
        if ((mods & wl.KeyboardModifierAlt != 0) && state == wl.KeyStatePressed) {
            for (var s in syms) {
                if (self.handleKeyBinding(s)) {
                    handled = true;
                };
            };
        };
        if (!handled) {
            self.seat.SetKeyboard(kb.Base());
            self.seat.NotifyKeyboardKey(t, code, state);
        };
    };

    fn handleKeyBinding(sym: xkb.KeySym) bool {
        if (sym == xkb.KeySymEscape) {
            self.display.Terminate();
            return true;
        };
        if (sym == xkb.KeySymF1) {
            if (len(self.views) < 2) {
                return true;
            };
            var next = self.views[1];
            self.focus(next, 0.0, 0.0);
            return true;
        };
        return false;
    };

    fn handleMotion(dev: wl.InputDevice, t: u32, dx: f64, dy: f64) void {
        self.cursor.Move(dev, dx, dy);
        self.processMotion(t);
    };

    fn handleMotionAbsolute(dev: wl.InputDevice, t: u32, x: f64, y: f64) void {
        self.cursor.WarpAbsolute(dev, x, y);
        self.processMotion(t);
    };

    fn processMotion(t: u32) void {
        if (self.mode == ModeMove) {
            self.processMove();
            return;
        };
        if (self.mode == ModeResize) {
            self.processResize();
            return;
        };
        var v, sx, sy = self.viewAt(self.cursor.X(), self.cursor.Y());
        if (v == nil) {
            self.cursor.SetXCursor(self.cursorMgr, "default");
        };
        if (v != nil) {
            var surface = v.xdg.Surface();
            self.seat.NotifyPointerEnter(surface, sx, sy);
            self.seat.NotifyPointerMotion(t, sx, sy);
        } else {
            self.seat.ClearPointerFocus();
        };
    };

    fn processMove() void {
        if (self.grabbed == nil) {
            return;
        };
        var node = self.grabbed.xdg.Surface().XDGSurface().TopLevel().Base().SceneTree().Node();
        node.SetPosition(self.cursor.X() - self.grabX, self.cursor.Y() - self.grabY);
    };

    fn processResize() void {
        if (self.grabbed == nil) {
            return;
        };
        var bx = self.cursor.X();
        var by = self.cursor.Y();
        var l = self.grabBox.X;
        var r = self.grabBox.X + self.grabBox.Width;
        var tp = self.grabBox.Y;
        var b = self.grabBox.Y + self.grabBox.Height;
        if (self.edges & wl.EdgeTop != 0) {
            tp = int(by);
            if (tp >= b) {
                tp = b - 1;
            };
        } else if (self.edges & wl.EdgeBottom != 0) {
            b = int(by);
            if (b <= tp) {
                b = tp + 1;
            };
        };
        if (self.edges & wl.EdgeLeft != 0) {
            l = int(bx);
            if (l >= r) {
                l = r - 1;
            };
        } else if (self.edges & wl.EdgeRight != 0) {
            r = int(bx);
            if (r <= l) {
                r = l + 1;
            };
        };
        self.grabbed.tl.Base().TopLevelSetSize(u32(r - l), u32(b - tp));
    };

    fn handleButton(dev: wl.InputDevice, t: u32, button: u32, state: wl.ButtonState) void {
        self.seat.NotifyPointerButton(t, button, state);
        if (state == wl.ButtonStateReleased) {
            self.resetMode();
            return;
        };
        var v, _, _ = self.viewAt(self.cursor.X(), self.cursor.Y());
        self.focus(v, 0.0, 0.0);
    };

    fn handleAxis(dev: wl.InputDevice, t: u32, source: wl.AxisSource, o: wl.AxisOrientation, delta: f64, dd: i32) void {
        self.seat.NotifyPointerAxis(t, o, delta, dd, source, wl.RelativeDirectionIdentical);
    };

    fn handleFrame2() void {
        self.seat.NotifyPointerFrame();
    };

    fn handleSetCursor(client: wl.SeatClient, surface: wl.Surface, serial: u32, hx: i32, hy: i32) void {
        if (self.seat.PointerState().FocusedClient() == client) {
            self.cursor.SetSurface(surface, hx, hy);
        };
    };

    fn beginInteractive(v: *View, mode: int, e: wl.Edges) void {
        if (v.xdg.Surface() != self.seat.PointerState().FocusedSurface()) {
            return;
        };
        self.grabbed = v;
        self.mode = mode;
        var node = v.tl.Base().SceneTree().Node();
        if (mode == ModeMove) {
            self.grabX = self.cursor.X() - f64(node.X());
            self.grabY = self.cursor.Y() - f64(node.Y());
            return;
        };
        var box = v.tl.Base().Geometry();
        var rr = 0;
        if (e & wl.EdgeRight != 0) {
            rr = box.Width;
        };
        var bb = 0;
        if (e & wl.EdgeBottom != 0) {
            bb = box.Height;
        };
        self.grabX = self.cursor.X() - f64(node.X() + box.X + rr);
        self.grabY = self.cursor.Y() - f64(node.Y() + box.Y + bb);
        self.grabBox = box;
        self.grabBox.X = box.X + node.X();
        self.grabBox.Y = box.Y + node.Y();
        self.edges = e;
    };

    fn start() void! {
        var socket = self.display.AddSocketAuto()
            or fail("add socket");
        self.backend.Start()
            or fail("start backend");
        os.Setenv("WAYLAND_DISPLAY", socket)
            or fail("setenv");
        _ = fmt.Println(fmt.Sprintf("ink-tinywl on WAYLAND_DISPLAY=%s", socket));
    };

    fn run() void {
        self.display.Run();
        self.display.DestroyClients();
        self.scene.Tree().Node().Destroy();
        self.cursorMgr.Destroy();
        self.outputLayout.Destroy();
        self.display.Destroy();
    };
};

fn newServer() *Server! {
    var s = &Server {
        views: nil,
        frames: 0,
        keyboards: 0,
        mode: ModePass,
        grabbed: nil,
    };
    s.display = wl.NewDisplay();
    s.backend = s.display.BackendAutocreate()
        or fail("backend");
    s.renderer = s.backend.RendererAutoCreate()
        or fail("renderer");
    s.renderer.InitDisplay(s.display);
    s.allocator = s.backend.AllocatorAutocreate(s.renderer)
        or fail("allocator");

    s.display.CompositorCreate(5, s.renderer);
    s.display.SubCompositorCreate();
    s.display.DataDeviceManagerCreate();
    s.outputLayout = wl.NewOutputLayout(s.display);

    s.backend.OnNewOutput(fn(o: wl.Output) {
        s.handleNewOutput(o);
    });

    s.scene = wl.NewScene();
    s.sceneLayout = s.scene.AttachOutputLayout(s.outputLayout);

    s.xdgShell = s.display.XDGShellCreate(3);
    s.xdgShell.OnNewTopLevel(fn(tl: wl.XDGTopLevel) {
        s.handleNewTopLevel(tl);
    });
    s.xdgShell.OnNewPopup(fn(p: wl.XDGPopup) {
        s.handleNewPopup(p);
    });

    s.cursor = wl.NewCursor();
    s.cursor.AttachOutputLayout(s.outputLayout);
    s.cursorMgr = wl.NewXCursorManager("", 24);
    s.cursor.OnMotion(fn(d: wl.InputDevice, t: u32, dx: f64, dy: f64) {
        s.handleMotion(d, t, dx, dy);
    });
    s.cursor.OnMotionAbsolute(fn(d: wl.InputDevice, t: u32, x: f64, y: f64) {
        s.handleMotionAbsolute(d, t, x, y);
    });
    s.cursor.OnButton(fn(d: wl.InputDevice, t: u32, b: u32, st: wl.ButtonState) {
        s.handleButton(d, t, b, st);
    });
    s.cursor.OnAxis(fn(d: wl.InputDevice, t: u32, src: wl.AxisSource, o: wl.AxisOrientation, delta: f64, dd: i32) {
        s.handleAxis(d, t, src, o, delta, dd);
    });
    s.cursor.OnFrame(fn() {
        s.handleFrame2();
    });
    s.cursorMgr.Load(1);

    s.backend.OnNewInput(fn(d: wl.InputDevice) {
        s.handleNewInput(d);
    });
    s.seat = s.display.SeatCreate("seat0");
    s.seat.OnSetCursorRequest(fn(c: wl.SeatClient, sf: wl.Surface, ser: u32, hx: i32, hy: i32) {
        s.handleSetCursor(c, sf, ser, hx, hy);
    });
    return s;
};

fn main() void! {
    runtime.LockOSThread();
    var s = newServer()
        or fail("init server");
    s.start()
        or fail("start server");
    s.run();
};
