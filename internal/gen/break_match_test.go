package gen

import (
	"strings"
	"testing"
)

func TestBreakContinueInsideMatch(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;

fn main() void {
    var sum = 0;
    for (var i = 0; i < 10; i += 1) {
        match (i) {
        case 3 => {
            break;
        };
        case 5 => {
            continue;
        };
        else => {
            sum += i;
        };
        };
    };
    fmt.Println(sum);

    var seen = 0;
    for (var i = 0; i < 6; i += 1) {
        match (i) {
        case 2 => {
            continue;
        };
        case 4 => {
            break;
        };
        else => {
            seen += 1;
        };
        };
    };
    fmt.Println(seen);

    var xs = i32[] { 1, 2, 3, 4 };
    var first = i32(0);
    for (var x in xs) {
        match (x) {
        case 3 => {
            break;
        };
        else => {
            first += x;
        };
        };
    };
    fmt.Println(first);

    var total = 0;
    for (var i = 0; i < 3; i += 1) {
        for (var j = 0; j < 4; j += 1) {
            match (j) {
            case 1 => {
                continue;
            };
            case 3 => {
                break;
            };
            else => {
                total += i * 10 + j;
            };
            };
        };
        match (i) {
        case 2 => {
            break;
        };
        else => {
            total += 100;
        };
        };
    };
    fmt.Println(total);
};
`,
	})
	src := p.source(t, "zz_ink.go")
	contains(t, src, "break _s_loop")
	contains(t, src, "continue _s_loop")
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "3\n3\n3\n266\n")
}

func TestPlainBreakStaysUnlabeled(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `use fmt;

fn main() void {
    var n = 0;
    for (var i = 0; i < 10; i += 1) {
        if (i == 3) {
            break;
        };
        n += i;
    };
    fmt.Println(n);
};
`,
	})
	src := p.source(t, "zz_ink.go")
	if strings.Contains(src, "_s_loop") {
		t.Fatalf("plain break should not use a loop label:\n%s", src)
	}
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "3\n")
}
