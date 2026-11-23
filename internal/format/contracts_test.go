package format

import (
	"testing"

	"github.com/bongdo/ink/internal/parser"
)

func TestContractRoundTrip(t *testing.T) {
	src := `type User struct {
    ID: i64,
    Email: str,

    fn Label() str {
        return Email;
    };

    wire {
        ID: "id",
        Email: "email",
    };

    admin {
        ID: "id",
        Email: "email",
    };
};

fn main() void {
    var u = User {};
    _ = u.admin();
};
`
	f, errs := parser.Parse("contracts.ink", []byte(src))
	if len(errs) > 0 {
		t.Fatalf("parse: %v", errs)
	}
	out := File(f)
	f2, errs := parser.Parse("contracts.ink", out)
	if len(errs) > 0 {
		t.Fatalf("parse formatted: %v\n---\n%s", errs, out)
	}
	out2 := File(f2)
	if string(out) != string(out2) {
		t.Fatalf("not idempotent:\n--- first ---\n%s\n--- second ---\n%s", out, out2)
	}
}
