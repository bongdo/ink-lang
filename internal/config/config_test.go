package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGoMod(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("module example.com/hello\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != filepath.Base(dir) || cfg.Module != "example.com/hello" || cfg.Go != "1.27" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadBareDir(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Module != "example.com/"+filepath.Base(dir) || cfg.Go != "1.27" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestSaveWritesGoMod(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, &Config{Name: "x", Module: "example.com/x", Go: "1.27"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "module example.com/x\n\ngo 1.27\n" {
		t.Fatalf("go.mod = %q", data)
	}
	if !Exists(dir) {
		t.Fatal("Exists = false")
	}
}

func TestFindRootPrefersGoMod(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte("module example.com/root\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, "lib")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, "main.ink"), []byte("fn main() void {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindRoot(child)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("root = %s, want %s", got, root)
	}
}

func TestFindRootFallsBackToMain(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.ink"), []byte("fn main() void {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("root = %s, want %s", got, root)
	}
}
