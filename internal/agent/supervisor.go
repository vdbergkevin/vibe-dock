package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/vdbergkevin/vibe-dock/internal/model"
	"github.com/vdbergkevin/vibe-dock/internal/store"
)

type Callbacks struct {
	Event        func(model.StreamEvent)
	AgentSession func(localSessionID, agentSessionID string)
}

type Supervisor struct {
	path      string
	callbacks Callbacks
	runtimeMu sync.RWMutex
	mu        sync.Mutex
	workers   map[string]*worker
	pending   map[string]chan string
}

type worker struct {
	localSessionID string
	root           string
	mode           string
	cmd            *exec.Cmd
	conn           *acp.ClientSideConnection
	agentSessionID string
	promptMu       sync.Mutex
	responseMu     sync.Mutex
	response       strings.Builder
	configMu       sync.RWMutex
	configOptions  []model.SessionConfigOption
	commandsMu     sync.RWMutex
	commands       []model.SlashCommand
	client         *client
}

type client struct {
	supervisor *Supervisor
	worker     *worker
}

func NewSupervisor(path string, callbacks Callbacks) *Supervisor {
	return &Supervisor{path: path, callbacks: callbacks, workers: make(map[string]*worker), pending: make(map[string]chan string)}
}

func (s *Supervisor) Available() bool { return s.path != "" }

func (s *Supervisor) Prompt(ctx context.Context, localSessionID, savedAgentSessionID, root, mode string, plugins []model.Plugin, libraries []model.Library, connectorContext, text string) (string, time.Duration, error) {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()

	startedAt := time.Now()
	if s.path == "" {
		return "", time.Since(startedAt), errors.New("vibe-acp was not found; install or configure Mistral Vibe first")
	}
	w, err := s.worker(ctx, localSessionID, savedAgentSessionID, root, mode, plugins)
	if err != nil {
		return "", time.Since(startedAt), err
	}
	w.promptMu.Lock()
	defer w.promptMu.Unlock()
	w.responseMu.Lock()
	w.response.Reset()
	w.responseMu.Unlock()

	if err := syncWorkerMode(ctx, w, mode); err != nil {
		return "", time.Since(startedAt), err
	}
	response, err := w.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: acp.SessionId(w.agentSessionID),
		Prompt:    promptBlocks(text, libraries, connectorContext),
	})
	w.responseMu.Lock()
	answer := w.response.String()
	w.responseMu.Unlock()
	if err != nil {
		return answer, time.Since(startedAt), fmt.Errorf("vibe prompt: %w", err)
	}
	duration := time.Since(startedAt)
	s.emit(model.StreamEvent{
		SessionID: localSessionID,
		Type:      "complete",
		Status:    string(response.StopReason),
		Data:      map[string]any{"durationMs": duration.Milliseconds()},
	})
	return answer, duration, nil
}

func promptBlocks(text string, libraries []model.Library, connectorContext string) []acp.ContentBlock {
	blocks := []acp.ContentBlock{acp.TextBlock(text)}
	if connectorContext != "" {
		blocks = append(blocks, acp.TextBlock(connectorContext))
	}
	for _, library := range libraries {
		summary := fmt.Sprintf("Reusable Library attached: %s", library.Name)
		if library.Description != "" {
			summary += " — " + library.Description
		}
		blocks = append(blocks, acp.TextBlock(summary))
		for _, document := range library.Documents {
			uri := document.Source
			if document.LocalPath != "" {
				uri = (&url.URL{Scheme: "file", Path: document.LocalPath}).String()
			}
			if uri == "" {
				continue
			}
			blocks = append(blocks, acp.ResourceLinkBlock(library.Name+" / "+document.Name, uri))
		}
	}
	return blocks
}

