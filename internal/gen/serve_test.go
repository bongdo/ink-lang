package gen

import (
	"strings"
	"testing"
)

func TestServeSerializesMethodsAndFreeFunctions(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `struct Counter {
    n: i32,

    fn Add(d: i32) void {
        n += d;
    };

    fn Value() i32 {
        return n;
    };
};

fn addTwice(c: *Counter, d: i32) void {
    c.Add(d);
    c.Add(d);
};

fn through(c: *Counter) void {
    addTwice(c, 5);
};

fn main() void! {
    var c = Counter { n: 0 };
    serve c;
    c.Add(1);
    addTwice(&c, 2);
    through(&c);
    par {
        {
            c.Add(10);
        };
        {
            c.Add(20);
        };
    };
    print(c.Value());
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, "_s_mu")
	contains(t, src, "_s_served")
	contains(t, src, "_s_raw_Add")
	contains(t, src, "_s_held_addTwice")
	contains(t, src, "_s_held_through")
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "45\n")
}

func TestServeFreeFunctionHeldInsideMethod(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `struct Box {
    n: i32,

    fn Add(d: i32) void {
        n += d;
    };

    fn AddTwice(d: i32) void {
        bump(self, d);
        bump(self, d);
    };

    fn Value() i32 {
        return n;
    };
};

fn bump(b: *Box, d: i32) void {
    b.Add(d);
};

fn main() void! {
    var b = Box { n: 0 };
    serve b;
    b.AddTwice(3);
    print(b.Value());
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "6\n")
}

func TestServeGeneratedRawSelf(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `struct User {
    Name: str,

    fn Greet() str {
        return "hi, " + Name;
    };

    fn Loud() str {
        return self.Greet() + "!";
    };
};

fn main() void! {
    var u = User { Name: "Alice" };
    serve u;
    print(u.Loud());
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, "_s_raw_Greet")
	if strings.Contains(src, "self.Greet()") {
		t.Fatalf("raw method called the locking wrapper:\n%s", src)
	}
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "hi, Alice!\n")
}
