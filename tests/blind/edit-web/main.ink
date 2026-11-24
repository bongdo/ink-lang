use fmt;
use encoding/json;
use net/http;
use net/http/httptest;
use sync;
use time;
use os;
use io;
use strconv;
use strings;
use context;
use os/signal;
use syscall;

type Item struct {
    ID: int,
    Title: str,
    Done: bool,

    wire {
        ID: "id",
        Title: "title",
        Done: "done",
    };
};

type CreateReq struct {
    Title: str,

    wire {
        Title: "title",
    };
};

type PatchReq struct {
    Done: bool,

    wire {
        Done: "done",
    };
};

type Store struct {
    mu: sync.Mutex,
    items: Item[],
    next: int,

    fn Add(title: str) Item {
        mu.Lock();
        defer mu.Unlock();
        var it = Item { ID: next, Title: title, Done: false };
        next += 1;
        items = append(items, it);
        return it;
    };

    fn List() Item[] {
        mu.Lock();
        defer mu.Unlock();
        var out = Item[] {};
        for (var it in items) {
            out = append(out, it);
        };
        return out;
    };

    fn Get(id: int) Item, bool {
        mu.Lock();
        defer mu.Unlock();
        for (var it in items) {
            if (it.ID == id) {
                return it, true;
            };
        };
        return Item { ID: 0, Title: "", Done: false }, false;
    };

    fn SetDone(id: int, done: bool) Item, bool {
        mu.Lock();
        defer mu.Unlock();
        for (var i = 0; i < len(items); i += 1) {
            if (items[i].ID == id) {
                items[i].Done = done;
                return items[i], true;
            };
        };
        return Item { ID: 0, Title: "", Done: false }, false;
    };

    fn Delete(id: int) bool {
        mu.Lock();
        defer mu.Unlock();
        var kept = Item[](cap: len(items));
        var found = false;
        for (var it in items) {
            if (it.ID == id) {
                found = true;
            } else {
                kept = append(kept, it);
            };
        };
        items = kept;
        return found;
    };
};

fn newStore() *Store {
    return &Store { mu: sync.Mutex {}, items: Item[] {}, next: 1 };
};

var store = newStore();

fn errJSON(msg: str) str {
    return `{"error":"` + msg + `"}`;
};

fn encode(v: any) str {
    var b = json.Marshal(v)
        on err {
            return errJSON("encode failed");
        };
    return str(b);
};

fn dispatch(r: *http.Request) int, str {
    var path = r.URL.Path;
    var method = r.Method;

    if (path == "/healthz") {
        if (method == "GET") {
            return 200, `{"ok":true}`;
        };
        return 405, errJSON("method not allowed");
    };

    if (path == "/items") {
        if (method == "GET") {
            return 200, encode(store.List());
        };
        if (method == "POST") {
            var data = io.ReadAll(r.Body)
                on err {
                    return 400, errJSON("unreadable body");
                };
            var req = CreateReq { Title: "" };
            json.Unmarshal(data, &req)
                on err {
                    return 400, errJSON("invalid json");
                };
            if (len(strings.TrimSpace(req.Title)) == 0) {
                return 400, errJSON("title required");
            };
            return 201, encode(store.Add(req.Title));
        };
        return 405, errJSON("method not allowed");
    };

    if (len(path) > 7 && path[:7] == "/items/") {
        var id = strconv.Atoi(path[7:])
            on err {
                return 404, errJSON("not found");
            };
        if (method == "GET") {
            var it, ok = store.Get(id);
            if (!ok) {
                return 404, errJSON("not found");
            };
            return 200, encode(it);
        };
        if (method == "PATCH") {
            var data = io.ReadAll(r.Body)
                on err {
                    return 400, errJSON("unreadable body");
                };
            var req = PatchReq { Done: false };
            json.Unmarshal(data, &req)
                on err {
                    return 400, errJSON("invalid json");
                };
            var it, ok = store.SetDone(id, req.Done);
            if (!ok) {
                return 404, errJSON("not found");
            };
            return 200, encode(it);
        };
        if (method == "DELETE") {
            if (!store.Delete(id)) {
                return 404, errJSON("not found");
            };
            return 204, "";
        };
        return 405, errJSON("method not allowed");
    };

    return 404, errJSON("not found");
};

fn handle(w: http.ResponseWriter, r: *http.Request) void {
    var start = time.Now();
    var status, body = dispatch(r);
    w.Header().Set("Content-Type", "application/json");
    w.WriteHeader(status);
    if (len(body) > 0) {
        _ = w.Write(bytes(body));
    };
    _ = fmt.Printf("method=%s path=%s status=%d duration=%s\n", r.Method, r.URL.Path, status, time.Since(start));
};

