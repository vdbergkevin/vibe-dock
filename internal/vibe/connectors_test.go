package vibe

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestUpdateConnectorConfigAddsEnabledConnectorAndPreservesConfig(t *testing.T) {
	input := `# user comment
enable_connectors = false
active_model = "mistral-medium-3.5"

[[mcp_servers]]
name = "docs"
transport = "stdio"
`
	updated, err := updateConnectorConfig(input, "github_app", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"# user comment",
		"enable_connectors = true",
		`active_model = "mistral-medium-3.5"`,
		`name = "docs"`,
		"[[connectors]]\nname = \"github_app\"\ndisabled = false",
	} {
		if !strings.Contains(updated, expected) {
			t.Fatalf("updated config missing %q:\n%s", expected, updated)
		}
	}
}

func TestUpdateConnectorConfigTogglesExistingConnectorWithoutDroppingTools(t *testing.T) {
	input := `[[connectors]]
name = "GitHub App"
disabled = true # selected in Vibe
disabled_tools = ["delete_issue"]

[[connectors]]
name = "linear"
disabled = false
`
	updated, err := updateConnectorConfig(input, "github_app", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updated, "disabled = false # selected in Vibe") {
		t.Fatalf("connector was not enabled in place:\n%s", updated)
	}
	if !strings.Contains(updated, `disabled_tools = ["delete_issue"]`) || !strings.Contains(updated, `name = "linear"`) {
		t.Fatalf("connector settings were lost:\n%s", updated)
	}

	disabled, err := updateConnectorConfig(updated, "github_app", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(disabled, "disabled = true # selected in Vibe") {
		t.Fatalf("connector was not disabled in place:\n%s", disabled)
	}
}

func TestUpdateConnectorConfigSupportsVibeInlineArray(t *testing.T) {
	input := `theme = "auto"
connectors = [
    { name = "atlassian", disabled = true },
]
models = [
    { thinking = "off", alias = "mistral-medium-3.5" },
]
`
	updated, err := updateConnectorConfig(input, "github_app", true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(updated, "[[connectors]]") {
		t.Fatalf("mixed connector formats:\n%s", updated)
	}
	if !strings.Contains(updated, `{ name = "atlassian", disabled = true },`) || !strings.Contains(updated, `{ name = "github_app", disabled = false },`) {
		t.Fatalf("inline connectors were not preserved and extended:\n%s", updated)
	}
	if !strings.Contains(updated, `{ thinking = "off", alias = "mistral-medium-3.5" }`) {
		t.Fatalf("model configuration was changed:\n%s", updated)
	}
	var parsed configFile
	if _, err := toml.Decode(updated, &parsed); err != nil {
		t.Fatalf("updated config is invalid: %v\n%s", err, updated)
	}
	if len(parsed.Connectors) != 2 || parsed.Connectors[1].Name != "github_app" || parsed.Connectors[1].Disabled {
		t.Fatalf("unexpected parsed connectors: %#v", parsed.Connectors)
	}

	disabled, err := updateConnectorConfig(updated, "github_app", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(disabled, `{ name = "github_app", disabled = true },`) {
		t.Fatalf("inline connector was not disabled in place:\n%s", disabled)
	}
}

func TestUpdateConnectorConfigSupportsSingleLineInlineArray(t *testing.T) {
	input := `connectors = [{ name = "github_app", disabled_tools = ["delete_issue"] }]`
	updated, err := updateConnectorConfig(input, "github_app", true)
	if err != nil {
		t.Fatal(err)
	}
	if updated != `connectors = [{ name = "github_app", disabled_tools = ["delete_issue"], disabled = false }]` {
		t.Fatalf("unexpected single-line update: %s", updated)
	}
	var parsed configFile
	if _, err := toml.Decode(updated, &parsed); err != nil {
		t.Fatalf("updated config is invalid: %v", err)
	}
}

func TestSetConnectorEnabledWritesSecureValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".vibe", "config.toml")
	if err := SetConnectorEnabled(path, "github_app", true); err != nil {
		t.Fatal(err)
	}
	config, found, err := readConfig(path)
	if err != nil || !found {
		t.Fatalf("read written config: found=%v err=%v", found, err)
	}
	if len(config.Connectors) != 1 || config.Connectors[0].Name != "github_app" || config.Connectors[0].Disabled {
		t.Fatalf("unexpected written connector config: %+v", config.Connectors)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows does not expose POSIX file permission bits and reports regular
	// writable files as 0666 even after Chmod. Keep the security assertion on
	// platforms where the requested mode can be represented.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("expected private config permissions, got %o", info.Mode().Perm())
	}
}

func TestSetConnectorEnabledRejectsMalformedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	writeTestFile(t, path, "[[connectors]\nname = broken\n")
	if err := SetConnectorEnabled(path, "github_app", true); err == nil {
		t.Fatal("expected malformed config to be rejected")
	}
}
