package parser

import (
	"testing"

	"github.com/bongdo/ink/internal/ast"
)

func TestContractBlocks(t *testing.T) {
	f := mustParse(t, `struct User {
    wire: str,
    ID: i64
    fn Label() str {
        return wire;
    };
    wire {
        ID: "id";
        wire: "wire"
    }
    admin {
        ID: "id",
    }
};

fn main() void {
    var u = User {};
    _ = u.admin();
};
`)
	checkSpans(t, f)
	td, ok := f.Decls[0].(*ast.TypeDecl)
	if !ok {
		t.Fatalf("first decl is %T", f.Decls[0])
	}
	if len(td.Contracts) != 2 {
		t.Fatalf("got %d contracts, want 2", len(td.Contracts))
	}
	if td.Contracts[0].Name != "wire" || td.Contracts[1].Name != "admin" {
		t.Fatalf("contract names: %q, %q", td.Contracts[0].Name, td.Contracts[1].Name)
	}
	if got := td.Contracts[1].Fields[0]; got.Field != "ID" || got.External != "id" {
		t.Fatalf("admin field: %+v", got)
	}
}

func TestContractOnlyInStruct(t *testing.T) {
	_, errs := Parse("bad.ink", []byte(`enum Shape {
    Circle,
    wire {
        A: "a",
    },
};

interface W {
    wire {
        A: "a",
    };
};
`))
	if len(errs) != 2 {
		t.Fatalf("want 2 errors, got %d: %v", len(errs), errs)
	}
}
