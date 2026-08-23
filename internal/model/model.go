package model

import "time"

type Project struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Path       string         `json:"path"`
	Kind       string         `json:"kind"`
	Color      string         `json:"color"`
	Icon       string         `json:"icon,omitempty"`
	Pinned     bool           `json:"pinned"`
	LastOpened time.Time      `json:"lastOpened"`
	CreatedAt  time.Time      `json:"createdAt"`
	Sessions   []Conversation `json:"sessions"`
}

type Conversation struct {
	ID             string    `json:"id"`
	ProjectID      string    `json:"projectId"`
	Title          string    `json:"title"`
	Mode           string    `json:"mode"`
	AgentSessionID string    `json:"agentSessionId,omitempty"`
	Preview        string    `json:"preview"`
	UpdatedAt      time.Time `json:"updatedAt"`
	CreatedAt      time.Time `json:"createdAt"`
	LibraryIDs     []string  `json:"libraryIds"`
}

type Library struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Color       string            `json:"color"`
	RemoteID    string            `json:"remoteId,omitempty"`
	Documents   []LibraryDocument `json:"documents"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

type LibraryDocument struct {
	ID        string    `json:"id"`
	LibraryID string    `json:"libraryId"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Source    string    `json:"source"`
	LocalPath string    `json:"localPath,omitempty"`
	Size      int64     `json:"size"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type Message struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionId"`
	Role      string         `json:"role"`
	Kind      string         `json:"kind"`
	Content   string         `json:"content"`
	Status    string         `json:"status"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

type Plugin struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Transport   string            `json:"transport"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	Enabled     bool              `json:"enabled"`
	Scope       string            `json:"scope"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

type MCPTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type MCPSource struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Kind        string    `json:"kind"`
	Transport   string    `json:"transport"`
	Status      string    `json:"status"`
	Connected   bool      `json:"connected"`
	Enabled     bool      `json:"enabled"`
	Scope       string    `json:"scope"`
	Tools       []MCPTool `json:"tools"`
	Error       string    `json:"error,omitempty"`
}

type MCPInventory struct {
	Sources        []MCPSource `json:"sources"`
	RefreshedAt    time.Time   `json:"refreshedAt"`
	CacheUpdatedAt *time.Time  `json:"cacheUpdatedAt,omitempty"`
	CacheAvailable bool        `json:"cacheAvailable"`
	CacheStale     bool        `json:"cacheStale"`
	Errors         []string    `json:"errors"`
}

type Skill struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	OriginalName  string    `json:"originalName,omitempty"`
	Description   string    `json:"description"`
	Instructions  string    `json:"instructions"`
	Scope         string    `json:"scope"`
	Source        string    `json:"source"`
	ProjectID     string    `json:"projectId,omitempty"`
	Path          string    `json:"path"`
	UserInvocable bool      `json:"userInvocable"`
	AllowedTools  []string  `json:"allowedTools"`
	Enabled       bool      `json:"enabled"`
	Editable      bool      `json:"editable"`
	Risk          string    `json:"risk"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type SkillInventory struct {
	Skills      []Skill  `json:"skills"`
	ProjectID   string   `json:"projectId,omitempty"`
	ProjectName string   `json:"projectName,omitempty"`
	GlobalPath  string   `json:"globalPath"`
	ProjectPath string   `json:"projectPath,omitempty"`
	Errors      []string `json:"errors"`
}

type SessionConfigChoice struct {
	Value       string `json:"value"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type SessionConfigOption struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Category     string                `json:"category"`
	CurrentValue string                `json:"currentValue"`
	Options      []SessionConfigChoice `json:"options"`
}

type SlashCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputHint   string `json:"inputHint,omitempty"`
	Source      string `json:"source"`
}

type SessionConfiguration struct {
	Options  []SessionConfigOption `json:"options"`
	Commands []SlashCommand        `json:"commands"`
}

type AccountStatus struct {
	Available  bool   `json:"available"`
	Configured bool   `json:"configured"`
	Source     string `json:"source,omitempty"`
	Detail     string `json:"detail"`
}

type Environment struct {
	VibeAvailable bool          `json:"vibeAvailable"`
	VibeVersion   string        `json:"vibeVersion"`
	ACPAvailable  bool          `json:"acpAvailable"`
	ACPPath       string        `json:"acpPath"`
	GitAvailable  bool          `json:"gitAvailable"`
	Platform      string        `json:"platform"`
	Account       AccountStatus `json:"account"`
}

type CodeEditor struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	Available bool   `json:"available"`
}

type Bootstrap struct {
	Projects    []Project    `json:"projects"`
	Plugins     []Plugin     `json:"plugins"`
	Libraries   []Library    `json:"libraries"`
	Environment Environment  `json:"environment"`
	Editors     []CodeEditor `json:"editors"`
	Editor      string       `json:"editor"`
	LastProject string       `json:"lastProject"`
	Workspace   string       `json:"workspace"`
	Theme       string       `json:"theme"`
	AccentTheme string       `json:"accentTheme"`
}

type StreamEvent struct {
	SessionID string         `json:"sessionId"`
	MessageID string         `json:"messageId,omitempty"`
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Status    string         `json:"status,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

type PermissionDecision struct {
	RequestID string `json:"requestId"`
	OptionID  string `json:"optionId"`
}

type ChangedFile struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}
