package driver

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOpenCodeConfigCommand(t *testing.T) {
	cfg := openCodeConfig([]string{"go", "tool", "ink"})
	var parsed struct {
		LSP map[string]struct {
			Command []string `json:"command"`
		} `json:"lsp"`
		Formatter map[string]struct {
			Command []string `json:"command"`
		} `json:"formatter"`
		MCP struct {
			Servers map[string]struct {
				Command []string `json:"command"`
			} `json:"servers"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(cfg, &parsed); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		got  []string
		want []string
	}{
		{parsed.LSP["ink"].Command, []string{"go", "tool", "ink", "lsp"}},
		{parsed.Formatter["ink"].Command, []string{"go", "tool", "ink", "fmt", "-w", "$FILE"}},
		{parsed.MCP.Servers["ink"].Command, []string{"go", "tool", "ink", "mcp"}},
	}
	for _, tc := range cases {
		if !reflect.DeepEqual(tc.got, tc.want) {
			t.Fatalf("command = %v, want %v", tc.got, tc.want)
		}
	}
}

func TestInkCommandUsesToolDirective(t *testing.T) {
	dir := t.TempDir()
	gomod := "module example.com/app\n\ngo 1.27\n\ntool " + mainModulePath() + "/cmd/ink\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644); err != nil {
		t.Fatal(err)
	}
	got := inkCommand(dir)
	want := []string{"go", "tool", "ink"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inkCommand = %v, want %v", got, want)
	}
}
