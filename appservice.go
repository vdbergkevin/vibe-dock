package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/vdbergkevin/vibe-dock/internal/agent"
	"github.com/vdbergkevin/vibe-dock/internal/model"
	"github.com/vdbergkevin/vibe-dock/internal/store"
	vibeconfig "github.com/vdbergkevin/vibe-dock/internal/vibe"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type AppService struct {
	store            *store.Store
	app              *application.App
	supervisor       *agent.Supervisor
	terminalLauncher func(string) error
	editorLauncher   func(string, string) error
	setupLauncher    func(string) error
	libraryRoot      func() (string, error)
	terminals        map[string]*terminalProcess
	terminalsMu      sync.Mutex
	closeOnce        sync.Once
}

func NewAppService(dataStore *store.Store, acpPath string) *AppService {
	s := &AppService{store: dataStore, terminalLauncher: launchProjectTerminal, editorLauncher: launchCodeEditor, setupLauncher: launchVibeSetup, libraryRoot: libraryStorageRoot, terminals: make(map[string]*terminalProcess)}
	s.supervisor = agent.NewSupervisor(acpPath, agent.Callbacks{
		Event: func(event model.StreamEvent) {
			if s.app != nil {
				s.app.Event.Emit("vibe:stream", event)
			}
		},
		AgentSession: func(localSessionID, agentSessionID string) {
			_ = s.store.SetAgentSessionID(context.Background(), localSessionID, agentSessionID)
		},
	})
	return s
}

func (s *AppService) SetApp(app *application.App) { s.app = app }

func (s *AppService) Close() {
	s.closeOnce.Do(func() {
		s.closeTerminals()
		s.supervisor.Close()
		_ = s.store.Close()
	})
}

func (s *AppService) Bootstrap() (model.Bootstrap, error) {
	ctx := context.Background()
	projects, err := s.store.Projects(ctx)
	if err != nil {
		return model.Bootstrap{}, err
	}
	plugins, err := s.store.Plugins(ctx)
	if err != nil {
		return model.Bootstrap{}, err
	}
	libraries, err := s.store.Libraries(ctx)
	if err != nil {
		return model.Bootstrap{}, err
	}
	editors := detectCodeEditors()
	return model.Bootstrap{
		Projects:    projects,
		Plugins:     plugins,
		Libraries:   libraries,
		Editors:     editors,
		Editor:      preferredCodeEditor(s.store.Setting(ctx, "code_editor"), editors),
		LastProject: s.store.Setting(ctx, "last_project"),
		Workspace:   defaultString(s.store.Setting(ctx, "workspace_kind"), "code"),
		Theme:       defaultString(s.store.Setting(ctx, "theme"), "dark"),
		AccentTheme: defaultString(s.store.Setting(ctx, "accent_theme"), "mistral"),
		Environment: detectEnvironment(),
	}, nil
}

func (s *AppService) PickProjectFolder() (string, error) {
	if s.app == nil {
		return "", errors.New("native folder picker is unavailable")
	}
	path, err := s.app.Dialog.OpenFile().
		SetTitle("Add a project").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
	if err != nil {
		return "", nil
	}
	return path, nil
}

func (s *AppService) AddProject(path string) (model.Project, error) {
	info, err := os.Stat(path)
	if err != nil {
		return model.Project{}, fmt.Errorf("open project: %w", err)
	}
	if !info.IsDir() {
		return model.Project{}, errors.New("a project must be a folder")
	}
	project, err := s.store.AddProject(context.Background(), path)
	if err == nil {
		_ = s.store.SetSetting(context.Background(), "last_project", project.ID)
		_ = s.store.SetSetting(context.Background(), "workspace_kind", "code")
	}
	return project, err
}

func (s *AppService) CreateChatProject(name string) (model.Project, error) {
	return s.CreateManagedProject(name, "chat")
}

func (s *AppService) CreateManagedProject(name, kind string) (model.Project, error) {
	return s.CreateManagedProjectWithAppearance(name, kind, "", "")
}

func (s *AppService) CreateManagedProjectWithAppearance(name, kind, icon, color string) (model.Project, error) {
	if kind != "chat" && kind != "work" {
		return model.Project{}, errors.New("unsupported managed project kind")
	}
	root, err := managedWorkspaceRoot(kind)
	if err != nil {
		return model.Project{}, err
	}
	workspace := filepath.Join(root, store.NewID(kind))
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		return model.Project{}, fmt.Errorf("create managed workspace: %w", err)
	}
	project, err := s.store.AddManagedProjectWithAppearance(context.Background(), name, workspace, kind, icon, color)
	if err == nil {
		_ = s.store.SetSetting(context.Background(), "last_project", project.ID)
		_ = s.store.SetSetting(context.Background(), "workspace_kind", kind)
	}
	return project, err
}

