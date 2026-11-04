package agent

import (
	"os"
	"testing"
)

func TestRootGuideMatchesEmbeddedShort(t *testing.T) {
	root, err := os.ReadFile("../../AGENTS.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(root) != ShortGuide {
		t.Fatal("AGENTS.md differs from internal/agent/short.md")
	}
}

func TestSkillMatchesEmbeddedGuide(t *testing.T) {
	raw, err := os.ReadFile("../../.opencode/skills/ink/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != Skill {
		t.Fatal("SKILL.md differs from the embedded skill")
	}
}
