package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bongdo/ink/internal/ast"
	"github.com/bongdo/ink/internal/sema"
)

func decodeLocations(t *testing.T, result json.RawMessage) []Location {
	t.Helper()
	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		t.Fatal(err)
	}
	return locations
}

func decodeEdit(t *testing.T, result json.RawMessage) WorkspaceEdit {
	t.Helper()
	var edit WorkspaceEdit
	if err := json.Unmarshal(result, &edit); err != nil {
		t.Fatal(err)
	}
	return edit
}

func applyWorkspaceEdit(src string, edits []TextEdit) string {
	data := []byte(src)
	for i := len(edits) - 1; i >= 0; i-- {
		start := OffsetAt(data, edits[i].Range.Start)
		end := OffsetAt(data, edits[i].Range.End)
		data = append(append(append([]byte{}, data[:start]...), []byte(edits[i].NewText)...), data[end:]...)
	}
	return string(data)
}

func openDocument(t *testing.T, c *testClient, uri, src string) *PublishDiagnosticsParams {
	t.Helper()
	c.notify("textDocument/didOpen", DidOpenParams{
		TextDocument: TextDocumentItem{URI: uri, LanguageID: "ink", Version: 1, Text: src},
	})
	return c.waitDiagnostics(uri, func(params *PublishDiagnosticsParams) bool {
		return params.Version != nil && *params.Version == 1
	})
}

func findDiagnostic(diags []Diagnostic, code string) *Diagnostic {
	for i := range diags {
		if diags[i].Code == code {
			return &diags[i]
		}
	}
	return nil
}

func TestReferences(t *testing.T) {
	root, _, uri := newProject(t, validSrc)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	openDocument(t, c, uri, validSrc)
	src := []byte(validSrc)
	position := PositionAt(src, strings.Index(validSrc, "total + 1")+1)
	msg := c.request("textDocument/references", ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		Context: ReferenceContext{IncludeDeclaration: true},
	})
	locations := decodeLocations(t, msg.Result)
	if len(locations) != 3 {
		t.Fatalf("local references = %d: %+v", len(locations), locations)
	}
	for _, location := range locations {
		if location.URI != uri {
			t.Fatalf("reference URI = %q", location.URI)
		}
	}
	msg = c.request("textDocument/references", ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
	})
	if locations = decodeLocations(t, msg.Result); len(locations) != 2 {
		t.Fatalf("local references without declaration = %d: %+v", len(locations), locations)
	}
	position = PositionAt(src, strings.Index(validSrc, "add(1, 2)")+1)
	msg = c.request("textDocument/references", ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		Context: ReferenceContext{IncludeDeclaration: true},
	})
	locations = decodeLocations(t, msg.Result)
	if len(locations) != 2 {
		t.Fatalf("function references = %d: %+v", len(locations), locations)
	}
	decl := PositionAt(src, strings.Index(validSrc, "fn add")+len("fn "))
	foundDecl := false
	for _, location := range locations {
		if location.Range.Start == decl {
			foundDecl = true
		}
	}
	if !foundDecl {
		t.Fatalf("function declaration missing from references: %+v", locations)
	}
}

func TestRename(t *testing.T) {
	root, _, uri := newProject(t, validSrc)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	openDocument(t, c, uri, validSrc)
	src := []byte(validSrc)
	position := PositionAt(src, strings.Index(validSrc, "total + 1")+1)
	msg := c.request("textDocument/prepareRename", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     position,
	})
	var prepared PrepareRenameResult
	if err := json.Unmarshal(msg.Result, &prepared); err != nil {
		t.Fatal(err)
	}
	if prepared.Placeholder != "total" {
		t.Fatalf("prepare rename = %+v", prepared)
	}
	msg = c.request("textDocument/rename", RenameParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		NewName: "sum",
	})
	edit := decodeEdit(t, msg.Result)
	edits := edit.Changes[uri]
	if len(edits) != 3 {
		t.Fatalf("rename edits = %d: %+v", len(edits), edits)
	}
	renamed := applyWorkspaceEdit(validSrc, edits)
	if strings.Contains(renamed, "total") || !strings.Contains(renamed, "var sum = 1;") {
		t.Fatalf("renamed source:\n%s", renamed)
	}
}

