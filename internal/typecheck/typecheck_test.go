package typecheck

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCachePutKeepsErrorsUnderRoot(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"ink", filepath.Join(root, "main.ink"), true},
		{"generated", filepath.Join(root, "zz_ink.go"), true},
		{"outside", filepath.Join(filepath.Dir(root), "elsewhere.ink"), false},
	}
	for _, tc := range cases {
		key := "test-" + tc.name
		cachePut(root, key, []Error{{Path: tc.path, Msg: "boom"}})
		if _, ok := cacheGet(key); ok != tc.want {
			t.Fatalf("%s: cached = %v, want %v", tc.name, ok, tc.want)
		}
	}
}

func TestVetErrorsMapToInkFiles(t *testing.T) {
	root := t.TempDir()
	out := "# example.com/x\n" +
		root + "/main.ink:4:13: conversion from untyped int to string yields a string of one rune\n" +
		"/elsewhere/main.ink:1:1: ignored\n" +
		"vet: something else\n"
	errs := vetErrors(root, out)
	if len(errs) != 1 {
		t.Fatalf("errs: %+v", errs)
	}
	if e := errs[0]; e.Path != filepath.Join(root, "main.ink") || e.Pos.Line != 4 || e.Pos.Col != 13 || !e.Vet {
		t.Fatalf("err: %+v", e)
	}
}

func TestAbsoluteLineDirectives(t *testing.T) {
	src := []byte("//line main.ink:3:5\nvar x = 1\n//line lib/lib.ink:2:1\n")
	got := string(absoluteLineDirectives(src, "/p"))
	if !strings.Contains(got, "//line /p/main.ink:3:5") || !strings.Contains(got, "//line /p/lib/lib.ink:2:1") {
		t.Fatalf("got:\n%s", got)
	}
}
