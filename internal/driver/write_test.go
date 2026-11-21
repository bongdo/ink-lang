package driver

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteIfChanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zz_ink.go")
	first := []byte("package a\n\nvar X = 1\n")
	wrote, err := writeIfChanged(path, first)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("first write reported no change")
	}
	sentinel := time.Unix(1000, 0)
	if err := os.Chtimes(path, sentinel, sentinel); err != nil {
		t.Fatal(err)
	}
	wrote, err = writeIfChanged(path, first)
	if err != nil {
		t.Fatal(err)
	}
	if wrote {
		t.Fatal("identical write reported a change")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(sentinel) {
		t.Fatalf("identical write touched mtime: %v", info.ModTime())
	}
	second := []byte("package a\n\nvar X = 2\n")
	wrote, err = writeIfChanged(path, second)
	if err != nil {
		t.Fatal(err)
	}
	if !wrote {
		t.Fatal("changed write reported no change")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, second) {
		t.Fatalf("content: %q", got)
	}
}

func TestGenerateRemovesStalePackages(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("ink.conf", "name = \"stale\"\nmodule = \"example.com/stale\"\n")
	write("main.ink", "fn main() void {\n    assert(true);\n};\n")
	write("a/a.ink", "fn A() i32 {\n    return 1;\n};\n")
	write("b/b.ink", "fn B() i32 {\n    return 2;\n};\n")

	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "bin", "stale")
	if err := p.Build(out, nil); err != nil {
		p.PrintErrors(os.Stderr)
		t.Fatal(err)
	}
	for _, rel := range []string{"a/zz_ink.go", "b/zz_ink.go"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
	}
	if got := readManifestJSON(t, p); len(got) != 3 {
		t.Fatalf("manifest after first build: %v", got)
	}

	aGen := filepath.Join(dir, "a", "zz_ink.go")
	sentinel := time.Unix(2000, 0)
	if err := os.Chtimes(aGen, sentinel, sentinel); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "b", "b.ink")); err != nil {
		t.Fatal(err)
	}
	if err := p.Build(out, nil); err != nil {
		p.PrintErrors(os.Stderr)
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "b", "zz_ink.go")); !os.IsNotExist(err) {
		t.Fatalf("stale b/zz_ink.go: %v", err)
	}
	info, err := os.Stat(aGen)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(sentinel) {
		t.Fatalf("a/zz_ink.go was rewritten: %v", info.ModTime())
	}
	manifest := readManifestJSON(t, p)
	for _, rel := range manifest {
		if rel == "b/zz_ink.go" {
			t.Fatalf("manifest keeps b: %v", manifest)
		}
	}
	found := false
	for _, rel := range manifest {
		if rel == "a/zz_ink.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("manifest lost a: %v", manifest)
	}
}

func TestGenerateSurvivesBrokenManifest(t *testing.T) {
	outer := t.TempDir()
	dir := filepath.Join(outer, "proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ink.conf"), []byte("name = \"mf\"\nmodule = \"example.com/mf\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ink"), []byte("fn main() void {\n};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(outer, "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(dir, ".ink", "generated.json")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(`["../keep.txt"]`), 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Check(CheckOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := p.Generate(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("file outside the project was removed: %v", err)
	}

	if err := os.WriteFile(manifest, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := p.Generate(false); err != nil {
		t.Fatal(err)
	}
	got := readManifestJSON(t, p)
	if len(got) != 1 || got[0] != "zz_ink.go" {
		t.Fatalf("manifest after broken input: %v", got)
	}
}

func readManifestJSON(t *testing.T, p *Project) []string {
	t.Helper()
	data, err := os.ReadFile(p.manifestPath())
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	if err := json.Unmarshal(data, &files); err != nil {
		t.Fatalf("manifest %q: %v", data, err)
	}
	return files
}