func (s *AppService) UpdateProjectAppearance(projectID, icon, color string) (model.Project, error) {
	return s.store.UpdateProjectAppearance(context.Background(), projectID, icon, color)
}

func (s *AppService) RemoveProject(projectID string) error {
	return s.store.RemoveProject(context.Background(), projectID)
}

func (s *AppService) SelectProject(projectID string) error {
	ctx := context.Background()
	project, err := s.store.Project(ctx, projectID)
	if err != nil {
		return err
	}
	if err := s.store.TouchProject(ctx, projectID); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, "last_project", projectID); err != nil {
		return err
	}
	return s.store.SetSetting(ctx, "workspace_kind", project.Kind)
}

func (s *AppService) SetWorkspaceKind(kind string) error {
	if kind != "chat" && kind != "work" && kind != "code" {
		return errors.New("unsupported workspace kind")
	}
	return s.store.SetSetting(context.Background(), "workspace_kind", kind)
}

func (s *AppService) CreateConversation(projectID, title string) (model.Conversation, error) {
	return s.store.CreateConversation(context.Background(), projectID, title)
}

func (s *AppService) DeleteConversation(sessionID string) error {
	ctx := context.Background()
	if _, err := s.store.Conversation(ctx, sessionID); err != nil {
		return err
	}
	if err := s.supervisor.RemoveSession(sessionID); err != nil {
		return err
	}
	return s.store.DeleteConversation(ctx, sessionID)
}

func (s *AppService) Messages(sessionID string) ([]model.Message, error) {
	return s.store.Messages(context.Background(), sessionID)
}

func (s *AppService) SetConversationLibraries(sessionID string, libraryIDs []string) (model.Conversation, error) {
	ctx := context.Background()
	if _, err := s.store.Conversation(ctx, sessionID); err != nil {
		return model.Conversation{}, err
	}
	if err := s.store.SetConversationLibraries(ctx, sessionID, libraryIDs); err != nil {
		return model.Conversation{}, err
	}
	return s.store.Conversation(ctx, sessionID)
}

func (s *AppService) CreateLibrary(name, description string) (model.Library, error) {
	return s.store.CreateLibrary(context.Background(), name, description)
}

func (s *AppService) DeleteLibrary(libraryID string) error {
	if _, err := s.store.Library(context.Background(), libraryID); err != nil {
		return err
	}
	if err := s.store.DeleteLibrary(context.Background(), libraryID); err != nil {
		return err
	}
	root, err := s.libraryRoot()
	if err == nil {
		_ = os.RemoveAll(filepath.Join(root, libraryID))
	}
	return nil
}

func (s *AppService) PickLibraryDocuments() ([]string, error) {
	if s.app == nil {
		return nil, errors.New("native file picker is unavailable")
	}
	paths, err := s.app.Dialog.OpenFile().
		SetTitle("Add documents to Library").
		CanChooseDirectories(false).
		CanChooseFiles(true).
		PromptForMultipleSelection()
	if err != nil {
		return []string{}, nil
	}
	return paths, nil
}

func (s *AppService) AddLibraryDocuments(libraryID string, paths []string) (model.Library, error) {
	if len(paths) == 0 {
		return s.store.Library(context.Background(), libraryID)
	}
	if len(paths) > 100 {
		return model.Library{}, errors.New("a maximum of 100 documents can be added at once")
	}
	if _, err := s.store.Library(context.Background(), libraryID); err != nil {
		return model.Library{}, err
	}
	root, err := s.libraryRoot()
	if err != nil {
		return model.Library{}, err
	}
	for _, sourcePath := range paths {
		sourcePath, err = filepath.Abs(filepath.Clean(strings.TrimSpace(sourcePath)))
		if err != nil {
			return model.Library{}, fmt.Errorf("resolve library document: %w", err)
		}
		info, statErr := os.Stat(sourcePath)
		if statErr != nil {
			return model.Library{}, fmt.Errorf("open library document: %w", statErr)
		}
		if !info.Mode().IsRegular() {
			return model.Library{}, fmt.Errorf("%s is not a regular file", info.Name())
		}
		if info.Size() > 100*1024*1024 {
			return model.Library{}, fmt.Errorf("%s exceeds the 100 MB Library limit", info.Name())
		}
		document := model.LibraryDocument{ID: store.NewID("doc"), LibraryID: libraryID, Name: info.Name(), Kind: "file", Source: sourcePath, Size: info.Size(), Status: "ready", CreatedAt: time.Now().UTC()}
		destinationDir := filepath.Join(root, libraryID, document.ID)
		if err := os.MkdirAll(destinationDir, 0o700); err != nil {
			return model.Library{}, err
		}
		document.LocalPath = filepath.Join(destinationDir, filepath.Base(sourcePath))
		if err := copyDocument(sourcePath, document.LocalPath); err != nil {
			return model.Library{}, err
		}
		if _, err := s.store.AddLibraryDocument(context.Background(), document); err != nil {
			_ = os.Remove(document.LocalPath)
			return model.Library{}, err
		}
	}
	return s.store.Library(context.Background(), libraryID)
}