fn addr() str {
    var port = os.Getenv("PORT");
    if (len(port) == 0) {
        port = "8080";
    };
    return ":" + port;
};

fn main() void! {
    var srv = &http.Server { Addr: addr(), Handler: http.HandlerFunc(handle) };
    var ctx, stop = signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM);
    defer stop();
    var shutdownCtx, cancel = context.WithTimeout(context.Background(), 5 * time.Second);
    defer cancel();
    var drained = sync.WaitGroup {};
    drained.Add(1);
    context.AfterFunc(ctx, fn() {
        srv.Shutdown(shutdownCtx)
            on err {
                _ = fmt.Fprintln(os.Stderr, "shutdown:", err);
            };
        _ = fmt.Fprintln(os.Stderr, "shutdown complete");
        drained.Done();
    });
    _ = fmt.Printf("starting on %s\n", addr());
    srv.ListenAndServe()
        on err {
            if (err != http.ErrServerClosed) {
                fail(err, "listen");
            };
        };
    drained.Wait();
};

fn call(method: str, path: str, body: str) *httptest.ResponseRecorder {
    var rec = httptest.NewRecorder();
    var req = httptest.NewRequest(method, path, strings.NewReader(body));
    handle(rec, req);
    return rec;
};

fn decodeItem(rec: *httptest.ResponseRecorder) Item! {
    var it = Item { ID: 0, Title: "", Done: false };
    json.Unmarshal(rec.Body.Bytes(), &it)
        or fail;
    return it;
};

test "healthz" {
    store = newStore();
    var rec = call("GET", "/healthz", "");
    assert_eq(rec.Code, 200);
    assert_eq(rec.Body.String(), `{"ok":true}`);
};

test "create list get patch delete" {
    store = newStore();
    var created = call("POST", "/items", `{"title":"first"}`);
    assert_eq(created.Code, 201);
    var it = decodeItem(created)
        or fail;
    assert_eq(it.ID, 1);
    assert_eq(it.Title, "first");
    assert_eq(it.Done, false);

    var rec = call("GET", "/items", "");
    assert_eq(rec.Code, 200);
    assert_eq(rec.Body.String(), `[{"id":1,"title":"first","done":false}]`);

    var got = call("GET", "/items/1", "");
    assert_eq(got.Code, 200);
    var gotItem = decodeItem(got)
        or fail;
    assert_eq(gotItem.Title, "first");

    var patched = call("PATCH", "/items/1", `{"done":true}`);
    assert_eq(patched.Code, 200);
    var patchedItem = decodeItem(patched)
        or fail;
    assert_eq(patchedItem.Done, true);

    var deleted = call("DELETE", "/items/1", "");
    assert_eq(deleted.Code, 204);
    assert_eq(call("GET", "/items/1", "").Code, 404);
};

test "empty list is an array" {
    store = newStore();
    var rec = call("GET", "/items", "");
    assert_eq(rec.Code, 200);
    assert_eq(rec.Body.String(), "[]");
};

test "missing item is 404" {
    store = newStore();
    assert_eq(call("GET", "/items/42", "").Code, 404);
    assert_eq(call("PATCH", "/items/42", `{"done":true}`).Code, 404);
    assert_eq(call("DELETE", "/items/42", "").Code, 404);
};

test "bad json is 400" {
    store = newStore();
    assert_eq(call("POST", "/items", "{oops").Code, 400);
    assert_eq(call("PATCH", "/items/1", "{oops").Code, 400);
};

test "empty title is 400" {
    store = newStore();
    assert_eq(call("POST", "/items", `{"title":""}`).Code, 400);
    assert_eq(call("POST", "/items", `{"title":"   "}`).Code, 400);
};

test "wrong method is 405 and unknown path is 404" {
    store = newStore();
    assert_eq(call("DELETE", "/items", "").Code, 405);
    assert_eq(call("POST", "/healthz", "").Code, 405);
    assert_eq(call("TRACE", "/items/1", "").Code, 405);
    assert_eq(call("GET", "/nope", "").Code, 404);
};

test "concurrent handlers share the store" {
    store = newStore();
    par for (var i = 0; i < 50; i += 1) limit 8 {
        var rec = call("POST", "/items", `{"title":"x"}`);
        assert_eq(rec.Code, 201);
    };
    assert_eq(len(store.List()), 50);
    assert_eq(len(store.List()), 50);
};

test "port env overrides" {
    var saved = os.Getenv("PORT");
    os.Setenv("PORT", "9999")
        or fail;
    assert_eq(addr(), ":9999");
    os.Setenv("PORT", "")
        or fail;
    assert_eq(addr(), ":8080");
    os.Setenv("PORT", saved)
        or fail;
};
