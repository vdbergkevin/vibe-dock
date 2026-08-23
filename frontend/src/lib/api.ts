import { Browser, Call, Events } from "@wailsio/runtime";
import { demoBootstrap, demoChanges, demoMCPInventory, demoMessages, demoSessionConfiguration, demoSkillInventory } from "./demo";
import type { AccentTheme, Bootstrap, ChangedFile, Conversation, Environment, Library, MCPInventory, Message, Mode, Plugin, Project, SessionConfiguration, Skill, SkillInventory, StreamEvent, TerminalDataEvent, TerminalExitEvent, TerminalSession, Theme, WorkspaceKind } from "./types";

const listeners = new Set<(event: StreamEvent) => void>();
const terminalDataListeners = new Set<(event: TerminalDataEvent) => void>();
const terminalExitListeners = new Set<(event: TerminalExitEvent) => void>();
const previewTerminalInput: Record<string, string> = {};
let previewMode = false;

function nativeEnvironment() {
  return !previewMode && typeof window !== "undefined" && typeof (window as any)._wails?.invoke === "function";
}

function copy<T>(value: T): T {
  return structuredClone(value);
}

async function call<T>(method: string, ...args: unknown[]): Promise<T> {
  return Call.ByName(`main.AppService.${method}`, ...args) as Promise<T>;
}

function emit(event: StreamEvent) {
  listeners.forEach((listener) => listener(event));
}

function id(prefix: string) {
  return `${prefix}_${Math.random().toString(36).slice(2, 10)}`;
}