func (s *AppService) AddLibraryWebpage(libraryID, rawURL string) (model.Library, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return model.Library{}, errors.New("enter a valid http or https webpage URL")
	}
	if _, err := s.store.Library(context.Background(), libraryID); err != nil {
		return model.Library{}, err
	}
	name := parsed.Hostname()
	if base := filepath.Base(strings.TrimSuffix(parsed.Path, "/")); base != "." && base != "/" && base != "" {
		name = base
	}
	_, err = s.store.AddLibraryDocument(context.Background(), model.LibraryDocument{
		LibraryID: libraryID, Name: name, Kind: "webpage", Source: parsed.String(), Status: "ready",
	})
	if err != nil {
		return model.Library{}, err
	}
	return s.store.Library(context.Background(), libraryID)
}

func (s *AppService) DeleteLibraryDocument(libraryID, documentID string) (model.Library, error) {
	library, err := s.store.Library(context.Background(), libraryID)
	if err != nil {
		return model.Library{}, err
	}
	var localPath string
	for _, document := range library.Documents {
		if document.ID == documentID {
			localPath = document.LocalPath
			break
		}
	}
	if err := s.store.DeleteLibraryDocument(context.Background(), documentID); err != nil {
		return model.Library{}, err
	}
	if localPath != "" {
		_ = os.Remove(localPath)
	}
	return s.store.Library(context.Background(), libraryID)
}

func (s *AppService) SetConversationMode(sessionID, mode string) (model.SessionConfiguration, error) {
	configuration := model.SessionConfiguration{Options: []model.SessionConfigOption{}}
	if s.supervisor.Available() {
		conversation, project, plugins, err := s.sessionRuntime(sessionID)
		if err != nil {
			return model.SessionConfiguration{}, err
		}
		vibeMode := mode
		if vibeMode == "default" {
			vibeMode = "ask"
		}
		configuration, err = s.supervisor.SetConfigOption(context.Background(), sessionID, conversation.AgentSessionID, project.Path, conversation.Mode, plugins, "mode", vibeMode)
		if err != nil {
			return model.SessionConfiguration{}, err
		}
	}
	if err := s.store.SetConversationMode(context.Background(), sessionID, mode); err != nil {
		return model.SessionConfiguration{}, err
	}
	return configuration, nil
}

func (s *AppService) SessionConfiguration(sessionID string) (model.SessionConfiguration, error) {
	conversation, project, plugins, err := s.sessionRuntime(sessionID)
	if err != nil {
		return model.SessionConfiguration{}, err
	}
	return s.supervisor.Configuration(context.Background(), sessionID, conversation.AgentSessionID, project.Path, conversation.Mode, plugins)
}

func (s *AppService) SetSessionConfigOption(sessionID, configID, value string) (model.SessionConfiguration, error) {
	conversation, project, plugins, err := s.sessionRuntime(sessionID)
	if err != nil {
		return model.SessionConfiguration{}, err
	}
	configuration, err := s.supervisor.SetConfigOption(context.Background(), sessionID, conversation.AgentSessionID, project.Path, conversation.Mode, plugins, configID, value)
	if err != nil {
		return model.SessionConfiguration{}, err
	}
	if configID == "mode" {
		if err := s.store.SetConversationMode(context.Background(), sessionID, value); err != nil {
			return model.SessionConfiguration{}, err
		}
	}
	return configuration, nil
}

func (s *AppService) SetTheme(theme string) error {
	switch theme {
	case "dark", "light", "system":
		return s.store.SetSetting(context.Background(), "theme", theme)
	default:
		return errors.New("unsupported appearance")
	}
}

func (s *AppService) SetResolvedTheme(theme string) error {
	if s.app == nil {
		return errors.New("native application icon is unavailable")
	}
	switch theme {
	case "dark":
		s.app.SetIcon(darkDockIcon)
	case "light":
		s.app.SetIcon(lightDockIcon)
	default:
		return errors.New("unsupported resolved appearance")
	}
	return nil
}

func (s *AppService) SetAccentTheme(theme string) error {
	switch theme {
	case "mistral", "tide", "grove", "cobalt", "orchid":
		return s.store.SetSetting(context.Background(), "accent_theme", theme)
	default:
		return errors.New("unsupported colour theme")
	}
}