func (s *Supervisor) worker(ctx context.Context, localSessionID, savedAgentSessionID, root, mode string, plugins []model.Plugin) (*worker, error) {
	s.mu.Lock()
	if existing := s.workers[localSessionID]; existing != nil {
		select {
		case <-existing.conn.Done():
			delete(s.workers, localSessionID)
		default:
			existing.mode = mode
			s.mu.Unlock()
			return existing, nil
		}
	}
	s.mu.Unlock()

	cmd := exec.Command(s.path)
	cmd.Dir = root
	cmd.Env = os.Environ()
	cmd.Stderr = io.Discard
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open vibe-acp stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open vibe-acp stdout: %w", err)
	}
	w := &worker{localSessionID: localSessionID, root: root, mode: mode, cmd: cmd}
	w.client = &client{supervisor: s, worker: w}
	w.conn = acp.NewClientSideConnection(w.client, stdin, stdout)
	w.conn.SetLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start vibe-acp: %w", err)
	}
	initResponse, err := w.conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientInfo:      &acp.Implementation{Name: "vibedock", Title: stringPtr("VibeDock"), Version: "0.1.0"},
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: true, WriteTextFile: true},
		},
	})
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("initialize vibe-acp: %w", err)
	}
	mcpServers := convertPlugins(plugins)
	if savedAgentSessionID != "" && initResponse.AgentCapabilities.LoadSession {
		var loaded acp.LoadSessionResponse
		loaded, err = w.conn.LoadSession(ctx, acp.LoadSessionRequest{SessionId: acp.SessionId(savedAgentSessionID), Cwd: root, McpServers: mcpServers})
		if err == nil {
			w.agentSessionID = savedAgentSessionID
			w.setConfigOptions(projectConfigOptions(loaded.ConfigOptions))
		}
	}
	if w.agentSessionID == "" {
		created, createErr := w.conn.NewSession(ctx, acp.NewSessionRequest{Cwd: root, McpServers: mcpServers})
		if createErr != nil {
			_ = cmd.Process.Kill()
			return nil, fmt.Errorf("create vibe session: %w", createErr)
		}
		w.agentSessionID = string(created.SessionId)
		w.setConfigOptions(projectConfigOptions(created.ConfigOptions))
		if s.callbacks.AgentSession != nil {
			s.callbacks.AgentSession(localSessionID, w.agentSessionID)
		}
	}
	if err := syncWorkerMode(ctx, w, mode); err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	s.mu.Lock()
	if raced := s.workers[localSessionID]; raced != nil {
		s.mu.Unlock()
		_ = cmd.Process.Kill()
		return raced, nil
	}
	s.workers[localSessionID] = w
	s.mu.Unlock()
	return w, nil
}

func (s *Supervisor) Cancel(ctx context.Context, localSessionID string) error {
	s.mu.Lock()
	w := s.workers[localSessionID]
	s.mu.Unlock()
	if w == nil {
		return nil
	}
	return w.conn.Cancel(ctx, acp.CancelNotification{SessionId: acp.SessionId(w.agentSessionID)})
}

// RemoveSession retires an idle ACP process before its local conversation is
// deleted. A prompt owns promptMu for its full lifetime, so TryLock keeps a
// running response from being orphaned by a deletion.
func (s *Supervisor) RemoveSession(localSessionID string) error {
	s.mu.Lock()
	w := s.workers[localSessionID]
	if w == nil {
		s.mu.Unlock()
		return nil
	}
	if !w.promptMu.TryLock() {
		s.mu.Unlock()
		return errors.New("stop Vibe before removing this conversation")
	}
	delete(s.workers, localSessionID)
	s.mu.Unlock()
	defer w.promptMu.Unlock()
	if w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
	}
	return nil
}

func (s *Supervisor) Configuration(ctx context.Context, localSessionID, savedAgentSessionID, root, mode string, plugins []model.Plugin) (model.SessionConfiguration, error) {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()

	w, err := s.worker(ctx, localSessionID, savedAgentSessionID, root, mode, plugins)
	if err != nil {
		return model.SessionConfiguration{}, err
	}
	return model.SessionConfiguration{Options: w.getConfigOptions(), Commands: w.getCommands()}, nil
}

