package uml

import (
	"testing"

	"github.com/bongdo/ink-lang/internal/ast"
	"github.com/bongdo/ink-lang/internal/parser"
)

func parse(t *testing.T, path, src string) *ast.File {
	t.Helper()
	f, errs := parser.Parse(path, []byte(src))
	for _, err := range errs {
		t.Fatalf("%s: %d:%d", err.Msg, err.Pos.Line, err.Pos.Col)
	}
	return f
}

func TestPlantUML(t *testing.T) {
	parsed := map[string][]*ast.File{
		".": {parse(t, "main.ink", `use ./lib;

alias Id = u64;

struct Holder {
    rect: lib.Rect,
    id: Id,

    fn Get() lib.Rect {
        return rect;
    };
};

enum Shape {
    Circle { r: f64 },
    Pair { a: Holder, b: lib.Rect },
    Empty,
};

interface Writer {
    Write(p: u8[]) int!;
};

fn area(s: Shape) f64 {
    return 0.0;
};
`)},
		"lib": {parse(t, "lib/lib.ink", `struct Rect {
    w: f64,

    fn Area() f64 {
        return w;
    };
};

fn Double(n: i32) i32 {
    return n * 2;
};
`)},
	}

	want := `@startuml
package "app" as p0 {
  class "Id = u64" as c0 <<alias>>
  class "Holder" as c1 {
      +rect: lib.Rect
      +id: Id
      +Get(): lib.Rect
  }
  enum "Shape" as c2 {
      Circle(r: f64)
      Pair(a: Holder, b: lib.Rect)
      Empty
  }
  interface "Writer" as c3 {
      +Write(p: u8[]): int!
  }
  class "main.ink" as c5 <<file>> {
      +area(s: Shape): f64
  }
}
package "lib" as p1 {
  class "Rect" as c4 {
      +w: f64
      +Area(): f64
  }
  class "lib.ink" as c6 <<file>> {
      +Double(n: i32): i32
  }
}
p0 ..> p1
c1 --> c4
c1 --> c0
c2 --> c1
c2 --> c4
@enduml
`
	if got := PlantUML(parsed, "app"); got != want {
		t.Fatalf("unexpected output:\n%s", got)
	}
}