func TestRenameDeclKinds(t *testing.T) {
	src := `struct User { Name: str };
enum Shape { Empty };
interface Writer { Write(p: u8[]) int!; };
alias ID = u64;

fn main() void {
    var u = User { Name: "x" };
    var s = Shape.Empty;
    var id: ID = 1;
    _ = u;
    _ = s;
    _ = id;
};
`
	cases := []struct {
		name string
		decl string
		want []string
	}{
		{"User", "struct User", []string{"struct Sum", "Sum { Name"}},
		{"Shape", "enum Shape", []string{"enum Sum", "Sum.Empty"}},
		{"Writer", "interface Writer", []string{"interface Sum"}},
		{"ID", "alias ID", []string{"alias Sum", "var id: Sum"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, _, uri := newProject(t, src)
			c := startClient(t, root)
			c.request("initialize", InitializeParams{})
			openDocument(t, c, uri, src)
			position := PositionAt([]byte(src), strings.Index(src, tc.decl)+len(tc.decl)-len(tc.name))
			msg := c.request("textDocument/rename", RenameParams{
				TextDocumentPositionParams: TextDocumentPositionParams{
					TextDocument: TextDocumentIdentifier{URI: uri},
					Position:     position,
				},
				NewName: "Sum",
			})
			edit := decodeEdit(t, msg.Result)
			renamed := applyWorkspaceEdit(src, edit.Changes[uri])
			for _, want := range tc.want {
				if !strings.Contains(renamed, want) {
					t.Fatalf("renamed source missing %q:\n%s", want, renamed)
				}
			}
		})
	}
}

func TestDocumentSymbolsDeclKinds(t *testing.T) {
	src := `struct User { Name: str };
enum Shape { Empty };
alias ID = u64;
`
	root, _, uri := newProject(t, src)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	openDocument(t, c, uri, src)
	msg := c.request("textDocument/documentSymbol", DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	})
	var docs []DocumentSymbol
	if err := json.Unmarshal(msg.Result, &docs); err != nil {
		t.Fatal(err)
	}
	kinds := make(map[string]int)
	for _, symbol := range docs {
		kinds[symbol.Name] = symbol.Kind
	}
	if kinds["User"] != dskStruct || kinds["Shape"] != dskEnum {
		t.Fatalf("document symbols: %+v", docs)
	}
}

func TestRenameMultiName(t *testing.T) {
	src := `fn pair() i32, bool {
    return 1, true;
};

fn main() void {
    var a, b = pair();
    var c = a;
    _ = c;
    _ = b;
};
`
	root, _, uri := newProject(t, src)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	diags := openDocument(t, c, uri, src)
	if len(diags.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags.Diagnostics)
	}
	data := []byte(src)
	position := PositionAt(data, strings.Index(src, "var a, b")+len("var "))
	msg := c.request("textDocument/rename", RenameParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		NewName: "sum",
	})
	edit := decodeEdit(t, msg.Result)
	edits := edit.Changes[uri]
	if len(edits) != 2 {
		t.Fatalf("multi-name rename edits = %d: %+v", len(edits), edits)
	}
	renamed := applyWorkspaceEdit(src, edits)
	if !strings.Contains(renamed, "var sum, b = pair();") || !strings.Contains(renamed, "var c = sum;") {
		t.Fatalf("renamed source:\n%s", renamed)
	}
	if !strings.Contains(renamed, "_ = b;") {
		t.Fatalf("sibling renamed:\n%s", renamed)
	}
}

func TestRenameGoSymbolRejected(t *testing.T) {
	src := `use fmt;

fn main() void {
    fmt.Println("hi");
};
`
	root, _, uri := newProject(t, src)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	openDocument(t, c, uri, src)
	data := []byte(src)
	position := PositionAt(data, strings.Index(src, "Println")+1)
	msg := c.request("textDocument/prepareRename", TextDocumentPositionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     position,
	})
	if string(msg.Result) != "null" {
		t.Fatalf("prepare rename on Go symbol = %s", msg.Result)
	}
	msg = c.request("textDocument/rename", RenameParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		NewName: "Print",
	})
	if string(msg.Result) != "null" {
		t.Fatalf("rename on Go symbol = %s", msg.Result)
	}
}