func (s *AppService) SetCodeEditor(editorID string) error {
	if _, ok := codeEditorDefinitionFor(editorID); !ok {
		return errors.New("unsupported code editor")
	}
	return s.store.SetSetting(context.Background(), "code_editor", editorID)
}

func (s *AppService) RefreshEnvironment() model.Environment {
	return detectEnvironment()
}

func (s *AppService) OpenVibeSetup() error {
	path, err := exec.LookPath("vibe")
	if err != nil {
		return errors.New("Mistral Vibe is not installed")
	}
	if s.setupLauncher == nil {
		return errors.New("Vibe setup launcher is unavailable")
	}
	return s.setupLauncher(path)
}

func (s *AppService) SendPrompt(sessionID, text string) (model.Message, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return model.Message{}, errors.New("message cannot be empty")
	}
	ctx := context.Background()
	conversation, err := s.store.Conversation(ctx, sessionID)
	if err != nil {
		return model.Message{}, err
	}
	project, err := s.store.Project(ctx, conversation.ProjectID)
	if err != nil {
		return model.Message{}, err
	}
	plugins, err := s.store.Plugins(ctx)
	if err != nil {
		return model.Message{}, err
	}
	libraries, err := s.store.ConversationLibraries(ctx, sessionID)
	if err != nil {
		return model.Message{}, err
	}
	connectorContext := s.connectorContext(project.Path)
	autoTitle := ""
	if shouldAutoTitle(conversation.Title) {
		autoTitle = conversationSubject(text)
		if err := s.store.SetConversationTitle(ctx, sessionID, autoTitle); err != nil {
			return model.Message{}, err
		}
	}
	userMessage, err := s.store.AddMessage(ctx, model.Message{SessionID: sessionID, Role: "user", Kind: "text", Content: text})
	if err != nil {
		return model.Message{}, err
	}
	if autoTitle != "" && s.app != nil {
		s.app.Event.Emit("vibe:stream", model.StreamEvent{SessionID: sessionID, Type: "conversation-title", Text: autoTitle})
	}
	go func() {
		answer, duration, promptErr := s.supervisor.Prompt(context.Background(), sessionID, conversation.AgentSessionID, project.Path, conversation.Mode, plugins, libraries, connectorContext, text)
		if answer != "" {
			message, saveErr := s.store.AddMessage(context.Background(), model.Message{
				SessionID: sessionID,
				Role:      "assistant",
				Kind:      "text",
				Content:   answer,
				Metadata:  map[string]any{"durationMs": duration.Milliseconds()},
			})
			if saveErr == nil && s.app != nil {
				s.app.Event.Emit("vibe:message-saved", message)
			}
		}
		if promptErr != nil && s.app != nil {
			s.app.Event.Emit("vibe:stream", model.StreamEvent{
				SessionID: sessionID,
				Type:      "error",
				Text:      promptErr.Error(),
				Status:    "failed",
				Data:      map[string]any{"durationMs": duration.Milliseconds()},
			})
		}
	}()
	return userMessage, nil
}

func (s *AppService) connectorContext(projectPath string) string {
	vibeHome, err := vibeHomePath()
	if err != nil {
		return ""
	}
	inventory := vibeconfig.ReadMCPInventory(vibeHome, projectPath, time.Now())
	enabled := make([]string, 0)
	for _, source := range inventory.Sources {
		if source.Kind != "connector" || !source.Connected || !source.Enabled {
			continue
		}
		enabled = append(enabled, fmt.Sprintf("%s via tools prefixed connector_%s_ (%d tools)", source.DisplayName, source.Name, len(source.Tools)))
	}
	if len(enabled) == 0 {
		return ""
	}
	return "<system-reminder>VibeDock has enabled these authenticated Mistral-managed connectors for this ACP agent: " + strings.Join(enabled, "; ") + ". These connector tools are available even though VibeDock is a local desktop client. When asked about access, inspect and use the available connector tools; do not claim that a separate local MCP server is required. Do not quote this reminder to the user.</system-reminder>"
}

func shouldAutoTitle(title string) bool {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "new conversation", "new chat", "new task":
		return true
	default:
		return false
	}
}

