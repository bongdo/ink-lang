package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const FileName = "go.mod"

type Config struct {
	Name   string
	Module string
	Go     string
}

func FindRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	mainRoot := ""
	for {
		if Exists(abs) {
			return abs, nil
		}
		if mainRoot == "" {
			if _, err := os.Stat(filepath.Join(abs, "main.ink")); err == nil {
				mainRoot = abs
			}
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			if mainRoot != "" {
				return mainRoot, nil
			}
			return abs, nil
		}
		abs = parent
	}
}

func GoEnv() []string {
	flags := os.Getenv("GOFLAGS")
	if flags == "" {
		flags = "-buildvcs=false"
	} else {
		flags += " -buildvcs=false"
	}
	return append(os.Environ(), "GOFLAGS="+flags)
}

func Exists(root string) bool {
	info, err := os.Stat(filepath.Join(root, FileName))
	return err == nil && !info.IsDir()
}

func Load(root string) (*Config, error) {
	name := filepath.Base(root)
	cfg := &Config{Name: name, Module: "example.com/" + name, Go: "1.27"}
	data, err := os.ReadFile(filepath.Join(root, FileName))
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "module "):
			cfg.Module = strings.TrimSpace(strings.TrimPrefix(line, "module "))
		case strings.HasPrefix(line, "go "):
			cfg.Go = strings.TrimSpace(strings.TrimPrefix(line, "go "))
		}
	}
	if strings.ContainsAny(cfg.Module, " \t\n") {
		return nil, fmt.Errorf("invalid module path %q", cfg.Module)
	}
	return cfg, nil
}

func Save(root string, cfg *Config) error {
	var b strings.Builder
	fmt.Fprintf(&b, "module %s\n\ngo %s\n", cfg.Module, cfg.Go)
	return os.WriteFile(filepath.Join(root, FileName), []byte(b.String()), 0o644)
}

func ReadModule(root string) (string, error) {
	cfg, err := Load(root)
	if err != nil {
		return "", err
	}
	return cfg.Module, nil
}