export const api = {
  get isNative() { return nativeEnvironment(); },
  async bootstrap(): Promise<Bootstrap> {
    // The runtime briefly installs a browser placeholder while initialising.
    // Give it a frame to settle before deciding between desktop and preview mode.
    await new Promise((resolve) => window.setTimeout(resolve, 30));
    if (!nativeEnvironment()) {
      previewMode = true;
      return copy(demoBootstrap);
    }
    try {
      return await call<Bootstrap>("Bootstrap");
    } catch (error) {
      if (location.hostname === "127.0.0.1" || location.hostname === "localhost") {
        previewMode = true;
        return copy(demoBootstrap);
      }
      throw error;
    }
  },
  async pickProjectFolder(): Promise<string> {
    return nativeEnvironment() ? call<string>("PickProjectFolder") : "/Users/kevin/Repositories/new-project";
  },
  async addProject(path: string): Promise<Project> {
    if (nativeEnvironment()) return call<Project>("AddProject", path);
    const project: Project = { id: id("prj"), name: path.split("/").filter(Boolean).pop() || "project", path, kind: "code", color: "#5c91d8", pinned: false, lastOpened: new Date().toISOString(), createdAt: new Date().toISOString(), sessions: [] };
    return project;
  },
  async createChatProject(name: string): Promise<Project> {
    return this.createManagedProject(name, "chat");
  },
  async createManagedProject(name: string, kind: "chat" | "work", icon = "", color = ""): Promise<Project> {
    if (nativeEnvironment()) return call<Project>("CreateManagedProjectWithAppearance", name, kind, icon, color);
    return { id: id("prj"), name: name.trim() || (kind === "work" ? "Work" : "Chats"), path: `/private/vibe-${kind}/${id("workspace")}`, kind, icon: icon || (kind === "work" ? "briefcase" : "messages"), color: color || (kind === "work" ? "#4fa78f" : "#ff7417"), pinned: false, lastOpened: new Date().toISOString(), createdAt: new Date().toISOString(), sessions: [] };
  },
  async updateProjectAppearance(projectId: string, icon: string, color: string): Promise<Project> {
    if (nativeEnvironment()) return call<Project>("UpdateProjectAppearance", projectId, icon, color);
    const project = demoBootstrap.projects.find((item) => item.id === projectId);
    if (!project) throw new Error("Project not found");
    project.icon = icon;
    project.color = color;
    return copy(project);
  },
  async removeProject(projectId: string) {
    if (nativeEnvironment()) await call<void>("RemoveProject", projectId);
  },
  async selectProject(projectId: string) {
    if (nativeEnvironment()) await call<void>("SelectProject", projectId);
  },
  async setWorkspaceKind(kind: WorkspaceKind) {
    if (nativeEnvironment()) await call<void>("SetWorkspaceKind", kind);
    else demoBootstrap.workspace = kind;
  },
  async createConversation(projectId: string, title = "New conversation"): Promise<Conversation> {
    if (nativeEnvironment()) return call<Conversation>("CreateConversation", projectId, title);
    const conversation = { id: id("ses"), projectId, title, mode: "default", preview: "", updatedAt: new Date().toISOString(), createdAt: new Date().toISOString(), libraryIds: [] } satisfies Conversation;
    const project = demoBootstrap.projects.find((item) => item.id === projectId);
    if (project) project.sessions.unshift(conversation);
    return copy(conversation);
  },
  async deleteConversation(sessionId: string) {
    if (nativeEnvironment()) {
      await call<void>("DeleteConversation", sessionId);
      return;
    }
    for (const project of demoBootstrap.projects) {
      project.sessions = project.sessions.filter((session) => session.id !== sessionId);
    }
    delete demoMessages[sessionId];
  },
  async messages(sessionId: string): Promise<Message[]> {
    return nativeEnvironment() ? call<Message[]>("Messages", sessionId) : copy(demoMessages[sessionId] || []);
  },
  async setMode(sessionId: string, mode: Mode): Promise<SessionConfiguration> {
    if (nativeEnvironment()) return call<SessionConfiguration>("SetConversationMode", sessionId, mode);
    const value = mode === "default" ? "ask" : mode;
    demoSessionConfiguration.options = demoSessionConfiguration.options.map((option) => option.id === "mode" ? { ...option, currentValue: value } : option);
    return copy(demoSessionConfiguration);
  },
  async sessionConfiguration(sessionId: string): Promise<SessionConfiguration> {
    return nativeEnvironment() ? call<SessionConfiguration>("SessionConfiguration", sessionId) : copy(demoSessionConfiguration);
  },
  async setSessionConfigOption(sessionId: string, configId: string, value: string): Promise<SessionConfiguration> {
    if (nativeEnvironment()) return call<SessionConfiguration>("SetSessionConfigOption", sessionId, configId, value);
    demoSessionConfiguration.options = demoSessionConfiguration.options.map((option) => option.id === configId ? { ...option, currentValue: value } : option);
    return copy(demoSessionConfiguration);
  },
  async setConversationLibraries(sessionId: string, libraryIds: string[]): Promise<Conversation> {
    if (nativeEnvironment()) return call<Conversation>("SetConversationLibraries", sessionId, libraryIds);
    for (const project of demoBootstrap.projects) {
      const conversation = project.sessions.find((session) => session.id === sessionId);
      if (conversation) {
        conversation.libraryIds = [...libraryIds];
        return copy(conversation);
      }
    }
    throw new Error("Conversation not found");
  },
  async createLibrary(name: string, description: string): Promise<Library> {
    if (nativeEnvironment()) return call<Library>("CreateLibrary", name, description);
    const now = new Date().toISOString();
    const library: Library = { id: id("lib"), name: name.trim(), description: description.trim(), color: "#ff7417", documents: [], createdAt: now, updatedAt: now };
    demoBootstrap.libraries.unshift(library);
    return copy(library);
  },
  async deleteLibrary(libraryId: string) {
    if (nativeEnvironment()) await call<void>("DeleteLibrary", libraryId);
    else demoBootstrap.libraries = demoBootstrap.libraries.filter((library) => library.id !== libraryId);
  },
  async pickLibraryDocuments(): Promise<string[]> {
    return nativeEnvironment() ? call<string[]>("PickLibraryDocuments") : [];
  },
  async addLibraryDocuments(libraryId: string, paths: string[]): Promise<Library> {
    if (nativeEnvironment()) return call<Library>("AddLibraryDocuments", libraryId, paths);
    const library = demoBootstrap.libraries.find((item) => item.id === libraryId);
    if (!library) throw new Error("Library not found");
    const now = new Date().toISOString();
    library.documents.push(...paths.map((path) => ({ id: id("doc"), libraryId, name: path.split("/").pop() || path, kind: "file" as const, source: path, localPath: path, size: 0, status: "ready", createdAt: now })));
    library.updatedAt = now;
    return copy(library);
  },
  async addLibraryWebpage(libraryId: string, url: string): Promise<Library> {
    if (nativeEnvironment()) return call<Library>("AddLibraryWebpage", libraryId, url);
    const library = demoBootstrap.libraries.find((item) => item.id === libraryId);
    if (!library) throw new Error("Library not found");
    const parsed = new URL(url);
    const now = new Date().toISOString();
    library.documents.push({ id: id("doc"), libraryId, name: parsed.hostname, kind: "webpage", source: parsed.toString(), size: 0, status: "ready", createdAt: now });
    library.updatedAt = now;
    return copy(library);
  },
  async deleteLibraryDocument(libraryId: string, documentId: string): Promise<Library> {
    if (nativeEnvironment()) return call<Library>("DeleteLibraryDocument", libraryId, documentId);
    const library = demoBootstrap.libraries.find((item) => item.id === libraryId);
    if (!library) throw new Error("Library not found");
    library.documents = library.documents.filter((document) => document.id !== documentId);
    library.updatedAt = new Date().toISOString();
    return copy(library);
  },
  async setTheme(theme: Theme) {
    if (nativeEnvironment()) await call<void>("SetTheme", theme);
    else demoBootstrap.theme = theme;
  },
  async setResolvedTheme(theme: "dark" | "light") {
    if (nativeEnvironment()) await call<void>("SetResolvedTheme", theme);
  },
  async setAccentTheme(theme: AccentTheme) {
    if (nativeEnvironment()) await call<void>("SetAccentTheme", theme);
    else demoBootstrap.accentTheme = theme;
  },
  async setCodeEditor(editorId: Bootstrap["editor"]) {
    if (nativeEnvironment()) await call<void>("SetCodeEditor", editorId);
    else demoBootstrap.editor = editorId;
  },
  async refreshEnvironment(): Promise<Environment> {
    return nativeEnvironment() ? call<Environment>("RefreshEnvironment") : copy(demoBootstrap.environment);
  },
  async openVibeSetup() {
    if (nativeEnvironment()) await call<void>("OpenVibeSetup");
  },
  async openMistralPage(url: string) {
    const destination = new URL(url);
    if (destination.protocol !== "https:" || (destination.hostname !== "mistral.ai" && !destination.hostname.endsWith(".mistral.ai"))) {
      throw new Error("Only official Mistral pages can be opened from this action");
    }
    if (nativeEnvironment()) await Browser.OpenURL(destination);
    else window.open(destination, "_blank", "noopener,noreferrer");
  },
  async sendPrompt(sessionId: string, text: string): Promise<Message> {
    if (nativeEnvironment()) return call<Message>("SendPrompt", sessionId, text);
    const message: Message = { id: id("msg"), sessionId, role: "user", kind: "text", content: text, status: "complete", createdAt: new Date().toISOString() };
    for (const project of demoBootstrap.projects) {
      const conversation = project.sessions.find((session) => session.id === sessionId);
      if (conversation && ["new conversation", "new chat", "new task"].includes(conversation.title.trim().toLowerCase())) {
        conversation.title = previewConversationSubject(text);
        emit({ sessionId, type: "conversation-title", text: conversation.title });
        break;
      }
    }
    simulate(sessionId, text);
    return message;
  },
  async cancelPrompt(sessionId: string) {
    if (nativeEnvironment()) await call<void>("CancelPrompt", sessionId);
    emit({ sessionId, type: "complete", status: "cancelled" });
  },
  async respondPermission(requestId: string, optionId: string) {
    if (nativeEnvironment()) await call<void>("RespondPermission", { requestId, optionId });
  },
  async savePlugin(plugin: Plugin): Promise<Plugin> {
    if (nativeEnvironment()) return call<Plugin>("SavePlugin", plugin);
    return { ...plugin, id: plugin.id || id("mcp"), updatedAt: new Date().toISOString() };
  },
  async mcpInventory(projectId = ""): Promise<MCPInventory> {
    return nativeEnvironment() ? call<MCPInventory>("MCPInventory", projectId) : copy(demoMCPInventory);
  },
  async setConnectorEnabled(projectId: string, name: string, enabled: boolean): Promise<MCPInventory> {
    if (nativeEnvironment()) return call<MCPInventory>("SetConnectorEnabled", projectId, name, enabled);
    const source = demoMCPInventory.sources.find((item) => item.kind === "connector" && item.name === name);
    if (source) {
      source.enabled = enabled;
      source.status = source.connected && enabled ? "connected" : "disabled";
      source.tools = source.tools.map((tool) => ({ ...tool, enabled: source.connected && enabled }));
    }
    return copy(demoMCPInventory);
  },
  async skills(projectId = ""): Promise<SkillInventory> {
    if (nativeEnvironment()) return call<SkillInventory>("Skills", projectId);
    return copy({ ...demoSkillInventory, projectId: projectId || undefined, projectName: projectId ? demoSkillInventory.projectName : undefined, projectPath: projectId ? demoSkillInventory.projectPath : undefined });
  },
  async saveSkill(skill: Skill): Promise<Skill> {
    if (nativeEnvironment()) return call<Skill>("SaveSkill", skill);
    const now = new Date().toISOString();
    const saved: Skill = { ...skill, id: `${skill.scope}:vibe:${skill.name}`, originalName: skill.name, source: "vibe", path: skill.scope === "project" ? `${demoSkillInventory.projectPath}/${skill.name}/SKILL.md` : `${demoSkillInventory.globalPath}/${skill.name}/SKILL.md`, editable: true, enabled: true, updatedAt: now };
    const index = demoSkillInventory.skills.findIndex((item) => item.scope === skill.scope && item.source === "vibe" && item.name === (skill.originalName || skill.name));
    if (index >= 0) demoSkillInventory.skills[index] = saved;
    else demoSkillInventory.skills.unshift(saved);
    return copy(saved);
  },
  async setSkillEnabled(scope: Skill["scope"], projectId: string, name: string, enabled: boolean) {
    if (nativeEnvironment()) await call<void>("SetSkillEnabled", scope, projectId, name, enabled);
    else demoSkillInventory.skills = demoSkillInventory.skills.map((skill) => skill.scope === scope && skill.name === name ? { ...skill, enabled } : skill);
  },
  async deleteSkill(scope: Skill["scope"], projectId: string, name: string) {
    if (nativeEnvironment()) await call<void>("DeleteSkill", scope, projectId, name);
    else demoSkillInventory.skills = demoSkillInventory.skills.filter((skill) => !(skill.scope === scope && skill.source === "vibe" && skill.name === name));
  },
  async pickSkillFolder(): Promise<string> {
    return nativeEnvironment() ? call<string>("PickSkillFolder") : "";
  },
  async importSkill(sourcePath: string, scope: Skill["scope"], projectId: string): Promise<Skill> {
    if (nativeEnvironment()) return call<Skill>("ImportSkill", sourcePath, scope, projectId);
    throw new Error("Skill import is available in the desktop app");
  },
  async reloadSkills() {
    if (nativeEnvironment()) await call<void>("ReloadSkills");
  },
  async gitStatus(projectId: string): Promise<ChangedFile[]> {
    return nativeEnvironment() ? call<ChangedFile[]>("GitStatus", projectId) : copy(demoChanges);
  },
  async startProjectTerminal(projectId: string, columns: number, rows: number): Promise<TerminalSession> {
    if (nativeEnvironment()) return call<TerminalSession>("StartProjectTerminal", projectId, columns, rows);
    const project = demoBootstrap.projects.find((item) => item.id === projectId);
    if (!project) throw new Error("Project not found");
    const session: TerminalSession = { id: id("term"), projectId, cwd: project.path, shell: "zsh" };
    previewTerminalInput[session.id] = "";
    window.setTimeout(() => terminalDataListeners.forEach((listener) => listener({ sessionId: session.id, data: `\u001b[38;2;255;116;23mVibeDock terminal\u001b[0m\r\n${project.path}\r\n% ` })), 80);
    return session;
  },
  async attachProjectTerminal(sessionId: string) {
    if (nativeEnvironment()) await call<void>("AttachProjectTerminal", sessionId);
  },
  async writeProjectTerminal(sessionId: string, data: string) {
    if (nativeEnvironment()) {
      await call<void>("WriteProjectTerminal", sessionId, data);
      return;
    }
    if (!(sessionId in previewTerminalInput)) return;
    if (data === "\r") {
      const command = previewTerminalInput[sessionId].trim();
      previewTerminalInput[sessionId] = "";
      let output = "";
      if (command === "pwd") output = demoBootstrap.projects.find((project) => project.id === demoBootstrap.lastProject)?.path || "/project";
      else if (command === "clear") output = "\u001b[2J\u001b[H";
      else if (command) output = `zsh: command not found: ${command}`;
      terminalDataListeners.forEach((listener) => listener({ sessionId, data: `\r\n${output}${output ? "\r\n" : ""}% ` }));
    } else if (data === "\u007f") {
      previewTerminalInput[sessionId] = previewTerminalInput[sessionId].slice(0, -1);
      terminalDataListeners.forEach((listener) => listener({ sessionId, data: "\b \b" }));
    } else if (!data.startsWith("\u001b")) {
      previewTerminalInput[sessionId] += data;
      terminalDataListeners.forEach((listener) => listener({ sessionId, data }));
    }
  },
  async resizeProjectTerminal(sessionId: string, columns: number, rows: number) {
    if (nativeEnvironment()) await call<void>("ResizeProjectTerminal", sessionId, columns, rows);
  },
  async stopProjectTerminal(sessionId: string) {
    if (nativeEnvironment()) await call<void>("StopProjectTerminal", sessionId);
    else {
      delete previewTerminalInput[sessionId];
      terminalExitListeners.forEach((listener) => listener({ sessionId, exitCode: 0 }));
    }
  },
  async openProjectTerminal(projectId: string) {
    if (nativeEnvironment()) await call<void>("OpenProjectTerminal", projectId);
  },
  async openProjectInEditor(projectId: string) {
    if (nativeEnvironment()) await call<void>("OpenProjectInEditor", projectId);
  },
  subscribe(callback: (event: StreamEvent) => void) {
    listeners.add(callback);
    let unsubscribeNative = () => {};
    if (nativeEnvironment()) {
      unsubscribeNative = Events.On("vibe:stream", (event) => callback(event.data as StreamEvent));
      const unsubscribeSaved = Events.On("vibe:message-saved", (event) => {
        const message = event.data as Message;
        callback({ sessionId: message.sessionId, type: "message-saved", data: { message } });
      });
      const original = unsubscribeNative;
      unsubscribeNative = () => { original(); unsubscribeSaved(); };
    }
    return () => { listeners.delete(callback); unsubscribeNative(); };
  },
  subscribeTerminal(onData: (event: TerminalDataEvent) => void, onExit: (event: TerminalExitEvent) => void) {
    terminalDataListeners.add(onData);
    terminalExitListeners.add(onExit);
    let unsubscribeData = () => {};
    let unsubscribeExit = () => {};
    if (nativeEnvironment()) {
      unsubscribeData = Events.On("terminal:data", (event) => onData(event.data as TerminalDataEvent));
      unsubscribeExit = Events.On("terminal:exit", (event) => onExit(event.data as TerminalExitEvent));
    }
    return () => {
      terminalDataListeners.delete(onData);
      terminalExitListeners.delete(onExit);
      unsubscribeData();
      unsubscribeExit();
    };
  }
};

