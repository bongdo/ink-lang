package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConf(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("name = \"hello\"\nmodule = \"example.com/hello\"\ngo = \"1.27\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "hello" || cfg.Module != "example.com/hello" || cfg.Go != "1.27" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadLegacyToml(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, LegacyFileName), []byte("name = \"legacy\"\nmodule = \"example.com/legacy\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "legacy" || cfg.Module != "example.com/legacy" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestSaveWritesConf(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, &Config{Name: "x", Module: "example.com/x", Go: "1.27"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, FileName)); err != nil {
		t.Fatal(err)
	}
}

func TestUnknownKey(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("name = \"x\"\nwat = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("unknown key accepted")
	}
}
