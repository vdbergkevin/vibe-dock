export type Mode = string;
export type Theme = "dark" | "light" | "system";
export type AccentTheme = "mistral" | "tide" | "grove" | "cobalt" | "orchid";
export type WorkspaceKind = "chat" | "work" | "code";

export interface Conversation {
  id: string;
  projectId: string;
  title: string;
  mode: Mode;
  agentSessionId?: string;
  preview: string;
  updatedAt: string;
  createdAt: string;
  libraryIds: string[];
}

export interface Project {
  id: string;
  name: string;
  path: string;
  kind: WorkspaceKind;
  color: string;
  icon?: string;
  pinned: boolean;
  lastOpened: string;
  createdAt: string;
  sessions: Conversation[];
}

export interface Message {
  id: string;
  sessionId: string;
  role: "user" | "assistant" | "system";
  kind: "text" | "thought" | "tool" | "plan" | "error";
  content: string;
  status: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
}

export interface Plugin {
  id: string;
  name: string;
  description: string;
  transport: "stdio" | "http" | "sse";
  command: string;
  args: string[];
  env: Record<string, string>;
  enabled: boolean;
  scope: "global" | "project";
  updatedAt: string;
}

export interface LibraryDocument {
  id: string;
  libraryId: string;
  name: string;
  kind: "file" | "webpage";
  source: string;
  localPath?: string;
  size: number;
  status: string;
  createdAt: string;
}

export interface Library {
  id: string;
  name: string;
  description: string;
  color: string;
  remoteId?: string;
  documents: LibraryDocument[];
  createdAt: string;
  updatedAt: string;
}

export type MCPStatus = "disabled" | "connected" | "enabled" | "needs_auth" | "needs_setup" | "unavailable";

export interface MCPTool {
  name: string;
  description: string;
  enabled: boolean;
}

export interface MCPSource {
  id: string;
  name: string;
  displayName: string;
  kind: "connector" | "server";
  transport: string;
  status: MCPStatus;
  connected: boolean;
  enabled: boolean;
  scope: "managed" | "global" | "project";
  tools: MCPTool[];
  error?: string;
}

export interface MCPInventory {
  sources: MCPSource[];
  refreshedAt: string;
  cacheUpdatedAt?: string;
  cacheAvailable: boolean;
  cacheStale: boolean;
  errors: string[];
}

export interface Skill {
  id: string;
  name: string;
  originalName?: string;
  description: string;
  instructions: string;
  scope: "global" | "project";
  source: "vibe" | "agents" | "custom";
  projectId?: string;
  path: string;
  userInvocable: boolean;
  allowedTools: string[];
  enabled: boolean;
  editable: boolean;
  risk: "limited" | "write" | "shell";
  updatedAt: string;
}

export interface SkillInventory {
  skills: Skill[];
  projectId?: string;
  projectName?: string;
  globalPath: string;
  projectPath?: string;
  errors: string[];
}

export interface SessionConfigChoice {
  value: string;
  name: string;
  description?: string;
}

export interface SessionConfigOption {
  id: string;
  name: string;
  category: string;
  currentValue: string;
  options: SessionConfigChoice[];
}

export interface SlashCommand {
  name: string;
  description: string;
  inputHint?: string;
  source: "vibe" | "desktop";
}

export interface SessionConfiguration {
  options: SessionConfigOption[];
  commands: SlashCommand[];
}

export interface AccountStatus {
  available: boolean;
  configured: boolean;
  source?: string;
  detail: string;
}

export interface Environment {
  vibeAvailable: boolean;
  vibeVersion: string;
  acpAvailable: boolean;
  acpPath: string;
  gitAvailable: boolean;
  platform: string;
  account: AccountStatus;
}

export interface CodeEditor {
  id: "vscodium" | "vscode" | "cursor" | "zed" | "sublime";
  name: string;
  icon: string;
  available: boolean;
}

export interface Bootstrap {
  projects: Project[];
  plugins: Plugin[];
  libraries: Library[];
  environment: Environment;
  editors: CodeEditor[];
  editor: CodeEditor["id"];
  lastProject: string;
  workspace: WorkspaceKind;
  theme: Theme;
  accentTheme: AccentTheme;
}

export interface ChangedFile {
  path: string;
  status: "M" | "A" | "D" | "U";
  additions: number;
  deletions: number;
}

export interface TerminalSession {
  id: string;
  projectId: string;
  cwd: string;
  shell: string;
}

export interface TerminalDataEvent {
  sessionId: string;
  data: string;
}

export interface TerminalExitEvent {
  sessionId: string;
  exitCode: number;
  error?: string;
}

export interface PermissionOption {
  id: string;
  name: string;
  kind: string;
}

export interface PermissionRequest {
  requestId: string;
  title: string;
  kind: string;
  input?: unknown;
  options: PermissionOption[];
}

export interface ToolState {
  id: string;
  title: string;
  kind: string;
  status: string;
  locations?: Array<{ path: string; line?: number }>;
  input?: unknown;
  output?: unknown;
}

export interface PlanEntry {
  content: string;
  status: "pending" | "in_progress" | "completed";
  priority: string;
}

export interface StreamEvent {
  sessionId: string;
  messageId?: string;
  type: "text" | "thought" | "tool-call" | "tool-update" | "plan" | "permission" | "usage" | "mode" | "config-options" | "commands" | "conversation-title" | "message-saved" | "complete" | "error";
  text?: string;
  status?: string;
  data?: Record<string, any>;
}

export interface StreamState {
  running: boolean;
	startedAt: number | null;
	durationMs: number;
  activePrompt: string;
  answer: string;
  thought: string;
  tools: ToolState[];
  plan: PlanEntry[];
  permission: PermissionRequest | null;
  error: string;
  usage: { used: number; size: number } | null;
}
