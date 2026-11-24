package typecheck

import (
	"path/filepath"
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
