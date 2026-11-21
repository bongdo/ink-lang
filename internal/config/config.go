package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	FileName       = "ink.conf"
	LegacyFileName = "ink.toml"
)

type Config struct {
	Name   string
	Module string
	Go     string
}

var nameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

func FindRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		if Exists(abs) {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("%s not found in %s or above", FileName, dir)
		}
		abs = parent
	}
}

func fileAt(root string) (string, bool) {
	for _, name := range []string{FileName, LegacyFileName} {
		path := filepath.Join(root, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, true
		}
	}
	return "", false
}

func Exists(root string) bool {
	_, ok := fileAt(root)
	return ok
}

func Load(root string) (*Config, error) {
	path, ok := fileAt(root)
	if !ok {
		return nil, fmt.Errorf("%s not found in %s", FileName, root)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if cfg.Name == "" {
		cfg.Name = filepath.Base(root)
	}
	if !nameRE.MatchString(cfg.Name) {
		return nil, fmt.Errorf("invalid project name %q", cfg.Name)
	}
	if cfg.Module == "" {
		cfg.Module = "example.com/" + cfg.Name
	}
	if strings.ContainsAny(cfg.Module, " \t\n") {
		return nil, fmt.Errorf("invalid module path %q", cfg.Module)
	}
	if cfg.Go == "" {
		cfg.Go = "1.27"
	}
	return cfg, nil
}

func parse(data []byte) (*Config, error) {
	cfg := &Config{}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = value", i+1)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, `"`) {
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			value = unquoted
		}
		switch key {
		case "name":
			cfg.Name = value
		case "module":
			cfg.Module = value
		case "go":
			cfg.Go = value
		default:
			return nil, fmt.Errorf("line %d: unknown key %q", i+1, key)
		}
	}
	return cfg, nil
}

func Save(root string, cfg *Config) error {
	var b strings.Builder
	fmt.Fprintf(&b, "name = %q\n", cfg.Name)
	fmt.Fprintf(&b, "module = %q\n", cfg.Module)
	fmt.Fprintf(&b, "go = %q\n", cfg.Go)
	return os.WriteFile(filepath.Join(root, FileName), []byte(b.String()), 0o644)
}

func ReadModule(root string) (string, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", errors.New("module line not found in go.mod")
}