func TestRenameInvalidIdentifier(t *testing.T) {
	root, _, uri := newProject(t, validSrc)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	openDocument(t, c, uri, validSrc)
	data := []byte(validSrc)
	position := PositionAt(data, strings.Index(validSrc, "total + 1")+1)
	msg := c.requestRaw("textDocument/rename", RenameParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		NewName: "for",
	})
	if msg.Error == nil {
		t.Fatalf("invalid rename accepted: %s", msg.Result)
	}
}

func TestCodeActions(t *testing.T) {
	tests := []struct {
		name        string
		src         string
		code        string
		want        string
		notWant     string
		extraAction bool
	}{
		{
			name: "const-local",
			src: `fn main() void {
    const N = 3;
    var a = N;
    _ = a;
};
`,
			code:    "const-local",
			want:    "var N = 3;",
			notWant: "const N",
		},
		{
			name: "decl-equals-fn",
			src: `fn main() void = {
};
`,
			code:    "decl-equals",
			want:    "fn main() void {",
			notWant: " = {",
		},
		{
			name: "iface-fn",
			src: `interface Writer {
    fn Write(p: u8[]) int!;
};

fn main() void {};
`,
			code:    "iface-fn",
			want:    "Write(p: u8[])",
			notWant: "fn Write",
		},
		{
			name: "import-simple",
			src: `use "fmt";

fn main() void {
    fmt.Println("hi");
};
`,
			code:    "import-simple",
			want:    "use fmt;",
			notWant: `use "fmt"`,
		},
		{
			name: "import-alias",
			src: `use example.com/bubbletea/v2;

fn main() void {};
`,
			code: "import-alias",
			want: `use example.com/bubbletea/v2 as bubbletea;`,
		},
		{
			name: "err-manual",
			src: `fn f() i32! {
    return 1;
};

fn main() i32! {
    var v, err = f();
    if (err != nil) {
        return err;
    };
    return v;
};
`,
			code:    "err-manual",
			want:    "var v = f() or fail;",
			notWant: "if (err != nil)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "import-alias" {
				t.Setenv("GOPROXY", "off")
			}
			root, _, uri := newProject(t, tc.src)
			c := startClient(t, root)
			c.request("initialize", InitializeParams{})
			diags := openDocument(t, c, uri, tc.src)
			diagnostic := findDiagnostic(diags.Diagnostics, tc.code)
			if diagnostic == nil {
				t.Fatalf("missing %s diagnostic: %+v", tc.code, diags.Diagnostics)
			}
			msg := c.request("textDocument/codeAction", CodeActionParams{
				TextDocument: TextDocumentIdentifier{URI: uri},
				Context:      CodeActionContext{Diagnostics: []Diagnostic{*diagnostic}},
			})
			var actions []CodeAction
			if err := json.Unmarshal(msg.Result, &actions); err != nil {
				t.Fatal(err)
			}
			if len(actions) == 0 {
				t.Fatalf("no code actions for %+v stderr=%s", *diagnostic, c.stderr.String())
			}
			action := actions[0]
			if action.Edit == nil || len(action.Diagnostics) != 1 || action.Diagnostics[0].Code != tc.code {
				t.Fatalf("action = %+v", action)
			}
			edits := action.Edit.Changes[uri]
			if len(edits) == 0 {
				t.Fatalf("action has no edits: %+v", action)
			}
			fixed := applyWorkspaceEdit(tc.src, edits)
			if !strings.Contains(fixed, tc.want) {
				t.Fatalf("fixed source:\n%s\nwant %q", fixed, tc.want)
			}
			if tc.notWant != "" && strings.Contains(fixed, tc.notWant) {
				t.Fatalf("fixed source still contains %q:\n%s", tc.notWant, fixed)
			}
		})
	}
}

func TestNilUntypedHasNoUnsafeFix(t *testing.T) {
	src := `fn main() void {
    var p = nil;
    _ = p;
};
`
	root, _, uri := newProject(t, src)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	diags := openDocument(t, c, uri, src)
	diagnostic := findDiagnostic(diags.Diagnostics, "nil-untyped")
	if diagnostic == nil {
		t.Fatalf("missing nil diagnostic: %+v", diags.Diagnostics)
	}
	msg := c.request("textDocument/codeAction", CodeActionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Context:      CodeActionContext{Diagnostics: []Diagnostic{*diagnostic}},
	})
	var actions []CodeAction
	if err := json.Unmarshal(msg.Result, &actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Fatalf("unsafe nil fix: %+v", actions)
	}
}

