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
