package agent

import (
	"testing"

	acp "github.com/coder/acp-go-sdk"
	"github.com/vdbergkevin/vibe-dock/internal/model"
)

func TestConvertPluginsResolvesEnvironmentReferences(t *testing.T) {
	t.Setenv("VIBE_TEST_TOKEN", "secret-from-process")
	servers := convertPlugins([]model.Plugin{{
		Name: "Example", Transport: "stdio", Command: "example-mcp", Enabled: true,
		Env: map[string]string{"API_TOKEN": "VIBE_TEST_TOKEN", "MISSING": "NOT_DEFINED"},
	}})
	if len(servers) != 1 || servers[0].Stdio == nil {
		t.Fatalf("unexpected servers: %#v", servers)
	}
	env := servers[0].Stdio.Env
	if len(env) != 1 || env[0].Name != "API_TOKEN" || env[0].Value != "secret-from-process" {
		t.Fatalf("unexpected resolved environment: %#v", env)
	}
}

func TestProjectConfigOptionsProjectsModelAndThinkingChoices(t *testing.T) {
	category := acp.SessionConfigOptionCategoryModel
	description := "mistral-medium-latest"
	choices := acp.SessionConfigSelectOptionsUngrouped{
		{Value: "medium", Name: "Mistral Medium", Description: &description},
		{Value: "local", Name: "Devstral local"},
	}
	options := projectConfigOptions([]acp.SessionConfigOption{{Select: &acp.SessionConfigOptionSelect{
		Id:           "model",
		Name:         "Model",
		Category:     &category,
		CurrentValue: "medium",
		Options:      acp.SessionConfigSelectOptions{Ungrouped: &choices},
		Type:         "select",
	}}})
	if len(options) != 1 {
		t.Fatalf("expected one projected option, got %#v", options)
	}
	option := options[0]
	if option.ID != "model" || option.Category != "model" || option.CurrentValue != "medium" {
		t.Fatalf("unexpected projected model option: %#v", option)
	}
	if len(option.Options) != 2 || option.Options[0].Description != description {
		t.Fatalf("unexpected model choices: %#v", option.Options)
	}
}

func TestProjectCommandsProjectsInputHints(t *testing.T) {
	commands := projectCommands([]acp.AvailableCommand{
		{Name: "/compact", Description: "Compact the current session"},
		{
			Name:        "teleport",
			Description: "Move a session",
			Input: &acp.AvailableCommandInput{Unstructured: &acp.UnstructuredCommandInput{
				Hint: "session id",
			}},
		},
	})
	if len(commands) != 2 {
		t.Fatalf("expected two projected commands, got %#v", commands)
	}
	if commands[0].Name != "compact" || commands[0].Source != "vibe" {
		t.Fatalf("unexpected projected command: %#v", commands[0])
	}
	if commands[1].Name != "teleport" || commands[1].InputHint != "session id" {
		t.Fatalf("unexpected command input hint: %#v", commands[1])
	}
}

func TestAgentModeMapsLegacyDefaultToVibeAsk(t *testing.T) {
	if got := agentMode("default"); got != "ask" {
		t.Fatalf("default mapped to %q", got)
	}
	if got := agentMode("accept-edits"); got != "accept-edits" {
		t.Fatalf("accept-edits mapped to %q", got)
	}
}

func TestPromptBlocksAttachLibraryResources(t *testing.T) {
	blocks := promptBlocks("Prepare a brief", []model.Library{{
		Name: "Product knowledge", Description: "Specs and release notes", Documents: []model.LibraryDocument{
			{Name: "spec.md", Kind: "file", LocalPath: "/tmp/Product docs/spec.md"},
			{Name: "Release notes", Kind: "webpage", Source: "https://example.com/releases"},
		},
	}}, "")
	if len(blocks) != 4 {
		t.Fatalf("expected prompt, Library summary, and two resources; got %#v", blocks)
	}
	if blocks[0].Text == nil || blocks[0].Text.Text != "Prepare a brief" {
		t.Fatalf("unexpected prompt block: %#v", blocks[0])
	}
	if blocks[2].ResourceLink == nil || blocks[2].ResourceLink.Uri != "file:///tmp/Product%20docs/spec.md" {
		t.Fatalf("unexpected local Library resource: %#v", blocks[2])
	}
	if blocks[3].ResourceLink == nil || blocks[3].ResourceLink.Uri != "https://example.com/releases" {
		t.Fatalf("unexpected web Library resource: %#v", blocks[3])
	}
}

func TestPromptBlocksIncludeConnectorContext(t *testing.T) {
	context := "<system-reminder>GitHub connector tools are available.</system-reminder>"
	blocks := promptBlocks("Check my repositories", nil, context)
	if len(blocks) != 2 || blocks[0].Text == nil || blocks[0].Text.Text != "Check my repositories" {
		t.Fatalf("unexpected prompt block: %#v", blocks)
	}
	if blocks[1].Text == nil || blocks[1].Text.Text != context {
		t.Fatalf("connector context was not attached: %#v", blocks[1])
	}
}

func TestUpdateConfigurationWhenIdleRejectsActiveRuntime(t *testing.T) {
	supervisor := NewSupervisor("", Callbacks{})
	supervisor.runtimeMu.RLock()
	called := false
	err := supervisor.UpdateConfigurationWhenIdle(func() error {
		called = true
		return nil
	})
	supervisor.runtimeMu.RUnlock()
	if err == nil || called {
		t.Fatalf("expected active runtime to reject update, called=%v err=%v", called, err)
	}

	if err := supervisor.UpdateConfigurationWhenIdle(func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("expected idle runtime update to run")
	}
}
