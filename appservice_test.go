package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vdbergkevin/vibe-dock/internal/model"
	"github.com/vdbergkevin/vibe-dock/internal/store"
)

func TestSetThemePersistsValidatedPreference(t *testing.T) {
	dataStore, err := store.Open(filepath.Join(t.TempDir(), "theme.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAppService(dataStore, "")
	defer service.Close()

	if err := service.SetTheme("light"); err != nil {
		t.Fatal(err)
	}
	if got := dataStore.Setting(t.Context(), "theme"); got != "light" {
		t.Fatalf("theme was not persisted: got %q", got)
	}
	if err := service.SetTheme("sepia"); err == nil {
		t.Fatal("expected unsupported theme to be rejected")
	}
	if got := dataStore.Setting(t.Context(), "theme"); got != "light" {
		t.Fatalf("invalid theme changed the preference: got %q", got)
	}
}

func TestSetAccentThemePersistsValidatedPreference(t *testing.T) {
	dataStore, err := store.Open(filepath.Join(t.TempDir(), "accent.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAppService(dataStore, "")
	defer service.Close()
	if err := service.SetAccentTheme("tide"); err != nil {
		t.Fatal(err)
	}
	if got := dataStore.Setting(t.Context(), "accent_theme"); got != "tide" {
		t.Fatalf("accent theme was not persisted: got %q", got)
	}
	if err := service.SetAccentTheme("neon-rainbow"); err == nil {
		t.Fatal("expected unsupported accent theme to be rejected")
	}
}

func TestSetConnectorEnabledPersistsVibeAccess(t *testing.T) {
	vibeHome := t.TempDir()
	t.Setenv("VIBE_HOME", vibeHome)
	dataStore, err := store.Open(filepath.Join(t.TempDir(), "connectors.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAppService(dataStore, "")
	defer service.Close()

	inventory, err := service.SetConnectorEnabled("", "github_app", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Sources) != 1 || inventory.Sources[0].Name != "github_app" || !inventory.Sources[0].Enabled {
		t.Fatalf("connector was not enabled in inventory: %#v", inventory.Sources)
	}
	contents, err := os.ReadFile(filepath.Join(vibeHome, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); got != "[[connectors]]\nname = \"github_app\"\ndisabled = false\n" {
		t.Fatalf("unexpected Vibe connector config:\n%s", got)
	}

	inventory, err = service.SetConnectorEnabled("", "github_app", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Sources) != 1 || inventory.Sources[0].Enabled || inventory.Sources[0].Status != "disabled" {
		t.Fatalf("connector was not disabled in inventory: %#v", inventory.Sources)
	}
}

func TestConnectorContextDescribesEnabledManagedTools(t *testing.T) {
	vibeHome := t.TempDir()
	t.Setenv("VIBE_HOME", vibeHome)
	if err := os.WriteFile(filepath.Join(vibeHome, "config.toml"), []byte("connectors = [{ name = \"github_app\", disabled = false }]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := fmt.Sprintf(`{"account":{"stored_at_timestamp":%d,"payload":{"connectors":[{"id":"github-id","name":"github_app","status":{"is_ready":true},"tools":[{"name":"get_me"},{"name":"search_repositories"}]}]}}}`, time.Now().Unix())
	if err := os.WriteFile(filepath.Join(vibeHome, "connector_bootstrap_cache.json"), []byte(cache), 0o600); err != nil {
		t.Fatal(err)
	}
	dataStore, err := store.Open(filepath.Join(t.TempDir(), "connector-context.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAppService(dataStore, "")
	defer service.Close()

	context := service.connectorContext("")
	for _, expected := range []string{"GitHub", "connector_github_app_", "2 tools", "local desktop client"} {
		if !strings.Contains(context, expected) {
			t.Fatalf("connector context missing %q: %s", expected, context)
		}
	}
}

func TestConversationSubjectSummarisesFirstPrompt(t *testing.T) {
	tests := map[string]string{
		"Can you please add an integrated terminal to the bottom of the screen?":         "Add an integrated terminal to the bottom of the screen",
		"I would love to implement selectable colour themes with a few polished options": "Implement selectable colour themes with a few polished options",
		"Fix login": "Fix login",
	}
	for prompt, want := range tests {
		if got := conversationSubject(prompt); got != want {
			t.Errorf("conversationSubject(%q) = %q, want %q", prompt, got, want)
		}
	}
}

func TestSetWorkspaceKindPersistsValidatedPreference(t *testing.T) {
	dataStore, err := store.Open(filepath.Join(t.TempDir(), "workspace.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAppService(dataStore, "")
	defer service.Close()

	if err := service.SetWorkspaceKind("chat"); err != nil {
		t.Fatal(err)
	}
	if got := dataStore.Setting(t.Context(), "workspace_kind"); got != "chat" {
		t.Fatalf("workspace was not persisted: got %q", got)
	}
	if err := service.SetWorkspaceKind("work"); err != nil {
		t.Fatal(err)
	}
	if got := dataStore.Setting(t.Context(), "workspace_kind"); got != "work" {
		t.Fatalf("work workspace was not persisted: got %q", got)
	}
	if err := service.SetWorkspaceKind("documents"); err == nil {
		t.Fatal("expected unsupported workspace to be rejected")
	}
}

func TestOpenProjectTerminalUsesCodeProjectPath(t *testing.T) {
	dataStore, err := store.Open(filepath.Join(t.TempDir(), "terminal.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAppService(dataStore, "")
	defer service.Close()
	projectRoot := t.TempDir()
	project, err := dataStore.AddProject(t.Context(), projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	openedPath := ""
	service.terminalLauncher = func(path string) error {
		openedPath = path
		return nil
	}
	if err := service.OpenProjectTerminal(project.ID); err != nil {
		t.Fatal(err)
	}
	if openedPath != projectRoot {
		t.Fatalf("opened terminal at %q, want %q", openedPath, projectRoot)
	}
}

func TestOpenProjectTerminalRejectsChatProject(t *testing.T) {
	dataStore, err := store.Open(filepath.Join(t.TempDir(), "terminal.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAppService(dataStore, "")
	defer service.Close()
	project, err := dataStore.AddChatProject(t.Context(), "Ideas", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.OpenProjectTerminal(project.ID); err == nil {
		t.Fatal("expected chat project terminal to be rejected")
	}
}

func TestOpenProjectInEditorUsesPreferenceAndCodeProjectPath(t *testing.T) {
	dataStore, err := store.Open(filepath.Join(t.TempDir(), "vscodium.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAppService(dataStore, "")
	defer service.Close()
	projectRoot := t.TempDir()
	project, err := dataStore.AddProject(t.Context(), projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SetCodeEditor("cursor"); err != nil {
		t.Fatal(err)
	}
	openedEditor, openedPath := "", ""
	service.editorLauncher = func(editorID, path string) error {
		openedEditor = editorID
		openedPath = path
		return nil
	}
	if err := service.OpenProjectInEditor(project.ID); err != nil {
		t.Fatal(err)
	}
	if openedEditor != "cursor" {
		t.Fatalf("opened editor %q, want cursor", openedEditor)
	}
	if openedPath != projectRoot {
		t.Fatalf("opened editor at %q, want %q", openedPath, projectRoot)
	}
}

func TestSetCodeEditorPersistsValidatedPreference(t *testing.T) {
	dataStore, err := store.Open(filepath.Join(t.TempDir(), "editor.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAppService(dataStore, "")
	defer service.Close()

	if err := service.SetCodeEditor("zed"); err != nil {
		t.Fatal(err)
	}
	if got := dataStore.Setting(t.Context(), "code_editor"); got != "zed" {
		t.Fatalf("code editor was not persisted: got %q", got)
	}
	if err := service.SetCodeEditor("unknown"); err == nil {
		t.Fatal("expected unsupported code editor to be rejected")
	}
	if got := dataStore.Setting(t.Context(), "code_editor"); got != "zed" {
		t.Fatalf("invalid editor changed the preference: got %q", got)
	}
}

func TestSkillLifecycleUsesVibeLocations(t *testing.T) {
	vibeHome := t.TempDir()
	t.Setenv("VIBE_HOME", vibeHome)
	dataStore, err := store.Open(filepath.Join(t.TempDir(), "skills.db"))
	if err != nil {
		t.Fatal(err)
	}
	service := NewAppService(dataStore, "")
	defer service.Close()
	projectRoot := t.TempDir()
	project, err := dataStore.AddProject(t.Context(), projectRoot)
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.SaveSkill(model.Skill{
		Name: "release-check", Description: "Check a release before publishing.", Instructions: "# Release check\n\nRun the release checklist.",
		Scope: "project", ProjectID: project.ID, UserInvocable: true, AllowedTools: []string{"read_file"},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(projectRoot, ".vibe", "skills", "release-check", "SKILL.md")
	if created.Path != wantPath {
		t.Fatalf("skill path = %q, want %q", created.Path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatal(err)
	}

	inventory, err := service.Skills(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(inventory.Skills) != 1 || inventory.Skills[0].Name != "release-check" {
		t.Fatalf("unexpected skill inventory: %#v", inventory)
	}
	if err := service.SetSkillEnabled("project", project.ID, "release-check", false); err != nil {
		t.Fatal(err)
	}
	inventory, _ = service.Skills(project.ID)
	if inventory.Skills[0].Enabled {
		t.Fatal("expected project skill to be disabled")
	}
	if err := service.DeleteSkill("project", project.ID, "release-check"); err != nil {
		t.Fatal(err)
	}
}
