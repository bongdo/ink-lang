package driver

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bongdo/ink/internal/token"
)

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewProject(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hello")
	if err := New(dir); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "hello")
	if err := p.Build(out, nil); err != nil {
		p.PrintErrors(os.Stderr)
		t.Fatal(err)
	}
	got, err := exec.Command(out).Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello from hello\n" {
		t.Fatalf("got %q", got)
	}
}

func TestCheckTypeErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/tc\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	main := "fn main() void {\n    var n = 1 + 2;\n    assert(n == 3);\n};\n"
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := "fn Square(x: i32) i32 {\n    return x + \"oops\";\n};\n"
	libPath := filepath.Join(dir, "lib", "lib.ink")
	if err := os.WriteFile(libPath, []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Typecheck: true}); err != nil {
		t.Fatal(err)
	}
	if len(p.Errors) != 1 {
		t.Fatalf("errors: %+v", p.Errors)
	}
	got := p.Errors[0]
	if got.Path != libPath || !strings.Contains(got.Msg, "mismatched types") {
		t.Fatalf("error: %+v", got)
	}
	if got.Pos.Line != 2 {
		t.Fatalf("line: %+v", got.Pos)
	}
	for _, name := range []string{"zz_ink.go", "lib/zz_ink.go"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s was written to the source tree", name)
		}
	}
}

func TestTypeRemovedIsOneDiagnostic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/removed\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "type User struct {\n    Name: str,\n};\n\nfn main() void {\n    var u = User { Name: \"x\" };\n    _ = u;\n};\n"
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Typecheck: true}); err != nil {
		t.Fatal(err)
	}
	if len(p.Errors) != 1 {
		t.Fatalf("errors: %+v", p.Errors)
	}
	got := p.Errors[0]
	if got.Code != "type-removed" || !strings.Contains(got.Msg, "declare struct User") || got.Hint == "" {
		t.Fatalf("error: %+v", got)
	}
	if got.Pos.Line != 1 || got.Pos.Col != 1 {
		t.Fatalf("position: %+v", got.Pos)
	}
}

func TestCheckPathWithSpaces(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "space dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/space\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "main.ink")
	src := "fn main() void {\n    var n = 1 + \"x\";\n    _ = n;\n};\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Typecheck: true}); err != nil {
		t.Fatal(err)
	}
	if len(p.Errors) != 1 {
		t.Fatalf("errors: %+v", p.Errors)
	}
	got := p.Errors[0]
	if got.Path != path || !strings.Contains(got.Msg, "mismatched types") {
		t.Fatalf("error: %+v", got)
	}
}

func TestGoTypeSpanSnapsToToken(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/span\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "fn main() void {\n    var word = \"hello\";\n    var n: i32 = word;\n    _ = n;\n};\n"
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Typecheck: true}); err != nil {
		t.Fatal(err)
	}
	diags := p.Diagnostics()
	if len(diags) != 1 {
		t.Fatalf("diags: %+v", diags)
	}
	d := diags[0]
	if d.Line != 3 || d.Col != 18 || d.EndCol != 22 {
		t.Fatalf("span = %d:%d-%d, want 3:18-22", d.Line, d.Col, d.EndCol)
	}
}

func TestCheckRunsVet(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/vet\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "use fmt;\n\nfn main() void! {\n    fmt.Println(str(42))\n?;\n    return;\n};\n"
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Typecheck: true}); err != nil {
		t.Fatal(err)
	}
	diags := p.Diagnostics()
	if len(diags) != 1 {
		t.Fatalf("diags: %+v", diags)
	}
	if d := diags[0]; d.Code != "str-number" || d.Line != 4 {
		t.Fatalf("diag: %+v", d)
	}
}