func (s *Supervisor) SetConfigOption(ctx context.Context, localSessionID, savedAgentSessionID, root, mode string, plugins []model.Plugin, configID, value string) (model.SessionConfiguration, error) {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()

	w, err := s.worker(ctx, localSessionID, savedAgentSessionID, root, mode, plugins)
	if err != nil {
		return model.SessionConfiguration{}, err
	}
	w.promptMu.Lock()
	defer w.promptMu.Unlock()
	options, err := setWorkerConfigOption(ctx, w, configID, value)
	if err != nil {
		return model.SessionConfiguration{}, err
	}
	if configID == "mode" {
		w.mode = value
	}
	return model.SessionConfiguration{Options: options, Commands: w.getCommands()}, nil
}

func (s *Supervisor) Decide(requestID, optionID string) error {
	s.mu.Lock()
	ch := s.pending[requestID]
	s.mu.Unlock()
	if ch == nil {
		return errors.New("permission request is no longer active")
	}
	select {
	case ch <- optionID:
		return nil
	default:
		return errors.New("permission request has already been answered")
	}
}

func (s *Supervisor) Close() {
	// Closing the application must be able to terminate an active prompt rather
	// than waiting for it to finish naturally.
	s.stopWorkers()
}

// Reload drops live ACP processes so the next request creates a fresh Vibe
// session runtime and rediscovers configuration, instructions, and skills.
func (s *Supervisor) Reload() {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	s.stopWorkers()
}

// UpdateConfigurationWhenIdle serialises a configuration mutation with all
// ACP requests, then retires the old processes. This prevents a prompt from
// starting with a half-written configuration or losing a response to reload.
func (s *Supervisor) UpdateConfigurationWhenIdle(update func() error) error {
	if !s.runtimeMu.TryLock() {
		return errors.New("wait for Vibe to finish before changing connector access")
	}
	defer s.runtimeMu.Unlock()
	if err := update(); err != nil {
		return err
	}
	s.stopWorkers()
	return nil
}

func (s *Supervisor) stopWorkers() {
	s.mu.Lock()
	workers := make([]*worker, 0, len(s.workers))
	for _, w := range s.workers {
		workers = append(workers, w)
	}
	s.workers = make(map[string]*worker)
	s.mu.Unlock()
	for _, w := range workers {
		if w.cmd.Process != nil {
			_ = w.cmd.Process.Kill()
		}
	}
}

func (s *Supervisor) emit(event model.StreamEvent) {
	if s.callbacks.Event != nil {
		s.callbacks.Event(event)
	}
}