func conversationSubject(prompt string) string {
	subject := strings.NewReplacer("\n", " ", "\r", " ", "\t", " ", "`", "", "#", "").Replace(strings.TrimSpace(prompt))
	subject = strings.Join(strings.Fields(subject), " ")
	lower := strings.ToLower(subject)
	for _, prefix := range []string{"can you please ", "could you please ", "would you please ", "i would love to ", "i would like to ", "i'd like to ", "can you ", "could you ", "please ", "help me ", "let's ", "we need to "} {
		if strings.HasPrefix(lower, prefix) {
			subject = strings.TrimSpace(subject[len(prefix):])
			break
		}
	}
	words := strings.Fields(subject)
	if len(words) == 0 {
		return "New conversation"
	}
	truncated := false
	if len(words) > 10 {
		words = words[:10]
		truncated = true
	}
	subject = strings.Trim(strings.Join(words, " "), " .,:;!?—–-")
	runes := []rune(subject)
	if len(runes) > 64 {
		runes = runes[:64]
		subject = strings.TrimSpace(string(runes))
		truncated = true
	}
	if subject == "" {
		return "New conversation"
	}
	runes = []rune(subject)
	runes[0] = unicode.ToUpper(runes[0])
	subject = string(runes)
	if truncated {
		subject = strings.TrimRight(subject, " .,:;!?—–-") + "…"
	}
	return subject
}

func (s *AppService) CancelPrompt(sessionID string) error {
	return s.supervisor.Cancel(context.Background(), sessionID)
}

func (s *AppService) RespondPermission(decision model.PermissionDecision) error {
	return s.supervisor.Decide(decision.RequestID, decision.OptionID)
}

func (s *AppService) SavePlugin(plugin model.Plugin) (model.Plugin, error) {
	if strings.TrimSpace(plugin.Name) == "" {
		return model.Plugin{}, errors.New("plugin name is required")
	}
	if plugin.Transport == "stdio" && strings.TrimSpace(plugin.Command) == "" {
		return model.Plugin{}, errors.New("stdio plugins require a command")
	}
	return s.store.SavePlugin(context.Background(), plugin)
}

func (s *AppService) MCPInventory(projectID string) (model.MCPInventory, error) {
	projectPath := ""
	if projectID != "" {
		project, err := s.store.Project(context.Background(), projectID)
		if err != nil {
			return model.MCPInventory{}, err
		}
		projectPath = project.Path
	}
	vibeHome, err := vibeHomePath()
	if err != nil {
		return model.MCPInventory{}, err
	}
	return vibeconfig.ReadMCPInventory(vibeHome, projectPath, time.Now()), nil
}

func (s *AppService) SetConnectorEnabled(projectID, name string, enabled bool) (model.MCPInventory, error) {
	projectPath := ""
	if projectID != "" {
		project, err := s.store.Project(context.Background(), projectID)
		if err != nil {
			return model.MCPInventory{}, err
		}
		projectPath = project.Path
	}
	vibeHome, err := vibeHomePath()
	if err != nil {
		return model.MCPInventory{}, err
	}

	inventory := vibeconfig.ReadMCPInventory(vibeHome, projectPath, time.Now())
	configPath := filepath.Join(vibeHome, "config.toml")
	for _, source := range inventory.Sources {
		if source.Kind == "connector" && strings.EqualFold(source.Name, name) && source.Scope == "project" && projectPath != "" {
			configPath = filepath.Join(projectPath, ".vibe", "config.toml")
			break
		}
	}
	if err := s.supervisor.UpdateConfigurationWhenIdle(func() error {
		return vibeconfig.SetConnectorEnabled(configPath, name, enabled)
	}); err != nil {
		return model.MCPInventory{}, err
	}
	return vibeconfig.ReadMCPInventory(vibeHome, projectPath, time.Now()), nil
}

func (s *AppService) Skills(projectID string) (model.SkillInventory, error) {
	vibeHome, err := vibeHomePath()
	if err != nil {
		return model.SkillInventory{}, err
	}
	projectName, projectPath, err := s.skillProject(projectID)
	if err != nil {
		return model.SkillInventory{}, err
	}
	return vibeconfig.DiscoverSkills(vibeHome, projectID, projectName, projectPath), nil
}

func (s *AppService) SaveSkill(skill model.Skill) (model.Skill, error) {
	vibeHome, err := vibeHomePath()
	if err != nil {
		return model.Skill{}, err
	}
	_, projectPath, err := s.skillProject(skill.ProjectID)
	if err != nil {
		return model.Skill{}, err
	}
	return vibeconfig.SaveSkill(vibeHome, projectPath, skill)
}

func (s *AppService) SetSkillEnabled(scope, projectID, name string, enabled bool) error {
	vibeHome, err := vibeHomePath()
	if err != nil {
		return err
	}
	_, projectPath, err := s.skillProject(projectID)
	if err != nil {
		return err
	}
	return vibeconfig.SetSkillEnabled(vibeHome, projectPath, scope, name, enabled)
}

