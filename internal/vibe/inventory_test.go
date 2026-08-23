package vibe

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vdbergkevin/vibe-dock/internal/model"
)

func TestReadMCPInventoryProjectsConnectorsAndServers(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	now := time.Unix(1_800_000_000, 0)

	writeTestFile(t, filepath.Join(home, "config.toml"), `
enable_connectors = true

[[connectors]]
name = "GitHub App"
disabled_tools = ["delete_*", "re:^admin_"]

[[mcp_servers]]
name = "docs"
transport = "stdio"
command = "docs-mcp"
`)
	writeTestFile(t, filepath.Join(project, ".vibe", "config.toml"), `
[[mcp_servers]]
name = "project-api"
transport = "streamable-http"
url = "https://example.test/mcp"
disabled = true
`)
	writeTestFile(t, filepath.Join(home, "connector_bootstrap_cache.json"), `{
  "cache-key": {
    "stored_at_timestamp": 1799999700,
    "payload": {
      "connectors": [
        {
          "id": "github-id",
          "name": "GitHub App",
          "status": {"is_ready": true},
          "auth_action": {"type": "oauth"},
          "tools": [
            {"name": "search_repositories", "description": "Search GitHub repositories"},
            {"name": "delete_issue", "description": "Delete an issue"},
            {"name": "admin_users", "description": "Manage users"}
          ]
        },
        {
          "id": "slack-id",
          "name": "Slack",
          "status": {"is_ready": false},
          "auth_action": {"type": "oauth"},
          "tools": []
        }
      ]
    }
  }
}`)

	inventory := ReadMCPInventory(home, project, now)
	if !inventory.CacheAvailable || inventory.CacheStale {
		t.Fatalf("unexpected cache state: available=%v stale=%v", inventory.CacheAvailable, inventory.CacheStale)
	}
	if len(inventory.Errors) != 0 {
		t.Fatalf("unexpected inventory errors: %v", inventory.Errors)
	}
	if len(inventory.Sources) != 4 {
		t.Fatalf("expected four sources, got %d", len(inventory.Sources))
	}

	github := sourceByID(t, inventory.Sources, "connector:GitHub_App")
	if github.DisplayName != "GitHub" || github.Status != "connected" || !github.Connected || !github.Enabled || github.Scope != "global" {
		t.Fatalf("unexpected GitHub projection: %+v", github)
	}
	if !github.Tools[2].Enabled || github.Tools[0].Enabled || github.Tools[1].Enabled {
		t.Fatalf("disabled tool patterns were not applied: %+v", github.Tools)
	}

	slack := sourceByID(t, inventory.Sources, "connector:Slack")
	if slack.Status != "needs_auth" || slack.Connected || slack.Enabled || slack.Scope != "managed" {
		t.Fatalf("unconnected managed connector should retain its discovered auth state: %+v", slack)
	}
	if sourceByID(t, inventory.Sources, "server:docs").Status != "enabled" {
		t.Fatal("global MCP server should be enabled")
	}
	projectServer := sourceByID(t, inventory.Sources, "server:project-api")
	if projectServer.Status != "disabled" || projectServer.Scope != "project" {
		t.Fatalf("unexpected project MCP server: %+v", projectServer)
	}
}

func TestReadMCPInventoryMatchesManagedConnectorNamesCaseInsensitively(t *testing.T) {
	home := t.TempDir()
	now := time.Unix(1_800_000_000, 0)
	writeTestFile(t, filepath.Join(home, "config.toml"), `
[[connectors]]
name = "GitHub App"
disabled_tools = ["delete_issue"]
`)
	writeTestFile(t, filepath.Join(home, "connector_bootstrap_cache.json"), `{
  "current": {
    "stored_at_timestamp": 1799999700,
    "payload": {"connectors": [{
      "id": "github-id",
      "name": "github_app",
      "status": {"is_ready": true},
      "tools": [
        {"name": "search_repositories", "description": "Search repositories"},
        {"name": "delete_issue", "description": "Delete an issue"}
      ]
    }]}
  }
}`)

	inventory := ReadMCPInventory(home, "", now)
	if len(inventory.Sources) != 1 {
		t.Fatalf("expected connector config and cache entry to merge, got %d sources", len(inventory.Sources))
	}
	github := sourceByID(t, inventory.Sources, "connector:github_app")
	if github.DisplayName != "GitHub" || github.Status != "connected" || !github.Connected || !github.Enabled || github.Scope != "global" {
		t.Fatalf("unexpected GitHub projection: %+v", github)
	}
	if !github.Tools[1].Enabled || github.Tools[0].Enabled {
		t.Fatalf("expected the configured disabled tool to be preserved: %+v", github.Tools)
	}
}

func TestReadMCPInventoryShowsReadyManagedConnectorAsConnectedButDisabled(t *testing.T) {
	home := t.TempDir()
	now := time.Unix(1_800_000_000, 0)
	writeTestFile(t, filepath.Join(home, "connector_bootstrap_cache.json"), `{
  "current": {
    "stored_at_timestamp": 1799999700,
    "payload": {"connectors": [{
      "id": "github-id",
      "name": "github_app",
      "status": {"is_ready": true},
      "tools": [{"name": "search_repositories", "description": "Search repositories"}]
    }]}
  }
}`)

	github := sourceByID(t, ReadMCPInventory(home, "", now).Sources, "connector:github_app")
	if github.Status != "disabled" || !github.Connected || github.Enabled || github.Scope != "managed" || github.Tools[0].Enabled {
		t.Fatalf("ready managed connector should be authenticated but disabled until selected in Vibe: %+v", github)
	}
}

func TestReadMCPInventoryReportsStaleCacheAndAuthState(t *testing.T) {
	home := t.TempDir()
	now := time.Unix(1_800_000_000, 0)
	writeTestFile(t, filepath.Join(home, "config.toml"), `
[[connectors]]
name = "linear"
`)
	writeTestFile(t, filepath.Join(home, "connector_bootstrap_cache.json"), `{
  "older": {
    "stored_at_timestamp": 1799990000,
    "payload": {"connectors": [{
      "id": "linear-id",
      "name": "linear",
      "status": {"is_ready": false},
      "auth_action": {"type": "oauth"},
      "tools": []
    }]}
  }
}`)

	inventory := ReadMCPInventory(home, "", now)
	if !inventory.CacheStale {
		t.Fatal("expected old connector cache to be marked stale")
	}
	if got := sourceByID(t, inventory.Sources, "connector:linear").Status; got != "needs_auth" {
		t.Fatalf("expected needs_auth, got %q", got)
	}
}

func sourceByID(t *testing.T, sources []model.MCPSource, id string) model.MCPSource {
	t.Helper()
	for _, source := range sources {
		if source.ID == id {
			return source
		}
	}
	t.Fatalf("source %q not found", id)
	return model.MCPSource{}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