func (c *client) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	u := params.Update
	w := c.worker
	event := model.StreamEvent{SessionID: w.localSessionID}
	switch {
	case u.AgentMessageChunk != nil && u.AgentMessageChunk.Content.Text != nil:
		event.Type = "text"
		event.Text = u.AgentMessageChunk.Content.Text.Text
		if u.AgentMessageChunk.MessageId != nil {
			event.MessageID = *u.AgentMessageChunk.MessageId
		}
		w.responseMu.Lock()
		w.response.WriteString(event.Text)
		w.responseMu.Unlock()
	case u.AgentThoughtChunk != nil && u.AgentThoughtChunk.Content.Text != nil:
		event.Type = "thought"
		event.Text = u.AgentThoughtChunk.Content.Text.Text
		if u.AgentThoughtChunk.MessageId != nil {
			event.MessageID = *u.AgentThoughtChunk.MessageId
		}
	case u.ToolCall != nil:
		event.Type = "tool-call"
		event.Status = string(u.ToolCall.Status)
		event.Data = map[string]any{"id": string(u.ToolCall.ToolCallId), "title": u.ToolCall.Title, "kind": string(u.ToolCall.Kind), "locations": u.ToolCall.Locations, "input": u.ToolCall.RawInput}
	case u.ToolCallUpdate != nil:
		event.Type = "tool-update"
		event.Data = map[string]any{"id": string(u.ToolCallUpdate.ToolCallId), "locations": u.ToolCallUpdate.Locations, "output": u.ToolCallUpdate.RawOutput}
		if u.ToolCallUpdate.Status != nil {
			event.Status = string(*u.ToolCallUpdate.Status)
		}
		if u.ToolCallUpdate.Title != nil {
			event.Data["title"] = *u.ToolCallUpdate.Title
		}
	case u.Plan != nil:
		event.Type = "plan"
		entries := make([]map[string]string, 0, len(u.Plan.Entries))
		for _, item := range u.Plan.Entries {
			entries = append(entries, map[string]string{"content": item.Content, "status": string(item.Status), "priority": string(item.Priority)})
		}
		event.Data = map[string]any{"entries": entries}
	case u.UsageUpdate != nil:
		event.Type = "usage"
		event.Data = map[string]any{"used": u.UsageUpdate.Used, "size": u.UsageUpdate.Size, "cost": u.UsageUpdate.Cost}
	case u.CurrentModeUpdate != nil:
		event.Type = "mode"
		w.mode = string(u.CurrentModeUpdate.CurrentModeId)
		event.Data = map[string]any{"mode": w.mode}
	case u.ConfigOptionUpdate != nil:
		options := projectConfigOptions(u.ConfigOptionUpdate.ConfigOptions)
		w.setConfigOptions(options)
		event.Type = "config-options"
		event.Data = map[string]any{"options": options}
	case u.AvailableCommandsUpdate != nil:
		commands := projectCommands(u.AvailableCommandsUpdate.AvailableCommands)
		w.setCommands(commands)
		event.Type = "commands"
		event.Data = map[string]any{"commands": commands}
	default:
		return nil
	}
	c.supervisor.emit(event)
	return nil
}

func (c *client) RequestPermission(ctx context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	if option := automaticPermission(c.worker.mode, params); option != "" {
		return selected(option), nil
	}
	requestID := store.NewID("perm")
	ch := make(chan string, 1)
	c.supervisor.mu.Lock()
	c.supervisor.pending[requestID] = ch
	c.supervisor.mu.Unlock()
	defer func() { c.supervisor.mu.Lock(); delete(c.supervisor.pending, requestID); c.supervisor.mu.Unlock() }()
	options := make([]map[string]string, 0, len(params.Options))
	for _, option := range params.Options {
		options = append(options, map[string]string{"id": string(option.OptionId), "name": option.Name, "kind": string(option.Kind)})
	}
	title, kind := "Run tool", "other"
	if params.ToolCall.Title != nil {
		title = *params.ToolCall.Title
	}
	if params.ToolCall.Kind != nil {
		kind = string(*params.ToolCall.Kind)
	}
	c.supervisor.emit(model.StreamEvent{SessionID: c.worker.localSessionID, Type: "permission", Data: map[string]any{
		"requestId": requestID, "title": title, "kind": kind, "options": options, "input": params.ToolCall.RawInput,
	}})
	select {
	case optionID := <-ch:
		return selected(acp.PermissionOptionId(optionID)), nil
	case <-ctx.Done():
		return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{Cancelled: &acp.RequestPermissionOutcomeCancelled{}}}, ctx.Err()
	}
}

func selected(optionID acp.PermissionOptionId) acp.RequestPermissionResponse {
	return acp.RequestPermissionResponse{Outcome: acp.RequestPermissionOutcome{Selected: &acp.RequestPermissionOutcomeSelected{OptionId: optionID}}}
}

func automaticPermission(mode string, request acp.RequestPermissionRequest) acp.PermissionOptionId {
	allow := mode == "auto-approve"
	if mode == "accept-edits" && request.ToolCall.Kind != nil {
		switch *request.ToolCall.Kind {
		case acp.ToolKindRead, acp.ToolKindEdit, acp.ToolKindDelete, acp.ToolKindMove, acp.ToolKindSearch:
			allow = true
		}
	}
	if !allow {
		return ""
	}
	for _, preferred := range []acp.PermissionOptionKind{acp.PermissionOptionKindAllowAlways, acp.PermissionOptionKindAllowOnce} {
		for _, option := range request.Options {
			if option.Kind == preferred {
				return option.OptionId
			}
		}
	}
	return ""
}