func TestMissingReturnDiagnostic(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/ret\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "fn f() i32 {\n};\n\nfn main() void {\n    _ = f();\n};\n"
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Fast: true}); err != nil {
		t.Fatal(err)
	}
	if len(p.Errors) != 1 {
		t.Fatalf("errors: %+v", p.Errors)
	}
	got := p.Errors[0]
	if !strings.Contains(got.Msg, "missing return") || got.Code != "missing-return" || got.Hint == "" {
		t.Fatalf("diagnostic: %+v", got)
	}
	if got.Pos.Line != 2 || got.Pos.Col < 1 {
		t.Fatalf("position: %+v", got.Pos)
	}
}

func TestDuplicateDiagnosticsJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/dup\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "fn main() void! {\n    var p: *Missing = nil;\n    return;\n};\n"
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Fast: true}); err != nil {
		t.Fatal(err)
	}
	if len(p.Errors) != 1 {
		t.Fatalf("errors: %+v", p.Errors)
	}
	var buf bytes.Buffer
	if err := p.WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var diags []JSONError
	if err := json.Unmarshal(buf.Bytes(), &diags); err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 || diags[0].Code != "semantic" || !strings.Contains(diags[0].Message, "unknown type Missing") {
		t.Fatalf("diagnostics: %+v", diags)
	}
}

func TestCheckFastGlobalCycle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/cycle\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "var a: i32 = b;\nvar b = a + 1;\n\nfn main() void {\n    _ = a;\n    _ = b;\n};\n"
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Fast: true}); err != nil {
		t.Fatal(err)
	}
	if len(p.Errors) != 1 || !strings.Contains(p.Errors[0].Msg, "initialization cycle: a -> b") {
		t.Fatalf("errors: %+v", p.Errors)
	}
}

func TestCheckFastAndJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/tc\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	main := "fn main() void {\n    var n = 1 + 2;\n    assert(n == 3);\n};\n"
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := "fn Square(x: i32) i32 {\n    return x + \"oops\";\n};\n"
	libPath := filepath.Join(dir, "lib", "lib.ink")
	if err := os.WriteFile(libPath, []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Typecheck: true}); err != nil {
		t.Fatal(err)
	}
	if len(p.Errors) != 1 {
		t.Fatalf("errors: %+v", p.Errors)
	}
	got := p.Errors[0]
	if got.Path != libPath || !strings.Contains(got.Msg, "mismatched types") {
		t.Fatalf("error: %+v", got)
	}
	if got.Pos.Line != 2 || got.Pos.Col < 2 {
		t.Fatalf("position: %+v", got.Pos)
	}
	var buf bytes.Buffer
	if err := p.WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var diags []JSONError
	if err := json.Unmarshal(buf.Bytes(), &diags); err != nil {
		t.Fatal(err)
	}
	if len(diags) != 1 {
		t.Fatalf("json: %s", buf.String())
	}
	d := diags[0]
	if d.File != libPath || d.Line != 2 || d.Col < 1 || d.EndLine != 2 || d.EndCol <= d.Col {
		t.Fatalf("json diagnostic: %+v", d)
	}
	if d.Severity != "error" || d.Code == "" || !strings.Contains(d.Message, "mismatched types") {
		t.Fatalf("json fields: %+v", d)
	}
	if err := p.Check(CheckOptions{Typecheck: true, Fast: true}); err != nil {
		t.Fatal(err)
	}
	if len(p.Errors) != 0 {
		t.Fatalf("fast check: %+v", p.Errors)
	}
	buf.Reset()
	if err := p.WriteJSON(&buf); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("clean json: %s", buf.String())
	}
}

func TestFormatListJSON(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.ink")
	src := []byte("fn  main() void {\n    var x = 1;\n};\n")
	if err := os.WriteFile(bad, src, 0o644); err != nil {
		t.Fatal(err)
	}
	good := filepath.Join(dir, "good.ink")
	if err := os.WriteFile(good, src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Format(true, false, false, []string{good}, nil); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Format(false, true, true, []string{dir}, &buf); err != nil {
		t.Fatal(err)
	}
	var listed []string
	if err := json.Unmarshal(buf.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0] != bad {
		t.Fatalf("listed: %v", listed)
	}
	if err := Format(true, false, false, []string{bad}, nil); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := Format(false, true, true, []string{dir}, &buf); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("formatted json: %s", buf.String())
	}
}

func TestCheckTestTypeErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/tc\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/tc\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	main := "test \"root\" {\n    assert(true);\n};\n\nfn main() void {\n    assert(true);\n};\n"
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte(main), 0o644); err != nil {
		t.Fatal(err)
	}
	lib := "test \"bad\" {\n    var x = 1 + \"s\";\n    assert(x == true);\n};\n"
	libPath := filepath.Join(dir, "lib", "lib.ink")
	if err := os.WriteFile(libPath, []byte(lib), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Typecheck: true}); err != nil {
		t.Fatal(err)
	}
	if len(p.Errors) != 1 {
		t.Fatalf("errors: %+v", p.Errors)
	}
	got := p.Errors[0]
	if got.Path != libPath || !strings.Contains(got.Msg, "mismatched types") || got.Pos.Line != 2 {
		t.Fatalf("error: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "lib", "zz_ink_test.go")); !os.IsNotExist(err) {
		t.Fatal("zz_ink_test.go was written to the source tree")
	}
}

func TestConformanceCases(t *testing.T) {
	base := filepath.Join("..", "..", "tests", "cases")
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Skip(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(base, entry.Name(), "expected-fast.txt")); err == nil {
				t.Skip("fast-check case")
			}
			if _, err := os.Stat(filepath.Join(base, entry.Name(), "expected-check.txt")); err == nil {
				t.Skip("check case")
			}
			dir := filepath.Join(t.TempDir(), entry.Name())
			copyTree(t, filepath.Join(base, entry.Name()), dir)
			p, err := Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(dir, "out")
			if err := p.Build(out, nil); err != nil {
				p.PrintErrors(os.Stderr)
				t.Fatal(err)
			}
			hasError := false
			for _, e := range p.Errors {
				if !e.Warning {
					hasError = true
				}
			}
			if hasError {
				p.PrintErrors(os.Stderr)
				t.Fatal("check failed")
			}
			cmd := exec.Command(out)
			cmd.Dir = dir
			var stderr strings.Builder
			cmd.Stderr = &stderr
			got, err := cmd.Output()
			if err != nil {
				t.Fatalf("%v\n%s", err, stderr.String())
			}
			expected, err := os.ReadFile(filepath.Join(dir, "expected.txt"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(expected) {
				t.Fatalf("output mismatch:\n--- got ---\n%s--- want ---\n%s", got, expected)
			}
			if _, err := os.Stat(filepath.Join(dir, "test")); err == nil {
				if err := p.Generate(true); err != nil {
					t.Fatal(err)
				}
				testCmd := exec.Command("go", "test", "-buildvcs=false", "./...")
				testCmd.Dir = dir
				if output, err := testCmd.CombinedOutput(); err != nil {
					t.Fatalf("go test: %v\n%s", err, output)
				}
			}
		})
	}
}

func TestFormatCommentFixedPoint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.ink")
	src := "use fmt; // note\n\nstruct User {\n    Name: str,\n}; // point\n\nfn main() void {\n    var x = match (1) {\n    case 1 => fmt.Println(\"one\");\n    else => fmt.Println(\"other\"); // tail\n    };\n    _ = x;\n}; // done\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Format(true, false, false, []string{path}, nil); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := Format(false, true, true, []string{dir}, &buf); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Fatalf("still unformatted: %s", buf.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"use fmt; // note", "}; // point", "// tail"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