func TestErrManualWithElseHasNoFix(t *testing.T) {
	src := `fn f() i32! {
    return 1;
};

fn main() i32! {
    var v, err = f();
    if (err != nil) {
        return err;
    } else {
        return v;
    };
    return v;
};
`
	root, _, uri := newProject(t, src)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	diags := openDocument(t, c, uri, src)
	diagnostic := findDiagnostic(diags.Diagnostics, "err-manual")
	if diagnostic == nil {
		t.Fatalf("missing err-manual diagnostic: %+v", diags.Diagnostics)
	}
	msg := c.request("textDocument/codeAction", CodeActionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Context:      CodeActionContext{Diagnostics: []Diagnostic{*diagnostic}},
	})
	var actions []CodeAction
	if err := json.Unmarshal(msg.Result, &actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Fatalf("unsafe err-manual fix: %+v", actions)
	}
}

func TestTestBlockTypeDiagnostic(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/hello\n\ngo 1.27\n",
	}
	src := `fn main() void {};

test "bad" {
    var n = 1 + "x";
    _ = n;
};
`
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "main.ink")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	uri := pathToURI(path)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	diags := openDocument(t, c, uri, src)
	for _, diag := range diags.Diagnostics {
		if strings.Contains(diag.Message, "mismatched types") {
			if diag.Range.Start.Line != 3 {
				t.Fatalf("test diagnostic line = %+v", diag.Range)
			}
			if diag.Code != "go-type" {
				t.Fatalf("test diagnostic code = %q, want go-type", diag.Code)
			}
			return
		}
	}
	t.Fatalf("missing test block type diagnostic: %+v", diags.Diagnostics)
}

func TestDidChangeDebounceAndStaleness(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/hello\n\ngo 1.27\n",
	}
	valid := "fn main() void {\n    var n = 1;\n    _ = n;\n};\n"
	bad := "fn main() void {\n    var n = 1 + \"x\";\n    _ = n;\n};\n"
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(root, "main.ink")
	if err := os.WriteFile(path, []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	uri := pathToURI(path)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	openDocument(t, c, uri, valid)
	c.notify("textDocument/didChange", DidChangeParams{
		TextDocument:   VersionedTextDocumentIdentifier{URI: uri, Version: 2},
		ContentChanges: []TextDocumentChange{{Text: bad}},
	})
	diags := c.waitDiagnostics(uri, func(params *PublishDiagnosticsParams) bool {
		if params.Version == nil || *params.Version != 2 {
			return false
		}
		for _, diag := range params.Diagnostics {
			if strings.Contains(diag.Message, "mismatched types") {
				return true
			}
		}
		return false
	})
	if len(diags.Diagnostics) == 0 {
		t.Fatal("debounced typecheck produced no diagnostics")
	}
	c.notify("textDocument/didChange", DidChangeParams{
		TextDocument:   VersionedTextDocumentIdentifier{URI: uri, Version: 4},
		ContentChanges: []TextDocumentChange{{Text: bad}},
	})
	c.notify("textDocument/didChange", DidChangeParams{
		TextDocument:   VersionedTextDocumentIdentifier{URI: uri, Version: 5},
		ContentChanges: []TextDocumentChange{{Text: valid}},
	})
	c.waitDiagnostics(uri, func(params *PublishDiagnosticsParams) bool {
		return params.Version != nil && *params.Version == 5
	})
	time.Sleep(350 * time.Millisecond)
	for _, msg := range c.notes {
		if msg.Method != "textDocument/publishDiagnostics" {
			continue
		}
		var params PublishDiagnosticsParams
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			continue
		}
		if params.Version == nil || *params.Version != 4 {
			continue
		}
		for _, diag := range params.Diagnostics {
			if strings.Contains(diag.Message, "mismatched types") {
				t.Fatalf("stale typecheck published: %+v", params)
			}
		}
	}
}

