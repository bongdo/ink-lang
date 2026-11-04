package driver

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	if err := os.WriteFile(filepath.Join(dir, "ink.toml"), []byte("name = \"tc\"\nmodule = \"example.com/tc\"\n"), 0o644); err != nil {
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

func TestCheckFastAndJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ink.toml"), []byte("name = \"tc\"\nmodule = \"example.com/tc\"\n"), 0o644); err != nil {
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
	if got.End.Line != got.Pos.Line || got.End.Col <= got.Pos.Col {
		t.Fatalf("end: %+v", got.End)
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
	if d.File != libPath || d.Line != 2 || d.Col != got.Pos.Col || d.EndLine != 2 || d.EndCol != d.Col+1 {
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
	if err := os.WriteFile(filepath.Join(dir, "ink.toml"), []byte("name = \"tc\"\nmodule = \"example.com/tc\"\n"), 0o644); err != nil {
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
			if len(p.Errors) > 0 {
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

func TestNewWritesAgentGuide(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "guided")
	if err := New(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ink check --json", "ink run"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("AGENTS.md missing %q", want)
		}
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(cfg) || !strings.Contains(string(cfg), `"extensions": [".ink"]`) {
		t.Fatalf("opencode.json: %s", cfg)
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
	if err := Init(dir); err == nil {
		t.Fatal("second Init succeeded")
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