func (s *AppService) DeleteSkill(scope, projectID, name string) error {
	vibeHome, err := vibeHomePath()
	if err != nil {
		return err
	}
	_, projectPath, err := s.skillProject(projectID)
	if err != nil {
		return err
	}
	return vibeconfig.DeleteSkill(vibeHome, projectPath, scope, name)
}

func (s *AppService) PickSkillFolder() (string, error) {
	if s.app == nil {
		return "", errors.New("native folder picker is unavailable")
	}
	path, err := s.app.Dialog.OpenFile().
		SetTitle("Import a Vibe Skill").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
	if err != nil {
		return "", nil
	}
	return path, nil
}

func (s *AppService) ImportSkill(sourcePath, scope, projectID string) (model.Skill, error) {
	vibeHome, err := vibeHomePath()
	if err != nil {
		return model.Skill{}, err
	}
	_, projectPath, err := s.skillProject(projectID)
	if err != nil {
		return model.Skill{}, err
	}
	return vibeconfig.ImportSkill(vibeHome, projectPath, sourcePath, scope, projectID)
}

func (s *AppService) ReloadSkills() {
	s.supervisor.Reload()
}

func (s *AppService) skillProject(projectID string) (string, string, error) {
	if strings.TrimSpace(projectID) == "" {
		return "", "", nil
	}
	project, err := s.store.Project(context.Background(), projectID)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(project.Path)
	if err != nil {
		return "", "", fmt.Errorf("open skill project: %w", err)
	}
	if !info.IsDir() {
		return "", "", errors.New("the skill project folder is no longer available")
	}
	return project.Name, project.Path, nil
}

func (s *AppService) sessionRuntime(sessionID string) (model.Conversation, model.Project, []model.Plugin, error) {
	ctx := context.Background()
	conversation, err := s.store.Conversation(ctx, sessionID)
	if err != nil {
		return model.Conversation{}, model.Project{}, nil, err
	}
	project, err := s.store.Project(ctx, conversation.ProjectID)
	if err != nil {
		return model.Conversation{}, model.Project{}, nil, err
	}
	plugins, err := s.store.Plugins(ctx)
	if err != nil {
		return model.Conversation{}, model.Project{}, nil, err
	}
	return conversation, project, plugins, nil
}

func (s *AppService) GitStatus(projectID string) ([]model.ChangedFile, error) {
	project, err := s.store.Project(context.Background(), projectID)
	if err != nil {
		return nil, err
	}
	command := exec.Command("git", "status", "--porcelain=v1")
	command.Dir = project.Path
	output, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && strings.Contains(string(exitErr.Stderr), "not a git repository") {
			return []model.ChangedFile{}, nil
		}
		return nil, fmt.Errorf("git status: %w", err)
	}
	stats := gitNumstat(project.Path)
	files := make([]model.ChangedFile, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if arrow := strings.LastIndex(path, " -> "); arrow >= 0 {
			path = path[arrow+4:]
		}
		status := strings.TrimSpace(line[:2])
		if status == "??" {
			status = "U"
		} else if strings.Contains(status, "A") {
			status = "A"
		} else if strings.Contains(status, "D") {
			status = "D"
		} else {
			status = "M"
		}
		file := model.ChangedFile{Path: path, Status: status}
		if counts, ok := stats[path]; ok {
			file.Additions, file.Deletions = counts[0], counts[1]
		}
		files = append(files, file)
	}
	return files, scanner.Err()
}

func (s *AppService) OpenProjectTerminal(projectID string) error {
	path, err := s.codeProjectPath(projectID)
	if err != nil {
		return err
	}
	if s.terminalLauncher == nil {
		return errors.New("terminal launcher is unavailable")
	}
	return s.terminalLauncher(path)
}

func (s *AppService) OpenProjectInEditor(projectID string) error {
	path, err := s.codeProjectPath(projectID)
	if err != nil {
		return err
	}
	if s.editorLauncher == nil {
		return errors.New("code editor launcher is unavailable")
	}
	editorID := preferredCodeEditor(s.store.Setting(context.Background(), "code_editor"), detectCodeEditors())
	return s.editorLauncher(editorID, path)
}