func (c *client) ReadTextFile(_ context.Context, params acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	path, err := safePath(c.worker.root, params.Path, false)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return acp.ReadTextFileResponse{}, err
	}
	content := string(data)
	if params.Line != nil || params.Limit != nil {
		lines := strings.Split(content, "\n")
		start := 0
		if params.Line != nil && *params.Line > 0 {
			start = min(*params.Line-1, len(lines))
		}
		end := len(lines)
		if params.Limit != nil && *params.Limit > 0 {
			end = min(start+*params.Limit, end)
		}
		content = strings.Join(lines[start:end], "\n")
	}
	return acp.ReadTextFileResponse{Content: content}, nil
}

func (c *client) WriteTextFile(_ context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	path, err := safePath(c.worker.root, params.Path, true)
	if err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	if err := os.WriteFile(path, []byte(params.Content), 0o644); err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	return acp.WriteTextFileResponse{}, nil
}

func (c *client) CreateTerminal(context.Context, acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, errors.New("terminal delegation is not enabled")
}
func (c *client) KillTerminal(context.Context, acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, errors.New("terminal delegation is not enabled")
}
func (c *client) TerminalOutput(context.Context, acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, errors.New("terminal delegation is not enabled")
}
func (c *client) ReleaseTerminal(context.Context, acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, errors.New("terminal delegation is not enabled")
}
func (c *client) WaitForTerminalExit(context.Context, acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, errors.New("terminal delegation is not enabled")
}

func safePath(root, requested string, forWrite bool) (string, error) {
	if !filepath.IsAbs(requested) {
		requested = filepath.Join(root, requested)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	check := requested
	if forWrite {
		check = filepath.Dir(requested)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(check); resolveErr == nil {
		if forWrite {
			requested = filepath.Join(resolved, filepath.Base(requested))
		} else {
			requested = resolved
		}
	}
	requested, err = filepath.Abs(filepath.Clean(requested))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, requested)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("ACP file operation is outside the active project")
	}
	return requested, nil
}

func convertPlugins(plugins []model.Plugin) []acp.McpServer {
	servers := make([]acp.McpServer, 0)
	for _, plugin := range plugins {
		if !plugin.Enabled || plugin.Transport != "stdio" || plugin.Command == "" {
			continue
		}
		env := make([]acp.EnvVariable, 0, len(plugin.Env))
		for name, source := range plugin.Env {
			source = strings.Trim(strings.TrimSpace(source), "${}")
			if source == "" {
				source = name
			}
			value, available := os.LookupEnv(source)
			if !available {
				continue
			}
			env = append(env, acp.EnvVariable{Name: name, Value: value})
		}
		servers = append(servers, acp.McpServer{Stdio: &acp.McpServerStdio{Name: plugin.Name, Command: plugin.Command, Args: plugin.Args, Env: env}})
	}
	return servers
}

func agentMode(mode string) string {
	if mode == "default" {
		return "ask"
	}
	return strings.TrimSpace(mode)
}

func projectConfigOptions(options []acp.SessionConfigOption) []model.SessionConfigOption {
	result := make([]model.SessionConfigOption, 0, len(options))
	for _, option := range options {
		if option.Select == nil {
			continue
		}
		selected := option.Select
		projected := model.SessionConfigOption{
			ID:           string(selected.Id),
			Name:         selected.Name,
			CurrentValue: string(selected.CurrentValue),
			Options:      []model.SessionConfigChoice{},
		}
		if selected.Category != nil {
			projected.Category = string(*selected.Category)
		}
		appendChoice := func(choice acp.SessionConfigSelectOption) {
			description := ""
			if choice.Description != nil {
				description = *choice.Description
			}
			projected.Options = append(projected.Options, model.SessionConfigChoice{
				Value:       string(choice.Value),
				Name:        choice.Name,
				Description: description,
			})
		}
		if selected.Options.Ungrouped != nil {
			for _, choice := range *selected.Options.Ungrouped {
				appendChoice(choice)
			}
		}
		if selected.Options.Grouped != nil {
			for _, group := range *selected.Options.Grouped {
				for _, choice := range group.Options {
					appendChoice(choice)
				}
			}
		}
		result = append(result, projected)
	}
	return result
}