func TestDidCloseClearsBufferDiagnostics(t *testing.T) {
	valid := "fn main() void {\n    var total = 1;\n    _ = total;\n};\n"
	buffer := "fn main() void {\n    var total = bufferUnknown;\n    _ = total;\n};\n"
	disk := "fn main() void {\n    var total = diskUnknown;\n    _ = total;\n};\n"
	root, path, uri := newProject(t, valid)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	diags := openDocument(t, c, uri, buffer)
	if findDiagnostic(diags.Diagnostics, "undefined") == nil {
		t.Fatalf("missing buffer diagnostic: %+v", diags.Diagnostics)
	}
	if err := os.WriteFile(path, []byte(disk), 0o644); err != nil {
		t.Fatal(err)
	}
	c.notify("textDocument/didClose", DidCloseParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	c.waitDiagnostics(uri, func(params *PublishDiagnosticsParams) bool {
		return len(params.Diagnostics) == 0
	})
	diags = c.waitDiagnostics(uri, func(params *PublishDiagnosticsParams) bool {
		for _, diag := range params.Diagnostics {
			if strings.Contains(diag.Message, "diskUnknown") {
				return true
			}
		}
		return false
	})
	for _, diag := range diags.Diagnostics {
		if strings.Contains(diag.Message, "bufferUnknown") {
			t.Fatalf("stale buffer diagnostic after close: %+v", diags.Diagnostics)
		}
	}
}

func TestLocalImportReferences(t *testing.T) {
	mainSrc := `use ./lib as lib;

fn main() void {
    var t: lib.Thing = lib.Make();
    _ = t;
};
`
	libSrc := `struct Thing {
    Value: i32,
};

fn Make() Thing {
    return Thing { Value = 1 };
};
`
	root, _, mainURI := newProject(t, mainSrc)
	libDir := filepath.Join(root, "lib")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}
	libPath := filepath.Join(libDir, "lib.ink")
	if err := os.WriteFile(libPath, []byte(libSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	libURI := pathToURI(libPath)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	diags := openDocument(t, c, mainURI, mainSrc)
	if len(diags.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags.Diagnostics)
	}
	data := []byte(mainSrc)
	position := PositionAt(data, strings.Index(mainSrc, "Thing")+1)
	msg := c.request("textDocument/references", ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: mainURI},
			Position:     position,
		},
		Context: ReferenceContext{IncludeDeclaration: true},
	})
	locations := decodeLocations(t, msg.Result)
	uris := make(map[string]bool)
	for _, location := range locations {
		uris[location.URI] = true
	}
	if !uris[mainURI] || !uris[libURI] || len(locations) < 3 {
		t.Fatalf("local import references = %+v", locations)
	}
	msg = c.request("textDocument/rename", RenameParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: mainURI},
			Position:     position,
		},
		NewName: "Item",
	})
	edit := decodeEdit(t, msg.Result)
	if len(edit.Changes[mainURI]) == 0 || len(edit.Changes[libURI]) == 0 {
		t.Fatalf("local import rename changes = %+v", edit.Changes)
	}
}

func TestFieldReferences(t *testing.T) {
	src := `struct User {
    Name: str,

    fn Hello() str {
        return Name;
    };
};

fn main() void {
    var u = User { Name = "x" };
    var s = u.Name;
    _ = s;
    _ = u.Hello();
};
`
	root, _, uri := newProject(t, src)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	diags := openDocument(t, c, uri, src)
	if len(diags.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", diags.Diagnostics)
	}
	data := []byte(src)
	position := PositionAt(data, strings.Index(src, "Name: str")+1)
	msg := c.request("textDocument/references", ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		Context: ReferenceContext{IncludeDeclaration: true},
	})
	if locations := decodeLocations(t, msg.Result); len(locations) != 4 {
		t.Fatalf("field references = %d: %+v", len(locations), locations)
	}
	position = PositionAt(data, strings.Index(src, "Name = \"x\"")+1)
	msg = c.request("textDocument/rename", RenameParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		NewName: "Title",
	})
	edit := decodeEdit(t, msg.Result)
	if len(edit.Changes[uri]) != 4 {
		t.Fatalf("field rename edits = %+v", edit.Changes[uri])
	}
	renamed := applyWorkspaceEdit(src, edit.Changes[uri])
	if strings.Contains(renamed, "Name") || !strings.Contains(renamed, "Title: str") {
		t.Fatalf("field rename:\n%s", renamed)
	}
}