function previewConversationSubject(prompt: string) {
  let title = prompt.replace(/\s+/g, " ").trim();
  for (const prefix of ["can you please ", "could you please ", "can you ", "could you ", "i would love to ", "i'd love to ", "please "]) {
    if (title.toLowerCase().startsWith(prefix)) {
      title = title.slice(prefix.length).trim();
      break;
    }
  }
  const words = title.split(" ").filter(Boolean);
  const truncated = words.length > 10 || title.length > 64;
  title = words.slice(0, 10).join(" ").slice(0, 64).trim().replace(/[.,!?;:]+$/, "");
  if (!title) return "New conversation";
  title = title[0].toUpperCase() + title.slice(1);
  return truncated ? `${title}\u2026` : title;
}

function simulate(sessionId: string, prompt: string) {
  const lowerPrompt = prompt.toLowerCase();
  const answer = lowerPrompt.includes("markdown")
    ? "## Overview\n\nMarkdown now renders as structured content.\n\n### Details\n\n- Headings use the correct hierarchy\n- Lists, **bold text**, and `inline code` are supported\n\n> Streaming remains safe and readable.\n\n| Feature | Status |\n| --- | --- |\n| Headings | Working |\n| Tables | Working |\n\n```ts\nconst markdownReady = true;\n```"
    : lowerPrompt.includes("plugin")
      ? "I’ll add the plugin through the shared MCP configuration, validate its command, and keep credentials out of SQLite."
      : "I’ve mapped this into the active project and prepared a focused implementation. The desktop shell will receive every update as a typed ACP event, so the interface stays responsive while Vibe works.";
  emit({ sessionId, type: "thought", text: "Inspecting the active project and choosing the smallest coherent change…" });
  emit({ sessionId, type: "tool-call", status: "in_progress", data: { id: "tool_demo", title: "Read project files", kind: "read", locations: [{ path: "frontend/src/App.svelte" }] } });
  const words = answer.split(" ");
  words.forEach((word, index) => setTimeout(() => emit({ sessionId, type: "text", text: `${word}${index === words.length - 1 ? "" : " "}` }), 260 + index * 34));
  setTimeout(() => emit({ sessionId, type: "tool-update", status: "completed", data: { id: "tool_demo", title: "Read project files" } }), 520);
  const durationMs = 350 + words.length * 34;
  setTimeout(() => emit({ sessionId, type: "complete", status: "end_turn", data: { durationMs } }), durationMs);
}