func projectCommands(commands []acp.AvailableCommand) []model.SlashCommand {
	result := make([]model.SlashCommand, 0, len(commands))
	for _, command := range commands {
		inputHint := ""
		if command.Input != nil && command.Input.Unstructured != nil {
			inputHint = command.Input.Unstructured.Hint
		}
		result = append(result, model.SlashCommand{
			Name: strings.TrimPrefix(strings.TrimSpace(command.Name), "/"), Description: command.Description, InputHint: inputHint, Source: "vibe",
		})
	}
	return result
}

func (w *worker) setConfigOptions(options []model.SessionConfigOption) {
	w.configMu.Lock()
	w.configOptions = append([]model.SessionConfigOption(nil), options...)
	w.configMu.Unlock()
}

func (w *worker) getConfigOptions() []model.SessionConfigOption {
	w.configMu.RLock()
	defer w.configMu.RUnlock()
	return append([]model.SessionConfigOption(nil), w.configOptions...)
}

func (w *worker) setCommands(commands []model.SlashCommand) {
	w.commandsMu.Lock()
	w.commands = append([]model.SlashCommand(nil), commands...)
	w.commandsMu.Unlock()
}

func (w *worker) getCommands() []model.SlashCommand {
	w.commandsMu.RLock()
	defer w.commandsMu.RUnlock()
	return append([]model.SlashCommand(nil), w.commands...)
}

func setWorkerConfigOption(ctx context.Context, w *worker, configID, value string) ([]model.SessionConfigOption, error) {
	configID, value = strings.TrimSpace(configID), strings.TrimSpace(value)
	if configID == "" || value == "" {
		return nil, errors.New("Vibe configuration option and value are required")
	}
	option, found := findConfigOption(w.getConfigOptions(), configID)
	if !found {
		return nil, fmt.Errorf("Vibe did not advertise a %q configuration option", configID)
	}
	valid := false
	for _, choice := range option.Options {
		if choice.Value == value {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("Vibe does not offer %q for %s", value, option.Name)
	}
	response, err := w.conn.SetSessionConfigOption(ctx, acp.SetSessionConfigOptionRequest{ValueId: &acp.SetSessionConfigOptionValueId{
		SessionId: acp.SessionId(w.agentSessionID),
		ConfigId:  acp.SessionConfigId(configID),
		Value:     acp.SessionConfigValueId(value),
	}})
	if err != nil {
		return nil, fmt.Errorf("set Vibe %s: %w", strings.ToLower(option.Name), err)
	}
	options := projectConfigOptions(response.ConfigOptions)
	w.setConfigOptions(options)
	return options, nil
}

func syncWorkerMode(ctx context.Context, w *worker, mode string) error {
	mapped := agentMode(mode)
	if mapped == "" {
		return nil
	}
	option, found := findConfigOption(w.getConfigOptions(), "mode")
	if !found || option.CurrentValue == mapped {
		w.mode = mapped
		return nil
	}
	for _, choice := range option.Options {
		if choice.Value == mapped {
			if _, err := setWorkerConfigOption(ctx, w, "mode", mapped); err != nil {
				return err
			}
			w.mode = mapped
			return nil
		}
	}
	return nil
}

func findConfigOption(options []model.SessionConfigOption, id string) (model.SessionConfigOption, bool) {
	for _, option := range options {
		if option.ID == id {
			return option, true
		}
	}
	return model.SessionConfigOption{}, false
}

func stringPtr(value string) *string { return &value }
