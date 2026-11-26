package gen

import "testing"

func TestSprintBuiltin(t *testing.T) {
	p := newProject(t, map[string]string{
		"main.ink": `fn main() void! {
    print(sprint(42));
    print(sprint(i64(-7)));
    print(sprint(3.5));
    print(sprint(true));
    print(sprint("hi"));
    var v: any = 9;
    print(sprint(v));
    print("n=" + sprint(11));
};
`,
	})
	stdout, stderr, err := p.run(t)
	if err != nil {
		t.Fatalf("run: %v\nstderr:\n%s", err, stderr)
	}
	expect(t, stdout, "42\n-7\n3.5\ntrue\nhi\n9\nn=11\n")
}