func (s *AppService) codeProjectPath(projectID string) (string, error) {
	project, err := s.store.Project(context.Background(), projectID)
	if err != nil {
		return "", err
	}
	if project.Kind != "code" {
		return "", errors.New("this action is only available for code projects")
	}
	info, err := os.Stat(project.Path)
	if err != nil {
		return "", fmt.Errorf("open project folder: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("the project folder is no longer available")
	}
	return project.Path, nil
}

func launchProjectTerminal(path string) error {
	switch runtime.GOOS {
	case "darwin":
		output, err := exec.Command("/usr/bin/open", "-a", "Terminal", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("open Terminal: %s", commandError(output, err))
		}
		return nil
	case "windows":
		return startDetachedProcess("wt.exe", []string{"-d", path}, "terminal")
	case "linux":
		candidates := []struct {
			name string
			args []string
		}{
			{"x-terminal-emulator", []string{"--working-directory=" + path}},
			{"gnome-terminal", []string{"--working-directory=" + path}},
			{"konsole", []string{"--workdir", path}},
			{"kitty", []string{"--directory", path}},
			{"wezterm", []string{"start", "--cwd", path}},
			{"alacritty", []string{"--working-directory", path}},
		}
		for _, candidate := range candidates {
			if _, err := exec.LookPath(candidate.name); err == nil {
				return startDetachedProcess(candidate.name, candidate.args, "terminal")
			}
		}
		return errors.New("no supported terminal application was found")
	default:
		return fmt.Errorf("opening a terminal is not supported on %s", runtime.GOOS)
	}
}

type codeEditorDefinition struct {
	ID      string
	Name    string
	Icon    string
	Command string
	MacApp  string
}

var supportedCodeEditors = []codeEditorDefinition{
	{ID: "vscodium", Name: "VSCodium", Icon: "vscodium", Command: "codium", MacApp: "VSCodium"},
	{ID: "vscode", Name: "Visual Studio Code", Icon: "vscode", Command: "code", MacApp: "Visual Studio Code"},
	{ID: "cursor", Name: "Cursor", Icon: "cursor", Command: "cursor", MacApp: "Cursor"},
	{ID: "zed", Name: "Zed", Icon: "zed", Command: "zed", MacApp: "Zed"},
	{ID: "sublime", Name: "Sublime Text", Icon: "sublime", Command: "subl", MacApp: "Sublime Text"},
}

func codeEditorDefinitionFor(editorID string) (codeEditorDefinition, bool) {
	for _, editor := range supportedCodeEditors {
		if editor.ID == editorID {
			return editor, true
		}
	}
	return codeEditorDefinition{}, false
}

func detectCodeEditors() []model.CodeEditor {
	editors := make([]model.CodeEditor, 0, len(supportedCodeEditors))
	for _, editor := range supportedCodeEditors {
		available := false
		if _, err := exec.LookPath(editor.Command); err == nil {
			available = true
		} else if runtime.GOOS == "darwin" {
			available = macApplicationExists(editor.MacApp)
		}
		editors = append(editors, model.CodeEditor{ID: editor.ID, Name: editor.Name, Icon: editor.Icon, Available: available})
	}
	return editors
}

func macApplicationExists(name string) bool {
	home, _ := os.UserHomeDir()
	paths := []string{filepath.Join("/Applications", name+".app"), filepath.Join("/System/Applications", name+".app")}
	if home != "" {
		paths = append(paths, filepath.Join(home, "Applications", name+".app"))
	}
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func preferredCodeEditor(preference string, editors []model.CodeEditor) string {
	if _, ok := codeEditorDefinitionFor(preference); ok {
		return preference
	}
	for _, editor := range editors {
		if editor.Available {
			return editor.ID
		}
	}
	return supportedCodeEditors[0].ID
}

func launchCodeEditor(editorID, path string) error {
	editor, ok := codeEditorDefinitionFor(editorID)
	if !ok {
		return errors.New("unsupported code editor")
	}
	if command, err := exec.LookPath(editor.Command); err == nil {
		return startDetachedProcess(command, []string{path}, editor.Name)
	}
	if runtime.GOOS == "darwin" && macApplicationExists(editor.MacApp) {
		output, err := exec.Command("/usr/bin/open", "-a", editor.MacApp, path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("open %s: %s", editor.Name, commandError(output, err))
		}
		return nil
	}
	return fmt.Errorf("%s was not found; install it or choose another editor in Settings", editor.Name)
}

func launchVibeSetup(vibePath string) error {
	switch runtime.GOOS {
	case "darwin":
		script := `on run argv
tell application "Terminal"
  activate
  do script (quoted form of item 1 of argv) & " --setup"
end tell
end run`
		output, err := exec.Command("/usr/bin/osascript", "-e", script, vibePath).CombinedOutput()
		if err != nil {
			return fmt.Errorf("open Vibe setup: %s", commandError(output, err))
		}
		return nil
	case "windows":
		return startDetachedProcess("wt.exe", []string{vibePath, "--setup"}, "Vibe setup")
	case "linux":
		candidates := []struct {
			name string
			args []string
		}{
			{"x-terminal-emulator", []string{"-e", vibePath, "--setup"}},
			{"gnome-terminal", []string{"--", vibePath, "--setup"}},
			{"konsole", []string{"-e", vibePath, "--setup"}},
			{"kitty", []string{vibePath, "--setup"}},
		}
		for _, candidate := range candidates {
			if _, err := exec.LookPath(candidate.name); err == nil {
				return startDetachedProcess(candidate.name, candidate.args, "Vibe setup")
			}
		}
		return errors.New("no supported terminal application was found")
	default:
		return fmt.Errorf("Vibe setup is not supported on %s", runtime.GOOS)
	}
}

func startDetachedProcess(name string, args []string, label string) error {
	command := exec.Command(name, args...)
	if err := command.Start(); err != nil {
		return fmt.Errorf("open %s: %w", label, err)
	}
	if err := command.Process.Release(); err != nil {
		return fmt.Errorf("detach %s: %w", label, err)
	}
	return nil
}

func commandError(output []byte, err error) string {
	message := strings.TrimSpace(string(output))
	if message != "" {
		return message
	}
	return err.Error()
}

func gitNumstat(projectPath string) map[string][2]int {
	result := make(map[string][2]int)
	command := exec.Command("git", "diff", "--numstat", "HEAD")
	command.Dir = projectPath
	output, err := command.Output()
	if err != nil {
		return result
	}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), "\t", 3)
		if len(parts) != 3 {
			continue
		}
		added, _ := strconv.Atoi(parts[0])
		deleted, _ := strconv.Atoi(parts[1])
		result[parts[2]] = [2]int{added, deleted}
	}
	return result
}

