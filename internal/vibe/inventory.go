package vibe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/vdbergkevin/vibe-dock/internal/model"
)

const connectorCacheTTL = 10 * time.Minute

var invalidSourceName = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

type configFile struct {
	EnableConnectors *bool             `toml:"enable_connectors"`
	Connectors       []connectorConfig `toml:"connectors"`
	MCPServers       []mcpServerConfig `toml:"mcp_servers"`
}

type connectorConfig struct {
	Name          string   `toml:"name"`
	Disabled      bool     `toml:"disabled"`
	DisabledTools []string `toml:"disabled_tools"`
}

type mcpServerConfig struct {
	Name          string   `toml:"name"`
	Transport     string   `toml:"transport"`
	Disabled      bool     `toml:"disabled"`
	DisabledTools []string `toml:"disabled_tools"`
}

type scopedConnector struct {
	connectorConfig
	Scope string
}

type scopedServer struct {
	mcpServerConfig
	Scope string
}

type cacheFile map[string]cacheEntry

type cacheEntry struct {
	StoredAt int64 `json:"stored_at_timestamp"`
	Payload  struct {
		Connectors []cachedConnector `json:"connectors"`
	} `json:"payload"`
}

type cachedConnector struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status struct {
		Ready bool `json:"is_ready"`
	} `json:"status"`
	Tools []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"tools"`
	AuthAction struct {
		Type string `json:"type"`
	} `json:"auth_action"`
	BootstrapErrors json.RawMessage `json:"bootstrap_errors"`
}

// ReadMCPInventory projects the same two source categories that Vibe shows in
// /mcp: configured MCP servers and Mistral-managed workspace connectors. The
// connector cache intentionally contains no access tokens or authentication URLs.
func ReadMCPInventory(vibeHome, projectPath string, now time.Time) model.MCPInventory {
	result := model.MCPInventory{
		Sources:     []model.MCPSource{},
		RefreshedAt: now,
		Errors:      []string{},
	}

	connectors := make(map[string]scopedConnector)
	servers := make(map[string]scopedServer)
	connectorsEnabled := true

	layers := []struct {
		Path  string
		Scope string
	}{
		{Path: filepath.Join(vibeHome, "config.toml"), Scope: "global"},
	}
	if projectPath != "" {
		layers = append(layers, struct {
			Path  string
			Scope string
		}{Path: filepath.Join(projectPath, ".vibe", "config.toml"), Scope: "project"})
	}

	for _, layer := range layers {
		config, found, err := readConfig(layer.Path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Could not read %s Vibe config: %v", layer.Scope, err))
			continue
		}
		if !found {
			continue
		}
		if config.EnableConnectors != nil {
			connectorsEnabled = *config.EnableConnectors
		}
		for _, connector := range config.Connectors {
			name := normalizeName(connector.Name)
			if name != "" {
				connector.Name = name
				connectors[sourceKey(name)] = scopedConnector{connectorConfig: connector, Scope: layer.Scope}
			}
		}
		for _, server := range config.MCPServers {
			name := normalizeName(server.Name)
			if name != "" {
				server.Name = name
				servers[sourceKey(name)] = scopedServer{mcpServerConfig: server, Scope: layer.Scope}
			}
		}
	}

	for _, server := range servers {
		status := "enabled"
		if server.Disabled {
			status = "disabled"
		}
		result.Sources = append(result.Sources, model.MCPSource{
			ID:          "server:" + server.Name,
			Name:        server.Name,
			DisplayName: displayName(server.Name),
			Kind:        "server",
			Transport:   defaultValue(server.Transport, "unknown"),
			Status:      status,
			Enabled:     !server.Disabled,
			Scope:       server.Scope,
			Tools:       []model.MCPTool{},
		})
	}

	entry, found, err := readNewestCache(filepath.Join(vibeHome, "connector_bootstrap_cache.json"))
	if err != nil {
		result.Errors = append(result.Errors, "Could not read Vibe connector cache: "+err.Error())
	} else if found {
		result.CacheAvailable = true
		cacheTime := time.Unix(entry.StoredAt, 0)
		result.CacheUpdatedAt = &cacheTime
		result.CacheStale = now.Sub(cacheTime) > connectorCacheTTL
		for _, connector := range entry.Payload.Connectors {
			name := normalizeName(defaultValue(connector.Name, connector.ID))
			if name == "" {
				continue
			}
			key := sourceKey(name)
			config, configured := connectors[key]
			enabled := connectorsEnabled && configured && !config.Disabled
			status := connectorStatus(connectorsEnabled, configured, config, connector)
			tools := make([]model.MCPTool, 0, len(connector.Tools))
			for _, tool := range connector.Tools {
				if tool.Name == "" {
					continue
				}
				tools = append(tools, model.MCPTool{
					Name:        tool.Name,
					Description: tool.Description,
					Enabled:     connector.Status.Ready && enabled && !matchesAny(tool.Name, config.DisabledTools),
				})
			}
			sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
			scope := "managed"
			if configured {
				scope = config.Scope
			}
			result.Sources = append(result.Sources, model.MCPSource{
				ID:          "connector:" + name,
				Name:        name,
				DisplayName: displayName(name),
				Kind:        "connector",
				Transport:   "connector",
				Status:      status,
				Connected:   connector.Status.Ready,
				Enabled:     enabled,
				Scope:       scope,
				Tools:       tools,
				Error:       cachedError(connector.BootstrapErrors),
			})
			delete(connectors, key)
		}
	}

	// Keep explicitly configured connectors visible even when discovery has not
	// produced a cache entry (for example while offline or before first login).
	for _, connector := range connectors {
		status := "unavailable"
		if !connectorsEnabled || connector.Disabled {
			status = "disabled"
		}
		result.Sources = append(result.Sources, model.MCPSource{
			ID:          "connector:" + connector.Name,
			Name:        connector.Name,
			DisplayName: displayName(connector.Name),
			Kind:        "connector",
			Transport:   "connector",
			Status:      status,
			Enabled:     connectorsEnabled && !connector.Disabled,
			Scope:       connector.Scope,
			Tools:       []model.MCPTool{},
		})
	}

	sort.Slice(result.Sources, func(i, j int) bool {
		if result.Sources[i].Kind != result.Sources[j].Kind {
			return result.Sources[i].Kind < result.Sources[j].Kind
		}
		return strings.ToLower(result.Sources[i].DisplayName) < strings.ToLower(result.Sources[j].DisplayName)
	})
	return result
}

func readConfig(path string) (configFile, bool, error) {
	var config configFile
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return config, false, nil
		}
		return config, false, err
	}
	if _, err := toml.DecodeFile(path, &config); err != nil {
		return config, true, err
	}
	return config, true, nil
}

func readNewestCache(path string) (cacheEntry, bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cacheEntry{}, false, nil
		}
		return cacheEntry{}, false, err
	}
	var entries cacheFile
	if err := json.Unmarshal(contents, &entries); err != nil {
		return cacheEntry{}, false, err
	}
	var newest cacheEntry
	found := false
	for _, entry := range entries {
		if !found || entry.StoredAt > newest.StoredAt {
			newest, found = entry, true
		}
	}
	return newest, found, nil
}

func connectorStatus(masterEnabled, configured bool, config scopedConnector, connector cachedConnector) string {
	if connector.Status.Ready {
		if masterEnabled && configured && !config.Disabled {
			return "connected"
		}
		return "disabled"
	}
	switch connector.AuthAction.Type {
	case "oauth":
		return "needs_auth"
	case "credentials_setup":
		return "needs_setup"
	default:
		return "unavailable"
	}
}

func normalizeName(value string) string {
	value = invalidSourceName.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_-")
	if len(value) > 256 {
		return value[:256]
	}
	return value
}

func sourceKey(value string) string {
	return strings.ToLower(normalizeName(value))
}

func displayName(name string) string {
	known := map[string]string{
		"atlassian":         "Atlassian",
		"bigquery":          "BigQuery",
		"box":               "Box",
		"github_app":        "GitHub",
		"gmail":             "Gmail",
		"google_calendar":   "Google Calendar",
		"google_drive_mcp":  "Google Drive",
		"linear":            "Linear",
		"notion":            "Notion",
		"outlook":           "Outlook Mail",
		"outlook_calendar":  "Outlook Calendar",
		"sharepoint_mcp":    "SharePoint",
		"sharepoint_online": "SharePoint Online",
		"slack":             "Slack",
		"stripe":            "Stripe",
	}
	if value, ok := known[strings.ToLower(name)]; ok {
		return value
	}
	words := strings.Fields(strings.NewReplacer("_", " ", "-", " ").Replace(name))
	for index, word := range words {
		if word != "" {
			words[index] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return strings.Join(words, " ")
}

func matchesAny(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == name {
			return true
		}
		if strings.HasPrefix(pattern, "re:") {
			if expression, err := regexp.Compile(strings.TrimPrefix(pattern, "re:")); err == nil && expression.MatchString(name) {
				return true
			}
			continue
		}
		if matched, err := filepath.Match(pattern, name); err == nil && matched {
			return true
		}
	}
	return false
}

func cachedError(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var message string
	if json.Unmarshal(raw, &message) == nil {
		return message
	}
	var messages []string
	if json.Unmarshal(raw, &messages) == nil {
		return strings.Join(messages, "; ")
	}
	return "Connector discovery reported an error"
}

func defaultValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
