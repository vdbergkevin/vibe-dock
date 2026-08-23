package vibe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/vdbergkevin/vibe-dock/internal/model"
)

func TestSaveDiscoverToggleAndDeleteSkill(t *testing.T) {
	vibeHome := t.TempDir()
	input := model.Skill{
		Name:          "code-review",
		Description:   "Review code with a focused checklist.",
		Instructions:  "# Code review\n\nInspect the diff and report concrete findings.",
		Scope:         "global",
		UserInvocable: true,
		AllowedTools:  []string{"read_file", "grep"},
	}
	saved, err := SaveSkill(vibeHome, "", input)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Name != input.Name || !saved.Enabled || saved.Risk != "limited" {
		t.Fatalf("unexpected saved skill: %#v", saved)
	}
	if _, err := os.Stat(filepath.Join(vibeHome, "skills", "code-review", "SKILL.md")); err != nil {
		t.Fatal(err)
	}

	if err := SetSkillEnabled(vibeHome, "", "global", "code-review", false); err != nil {
		t.Fatal(err)
	}
	inventory := DiscoverSkills(vibeHome, "", "", "")
	if len(inventory.Skills) != 1 || inventory.Skills[0].Enabled {
		t.Fatalf("expected disabled discovered skill, got %#v", inventory.Skills)
	}
	config, err := os.ReadFile(filepath.Join(vibeHome, "config.toml"))
	if err != nil || !strings.Contains(string(config), `disabled_skills = ["code-review"]`) {
		t.Fatalf("unexpected config: %q, %v", config, err)
	}

	if err := SetSkillEnabled(vibeHome, "", "global", "code-review", true); err != nil {
		t.Fatal(err)
	}
	if err := DeleteSkill(vibeHome, "", "global", "code-review"); err != nil {
		t.Fatal(err)
	}
	if got := DiscoverSkills(vibeHome, "", "", "").Skills; len(got) != 0 {
		t.Fatalf("expected deleted skill to disappear, got %#v", got)
	}
}

func TestWriteSkillFiltersPreservesTablesAndTopLevelPlacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	existing := "theme = \"auto\"\n\n[tools.bash]\npermission = \"ask\"\n"
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeSkillFilters(path, nil, []string{"dangerous"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	if strings.Index(text, "disabled_skills") > strings.Index(text, "[tools.bash]") {
		t.Fatalf("skill filter was written inside the tools table:\n%s", text)
	}
	var decoded map[string]any
	if _, err := toml.Decode(text, &decoded); err != nil {
		t.Fatalf("rewritten config is invalid TOML: %v\n%s", err, text)
	}
}

func TestImportSkillRejectsSymlinks(t *testing.T) {
	vibeHome := t.TempDir()
	source := t.TempDir()
	content := "---\nname: linked-skill\ndescription: Test import\nuser-invocable: true\n---\n\n# Instructions\n"
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(source, "SKILL.md"), filepath.Join(source, "linked.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := ImportSkill(vibeHome, "", source, "global", ""); err == nil {
		t.Fatal("expected symlink import to be rejected")
	}
}