func TestNewWritesAgentGuide(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "guided")
	if err := New(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ink check --json", "ink run", "ink guide"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("AGENTS.md missing %q", want)
		}
	}
	if strings.Contains(string(data), "design/SPEC.md") {
		t.Fatal("AGENTS.md points at design/SPEC.md, which a new project does not have")
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(cfg) || !strings.Contains(string(cfg), `"extensions": [".ink"]`) {
		t.Fatalf("opencode.json: %s", cfg)
	}
	var lspCfg struct {
		LSP map[string]struct {
			Command []string `json:"command"`
		} `json:"lsp"`
	}
	if err := json.Unmarshal(cfg, &lspCfg); err != nil {
		t.Fatalf("opencode.json: %v", err)
	}
	cmd := lspCfg.LSP["ink"].Command
	if len(cmd) != 2 || cmd[0] == "" || cmd[1] != "lsp" {
		t.Fatalf("lsp command: %+v", cmd)
	}
	var mcpCfg struct {
		MCP struct {
			Servers map[string]struct {
				Type    string   `json:"type"`
				Command []string `json:"command"`
			} `json:"servers"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(cfg, &mcpCfg); err != nil {
		t.Fatalf("opencode.json: %v", err)
	}
	mcpCmd := mcpCfg.MCP.Servers["ink"]
	if mcpCmd.Type != "local" || len(mcpCmd.Command) != 2 || mcpCmd.Command[1] != "mcp" {
		t.Fatalf("mcp config: %+v", mcpCmd)
	}
	plugin, err := os.ReadFile(filepath.Join(dir, ".opencode", "plugins", "ink", "index.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plugin), "ctx.session.synthetic") || !strings.Contains(string(plugin), `"check", "--json"`) {
		t.Fatalf("plugin:\n%s", plugin)
	}
	skill, err := os.ReadFile(filepath.Join(dir, ".opencode", "skills", "ink", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(skill), "Canonical syntax") {
		t.Fatalf("SKILL.md: %s", skill)
	}
}

func TestInitWritesAgentGuideOnce(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ink check --json", "ink run"} {
		if !strings.Contains(string(first), want) {
			t.Fatalf("AGENTS.md missing %q", want)
		}
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("Init changed AGENTS.md")
	}
}

func TestInitPreservesAgentGuide(t *testing.T) {
	dir := t.TempDir()
	custom := []byte("# custom guide\n")
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, custom) {
		t.Fatal("Init overwrote AGENTS.md")
	}
}

func TestBuildEmbedWritesAssetsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/emb\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "web", "index.html"), []byte("<h1>x</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte("fn main() void {\n    var data, ok = asset(\"web/index.html\");\n    _, _ = data, ok;\n};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Typecheck: true}); err != nil {
		t.Fatal(err)
	}
	if p.HasErrors() {
		t.Fatalf("errors: %+v", p.Errors)
	}
	p.Embed = []string{"web"}
	if err := p.Generate(false); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(dir, "zz_ink_assets.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "//go:embed web") || !strings.Contains(string(src), "package main") {
		t.Fatalf("assets file:\n%s", src)
	}
}

func TestSanitizeGeneratedNames(t *testing.T) {
	cases := map[string]string{
		"cannot use _s_v8 (variable of type int64) as int value in assignment": "cannot use value (variable of type int64) as int value in assignment",
		"cannot use b (variable of type int) as int64 value":                   "cannot use b (variable of type int) as int64 value",
	}
	for in, want := range cases {
		if got := sanitizeMessage(in); got != want {
			t.Fatalf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestPrintErrorsIncludesHint(t *testing.T) {
	p := &Project{Errors: []Error{{Path: "main.ink", Pos: token.Pos{Line: 1, Col: 1}, Msg: "boom", Hint: "fix it"}}}
	var buf bytes.Buffer
	p.PrintErrors(&buf)
	if !strings.Contains(buf.String(), "hint: fix it") {
		t.Fatalf("output = %q", buf.String())
	}
}

func TestNeedsRace(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/race\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "fn work() void {\n};\n\nfn main() void {\n    par {\n        work();\n        work();\n    };\n};\n"
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{Fast: true}); err != nil {
		t.Fatal(err)
	}
	if !needsRace(p.Program) {
		t.Fatal("par should need race")
	}
}

func TestStripNoRace(t *testing.T) {
	args, no := stripNoRace([]string{"-v", "--no-race", "-count=1"})
	if !no || len(args) != 2 {
		t.Fatalf("args = %v no = %v", args, no)
	}
}