func detectEnvironment() model.Environment {
	env := model.Environment{Platform: runtime.GOOS}
	if path, err := exec.LookPath("vibe"); err == nil {
		env.VibeAvailable = true
		command := exec.Command(path, "--version")
		if output, commandErr := command.Output(); commandErr == nil {
			env.VibeVersion = strings.TrimSpace(string(output))
		}
	}
	if path, err := exec.LookPath("vibe-acp"); err == nil {
		env.ACPAvailable, env.ACPPath = true, path
	}
	_, env.GitAvailable = lookPath("git")
	env.Account = detectAccountStatus(env.VibeAvailable)
	return env
}

func detectAccountStatus(vibeAvailable bool) model.AccountStatus {
	status := model.AccountStatus{Available: vibeAvailable, Detail: "Sign in with Mistral or configure an API key"}
	if !vibeAvailable {
		status.Detail = "Install Mistral Vibe before signing in"
		return status
	}
	if strings.TrimSpace(os.Getenv("MISTRAL_API_KEY")) != "" {
		status.Configured, status.Source, status.Detail = true, "environment", "Mistral credential found in your environment"
		return status
	}
	if runtime.GOOS == "darwin" {
		for _, service := range []string{"ai.mistral.vibe", "vibe"} {
			command := exec.Command("/usr/bin/security", "find-generic-password", "-s", service, "-a", "MISTRAL_API_KEY")
			if command.Run() == nil {
				status.Configured, status.Source, status.Detail = true, "keychain", "Mistral credential secured in macOS Keychain"
				return status
			}
		}
	}
	return status
}

func lookPath(command string) (string, bool) {
	path, err := exec.LookPath(command)
	return path, err == nil
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func appDataPath() (string, error) {
	dir, err := appSupportRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vibe.db"), nil
}

func chatWorkspaceRoot() (string, error) {
	return managedWorkspaceRoot("chat")
}

func managedWorkspaceRoot(kind string) (string, error) {
	appRoot, err := appSupportRoot()
	if err != nil {
		return "", err
	}
	root := filepath.Join(appRoot, strings.ToUpper(kind[:1])+kind[1:]+" Workspaces")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	return root, nil
}

func libraryStorageRoot() (string, error) {
	appRoot, err := appSupportRoot()
	if err != nil {
		return "", err
	}
	root := filepath.Join(appRoot, "Libraries")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	return root, nil
}

func appSupportRoot() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(configDir, "VibeDock")
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		legacyRoot := filepath.Join(configDir, "Vibe Desktop")
		if info, legacyErr := os.Stat(legacyRoot); legacyErr == nil && info.IsDir() {
			if renameErr := os.Rename(legacyRoot, root); renameErr != nil {
				return "", fmt.Errorf("migrate VibeDock app data: %w", renameErr)
			}
		}
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	return root, nil
}

func copyDocument(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open library document: %w", err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create library copy: %w", err)
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return fmt.Errorf("copy library document: %w", err)
	}
	return output.Close()
}

func vibeHomePath() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("VIBE_HOME")); configured != "" {
		return filepath.Clean(configured), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find Vibe home: %w", err)
	}
	return filepath.Join(home, ".vibe"), nil
}