func TestWorkspaceSymbols(t *testing.T) {
	src := `const Answer = 42;
var Count = 1;

struct User {
    Name: str,

    fn hello() str {
        return "hi";
    };
};

fn main() void {
    _ = Count;
    _ = Answer;
};
`
	root, _, uri := newProject(t, src)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	openDocument(t, c, uri, src)
	msg := c.request("workspace/symbol", WorkspaceSymbolParams{})
	var symbols []SymbolInformation
	if err := json.Unmarshal(msg.Result, &symbols); err != nil {
		t.Fatal(err)
	}
	names := make(map[string]int)
	for _, symbol := range symbols {
		names[symbol.Name] = symbol.Kind
		if symbol.Location.URI != uri {
			t.Fatalf("symbol URI = %q", symbol.Location.URI)
		}
	}
	for want, kind := range map[string]int{
		"Answer": dskConstant,
		"Count":  dskVariable,
		"User":   dskStruct,
		"hello":  dskMethod,
		"main":   dskFunction,
	} {
		if names[want] != kind {
			t.Fatalf("workspace symbol %q kind = %d, want %d: %+v", want, names[want], kind, names)
		}
	}
}

func TestRenameConflict(t *testing.T) {
	a := &ast.FuncDecl{Name: "a"}
	b := &ast.FuncDecl{Name: "b"}
	fi := &fileInfo{rel: ".", prog: &sema.Program{Packages: map[string]*sema.Package{
		".": {Funcs: map[string]*ast.FuncDecl{"a": a, "b": b}},
	}}}
	s := &server{}
	if !s.renameConflict(fi, &renameTarget{decl: b, name: "b"}, "a") {
		t.Fatal("collision not detected")
	}
	if s.renameConflict(fi, &renameTarget{decl: b, name: "b"}, "c") {
		t.Fatal("false collision")
	}
}

func TestRenameFieldInContract(t *testing.T) {
	src := `struct User {
    ID: i64,
    Email: str,

    wire {
        ID: "id",
        Email: "email",
    };

    admin {
        ID: "id",
    };
};

fn main() void {
    var u = User { ID: 1, Email: "e" };
    _ = u.admin();
};
`
	root, _, uri := newProject(t, src)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	openDocument(t, c, uri, src)
	position := PositionAt([]byte(src), strings.Index(src, "ID: i64")+1)
	msg := c.request("textDocument/rename", RenameParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		NewName: "Key",
	})
	edit := decodeEdit(t, msg.Result)
	renamed := applyWorkspaceEdit(src, edit.Changes[uri])
	if strings.Contains(renamed, "ID: \"id\"") {
		t.Fatalf("contract entry was not renamed:\n%s", renamed)
	}
	if !strings.Contains(renamed, "Key: \"id\"") {
		t.Fatalf("expected renamed contract entry:\n%s", renamed)
	}
}

func TestRenameShadowingPackageRejected(t *testing.T) {
	src := `use fmt;

fn main() void! {
    var total = 1;
    fmt.Println(total)
        or fail;
    return;
};
`
	root, _, uri := newProject(t, src)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	openDocument(t, c, uri, src)
	position := PositionAt([]byte(src), strings.Index(src, "total = 1"))
	msg := c.requestRaw("textDocument/rename", RenameParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		NewName: "fmt",
	})
	if msg.Error == nil {
		t.Fatalf("rename that shadows a package was accepted: %s", msg.Result)
	}
}

func TestRenameSilentShadowRejected(t *testing.T) {
	src := `fn main() void {
    var total = 1;
    {
        var sum = 2;
        _ = total + sum;
    };
    _ = total;
};
`
	root, _, uri := newProject(t, src)
	c := startClient(t, root)
	c.request("initialize", InitializeParams{})
	openDocument(t, c, uri, src)
	position := PositionAt([]byte(src), strings.Index(src, "total = 1"))
	msg := c.requestRaw("textDocument/rename", RenameParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		NewName: "sum",
	})
	if msg.Error == nil {
		t.Fatalf("shadowing rename accepted: %s", msg.Result)
	}
}
