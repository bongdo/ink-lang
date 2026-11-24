package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolsListed(t *testing.T) {
	want := []string{"diagnose", "hover", "definition", "references", "complete", "symbols", "rename", "quickfix"}
	for _, name := range want {
		found := false
		for _, tool := range tools {
			if tool.Name == name {
				found = true
			}
		}
		if !found {
			t.Fatalf("tool %q missing", name)
		}
	}
}

func TestFormatEdits(t *testing.T) {
	raw := json.RawMessage(`{"changes":{"file:///p/main.ink":[{"range":{"start":{"line":1,"character":4}},"newText":"sum"}]}}`)
	got := formatEdits(raw)
	if !strings.Contains(got, "/p/main.ink:2:5 -> sum") {
		t.Fatalf("edits = %q", got)
	}
}

func TestFormatCompletions(t *testing.T) {
	raw := json.RawMessage(`{"items":[{"label":"Name","detail":"str"},{"label":"Add"}]}`)
	got := formatCompletions(raw)
	if !strings.Contains(got, "Name — str") || !strings.Contains(got, "Add") {
		t.Fatalf("completions = %q", got)
	}
}
