<script lang="ts">
  import { afterUpdate, onMount, tick } from "svelte";
  import { BriefcaseBusiness, Bug, Code2, FileSearch, FolderPlus, GitCompareArrows, Lightbulb, ListChecks, LoaderCircle, MessageSquarePlus, MessagesSquare, Network, RotateCcw, Scale, Search, SearchCode, Sparkles, Trash2, X } from "@lucide/svelte";
  import Sidebar from "./components/Sidebar.svelte";
  import ChatHeader from "./components/ChatHeader.svelte";
  import Messages from "./components/Messages.svelte";
  import Composer from "./components/Composer.svelte";
  import ChangesPanel from "./components/ChangesPanel.svelte";
  import PluginsView from "./components/PluginsView.svelte";
  import SettingsView from "./components/SettingsView.svelte";
  import LibrariesView from "./components/LibrariesView.svelte";
  import SkillsView from "./components/SkillsView.svelte";
  import TerminalPanel from "./components/TerminalPanel.svelte";
  import ProjectAppearancePicker from "./components/ProjectAppearancePicker.svelte";
  import ProjectAvatar from "./components/ProjectAvatar.svelte";
  import VibeDockMark from "./components/VibeDockMark.svelte";
  import { api } from "./lib/api";
  import { mistralLinks, type MistralDestination } from "./lib/mistral-links";
  import { defaultProjectAppearance } from "./lib/project-icons";
  import type { AccentTheme, Bootstrap, ChangedFile, CodeEditor, Conversation, Library, MCPInventory, Message, Mode, PermissionRequest, PlanEntry, Plugin, Project, SessionConfiguration, Skill, SkillInventory, StreamEvent, StreamState, Theme, ToolState, WorkspaceKind } from "./lib/types";

  const fallbackCodeEditor: CodeEditor = { id: "vscodium", name: "VSCodium", icon: "vscodium", available: false };

  let data: Bootstrap | null = null;
  let projects: Project[] = [];
  let plugins: Plugin[] = [];
  let libraries: Library[] = [];
  let libraryBusy = "";
  let mcpInventory: MCPInventory = { sources: [], refreshedAt: "", cacheAvailable: false, cacheStale: false, errors: [] };
  let mcpLoading = false;
  let connectorUpdating = "";
  let skillInventory: SkillInventory = { skills: [], globalPath: "", errors: [] };
  let skillsLoading = false;
  let sessionConfigurations: Record<string, SessionConfiguration> = {};
  let sessionConfigLoading: Record<string, boolean> = {};
  let activeProjectId = "";
  let activeSessionId = "";
  let messages: Message[] = [];
  let streams: Record<string, StreamState> = {};
  let queuedPrompts: Record<string, string[]> = {};
  let view: "chat" | "libraries" | "skills" | "plugins" | "settings" = "chat";
  let loading = true;
  let fatalError = "";
  let changesOpen = false;
  let terminalOpen = false;
  let changes: ChangedFile[] = [];
  let changesLoading = false;
  let paletteOpen = false;
  let paletteQuery = "";
  let toast = "";
  let accountRefreshing = false;
  let accountPollTimer = 0;
  let accountPollAttempts = 0;
  let messageViewport: HTMLElement;
  let paletteInput: HTMLInputElement;
  let stickToBottom = true;
  let theme: Theme = "dark";
  let accentTheme: AccentTheme = "mistral";
  let workspaceKind: WorkspaceKind = "code";
  let sidebarCollapsed = false;
  let managedProjectModal = false;
  let managedProjectKind: "chat" | "work" = "chat";
  let managedProjectName = "";
  let managedProjectIcon = "messages";
  let managedProjectColor = "#ff7417";
  let managedProjectEditId = "";
  let managedProjectInput: HTMLInputElement;
  let deletingConversation: { projectId: string; session: Conversation } | null = null;
  let deletingConversationBusy = false;
  let editors: CodeEditor[] = [fallbackCodeEditor];
  let editorId: CodeEditor["id"] = "vscodium";
  let codeEditor = fallbackCodeEditor;
  let terminalHeight = 270;

  $: activeProject = projects.find((project) => project.id === activeProjectId);
  $: activeConversation = activeProject?.sessions.find((session) => session.id === activeSessionId);
  $: activeStream = streams[activeSessionId] || emptyStream();
  $: activeConfiguration = sessionConfigurations[activeSessionId] || { options: [], commands: [] };
  $: anyStreamRunning = Object.values(streams).some((stream) => stream.running);
  $: runningSessionIds = new Set(Object.entries(streams).filter(([, stream]) => stream.running).map(([sessionId]) => sessionId));
  $: allConversations = projects.filter((project) => project.kind === workspaceKind).flatMap((project) => project.sessions.map((session) => ({ project, session })));
  $: paletteResults = allConversations.filter(({ project, session }) => `${project.name} ${session.title}`.toLowerCase().includes(paletteQuery.toLowerCase())).slice(0, 12);
  $: codeEditor = editors.find((editor) => editor.id === editorId) || editors[0] || fallbackCodeEditor;
  $: if (paletteOpen) tick().then(() => paletteInput?.focus());
  $: if (managedProjectModal) tick().then(() => managedProjectInput?.focus());

  onMount(() => {
    sidebarCollapsed = window.localStorage.getItem("vibe-sidebar-collapsed") === "true";
    const savedTerminalHeight = Number(window.localStorage.getItem("vibe-terminal-height"));
    if (Number.isFinite(savedTerminalHeight) && savedTerminalHeight >= 140) terminalHeight = savedTerminalHeight;
    const unsubscribe = api.subscribe(handleStream);
    const systemTheme = window.matchMedia("(prefers-color-scheme: dark)");
    const followSystemTheme = () => { if (theme === "system") applyTheme(theme); };
    systemTheme.addEventListener("change", followSystemTheme);
    void initialise();
    window.addEventListener("keydown", keyboardShortcuts);
    return () => {
      unsubscribe();
      if (accountPollTimer) window.clearInterval(accountPollTimer);
      systemTheme.removeEventListener("change", followSystemTheme);
      window.removeEventListener("keydown", keyboardShortcuts);
    };
  });

  afterUpdate(() => {
    if (stickToBottom && messageViewport) messageViewport.scrollTop = messageViewport.scrollHeight;
  });

  async function initialise() {
    try {
      data = await api.bootstrap();
      theme = normaliseTheme(data.theme);
      applyTheme(theme);
      accentTheme = normaliseAccentTheme(data.accentTheme);
      applyAccentTheme(accentTheme);
      projects = data.projects || [];
      plugins = data.plugins || [];
      libraries = data.libraries || [];
      editors = data.editors?.length ? data.editors : [fallbackCodeEditor];
      editorId = data.editor || editors[0].id;
      workspaceKind = data.workspace === "chat" || data.workspace === "work" ? data.workspace : "code";
      const workspaceProjects = projects.filter((project) => project.kind === workspaceKind);
      const initialProject = workspaceProjects.find((project) => project.id === data?.lastProject) || workspaceProjects[0];
      if (initialProject) {
        activeProjectId = initialProject.id;
        activeSessionId = initialProject.sessions[0]?.id || "";
        if (activeSessionId) {
          messages = await api.messages(activeSessionId);
          void loadSessionConfiguration(activeSessionId);
        }
      }
    } catch (error) {
      fatalError = errorMessage(error);
    } finally {
      loading = false;
    }
  }

  async function selectProject(projectId: string) {
    activeProjectId = projectId;
    const project = projects.find((item) => item.id === projectId);
    if (!project) return;
    workspaceKind = project.kind;
    activeSessionId = project?.sessions[0]?.id || "";
    if (activeSessionId) {
      messages = await api.messages(activeSessionId);
      void loadSessionConfiguration(activeSessionId);
    } else {
      messages = [];
    }
    view = "chat";
    changesOpen = false;
    if (project.kind !== "code") terminalOpen = false;
    void api.selectProject(projectId);
  }

  async function switchWorkspace(kind: WorkspaceKind) {
    workspaceKind = kind;
    view = "chat";
    changesOpen = false;
    if (kind !== "code") terminalOpen = false;
    paletteOpen = false;
    void api.setWorkspaceKind(kind);
    const nextProject = projects.find((project) => project.kind === kind);
    if (nextProject) {
      await selectProject(nextProject.id);
      return;
    }
    activeProjectId = "";
    activeSessionId = "";
    messages = [];
  }

  async function selectSession(projectId: string, sessionId: string) {
    activeProjectId = projectId;
    activeSessionId = sessionId;
    view = "chat";
    messages = await api.messages(sessionId);
    void loadSessionConfiguration(sessionId);
    stickToBottom = true;
    await tick();
  }

  async function newConversation(projectId = activeProjectId) {
    try {
      if (!projectId) {
        if (workspaceKind === "code") { await addProject(); return; }
        const project = await createManagedProjectRecord(workspaceKind === "work" ? "Work" : "Chats", workspaceKind);
        projectId = project.id;
      }
      const project = projects.find((item) => item.id === projectId);
      const conversation = await api.createConversation(projectId, project?.kind === "chat" ? "New chat" : project?.kind === "work" ? "New task" : "New conversation");
      projects = projects.map((project) => project.id === projectId ? { ...project, sessions: [conversation, ...project.sessions] } : project);
      await selectSession(projectId, conversation.id);
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function removeConversation() {
    if (!deletingConversation || deletingConversationBusy) return;
    const target = deletingConversation;
    const sessionId = target.session.id;
    const wasActive = activeSessionId === sessionId;
    const nextSession = projects.find((project) => project.id === target.projectId)?.sessions.find((session) => session.id !== sessionId);
    deletingConversationBusy = true;
    try {
      await api.deleteConversation(sessionId);
      projects = projects.map((project) => project.id === target.projectId ? { ...project, sessions: project.sessions.filter((session) => session.id !== sessionId) } : project);

      const nextStreams = { ...streams };
      const nextQueues = { ...queuedPrompts };
      const nextConfigurations = { ...sessionConfigurations };
      const nextConfigurationLoading = { ...sessionConfigLoading };
      delete nextStreams[sessionId];
      delete nextQueues[sessionId];
      delete nextConfigurations[sessionId];
      delete nextConfigurationLoading[sessionId];
      streams = nextStreams;
      queuedPrompts = nextQueues;
      sessionConfigurations = nextConfigurations;
      sessionConfigLoading = nextConfigurationLoading;
      deletingConversation = null;

      if (wasActive) {
        if (nextSession) {
          await selectSession(target.projectId, nextSession.id);
        } else {
          activeProjectId = target.projectId;
          activeSessionId = "";
          messages = [];
        }
      }
      showToast(`“${target.session.title}” removed`);
    } catch (error) {
      showToast(errorMessage(error));
    } finally {
      deletingConversationBusy = false;
    }
  }

  async function addProject() {
    if (workspaceKind !== "code") {
      managedProjectName = "";
      managedProjectKind = workspaceKind;
      const appearance = defaultProjectAppearance(workspaceKind);
      managedProjectIcon = appearance.icon;
      managedProjectColor = appearance.color;
      managedProjectEditId = "";
      managedProjectModal = true;
      return;
    }
    try {
      const path = await api.pickProjectFolder();
      if (!path) return;
      const project = await api.addProject(path);
      projects = [project, ...projects];
      workspaceKind = "code";
      activeProjectId = project.id;
      activeSessionId = "";
      messages = [];
      view = "chat";
      showToast(`Added ${project.name}`);
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function createManagedProjectRecord(name: string, kind: "chat" | "work", icon = "", color = "") {
    const appearance = defaultProjectAppearance(kind);
    const project = await api.createManagedProject(name, kind, icon || appearance.icon, color || appearance.color);
    projects = [project, ...projects];
    workspaceKind = kind;
    activeProjectId = project.id;
    activeSessionId = "";
    messages = [];
    view = "chat";
    return project;
  }

  async function submitManagedProject() {
    const name = managedProjectName.trim();
    if (!name) return;
    try {
      if (managedProjectEditId) {
        const project = await api.updateProjectAppearance(managedProjectEditId, managedProjectIcon, managedProjectColor);
        projects = projects.map((item) => item.id === project.id ? project : item);
        managedProjectModal = false;
        managedProjectEditId = "";
        showToast(`${project.name} appearance updated`);
        return;
      }
      const project = await createManagedProjectRecord(name, managedProjectKind, managedProjectIcon, managedProjectColor);
      managedProjectModal = false;
      showToast(`${project.name} created`);
      await newConversation(project.id);
    } catch (error) { showToast(errorMessage(error)); }
  }

  function openManagedProjectAppearance(project: Project) {
    if (project.kind !== "chat" && project.kind !== "work") return;
    const fallback = defaultProjectAppearance(project.kind);
    managedProjectEditId = project.id;
    managedProjectKind = project.kind;
    managedProjectName = project.name;
    managedProjectIcon = project.icon || fallback.icon;
    managedProjectColor = project.color || fallback.color;
    managedProjectModal = true;
  }

  async function setConversationLibraries(libraryIds: string[]) {
    if (!activeConversation) return;
    try {
      const updated = await api.setConversationLibraries(activeConversation.id, libraryIds);
      projects = projects.map((project) => ({ ...project, sessions: project.sessions.map((session) => session.id === updated.id ? updated : session) }));
      const names = libraries.filter((library) => libraryIds.includes(library.id)).map((library) => library.name);
      showToast(names.length ? `${names.length} ${names.length === 1 ? "Library" : "Libraries"} attached` : "Libraries detached");
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function createLibrary(name: string, description: string) {
    try {
      const library = await api.createLibrary(name, description);
      libraries = [library, ...libraries];
      showToast(`${library.name} created`);
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function deleteLibrary(libraryId: string) {
    try {
      await api.deleteLibrary(libraryId);
      libraries = libraries.filter((library) => library.id !== libraryId);
      projects = projects.map((project) => ({ ...project, sessions: project.sessions.map((session) => ({ ...session, libraryIds: session.libraryIds.filter((id) => id !== libraryId) })) }));
      showToast("Library deleted");
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function uploadLibraryDocuments(libraryId: string) {
    try {
      const paths = await api.pickLibraryDocuments();
      if (!paths.length) return;
      libraryBusy = libraryId;
      const updated = await api.addLibraryDocuments(libraryId, paths);
      libraries = libraries.map((library) => library.id === updated.id ? updated : library);
      showToast(`${paths.length} ${paths.length === 1 ? "document" : "documents"} added`);
    } catch (error) { showToast(errorMessage(error)); }
    finally { libraryBusy = ""; }
  }

  async function addLibraryWebpage(libraryId: string, url: string) {
    try {
      const updated = await api.addLibraryWebpage(libraryId, url);
      libraries = libraries.map((library) => library.id === updated.id ? updated : library);
      showToast("Webpage added");
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function deleteLibraryDocument(libraryId: string, documentId: string) {
    try {
      const updated = await api.deleteLibraryDocument(libraryId, documentId);
      libraries = libraries.map((library) => library.id === updated.id ? updated : library);
      showToast("Source removed");
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function send(text: string) {
    if (!activeSessionId) return;
    const sessionId = activeSessionId;
    const prompt = text.trim();
    if (!prompt) return;
    if ((streams[sessionId] || emptyStream()).running) {
      queuedPrompts = { ...queuedPrompts, [sessionId]: [...(queuedPrompts[sessionId] || []), prompt] };
      showToast("Follow-up queued");
      return;
    }
    await startPrompt(sessionId, prompt);
  }

  async function startPrompt(sessionId: string, text: string) {
    streams = { ...streams, [sessionId]: { ...emptyStream(), running: true, startedAt: Date.now(), activePrompt: text } };
    stickToBottom = true;
    try {
      const message = await api.sendPrompt(sessionId, text);
      if (activeSessionId === sessionId) messages = [...messages, message];
    } catch (error) {
      const current = streams[sessionId] || emptyStream();
      streams = { ...streams, [sessionId]: { ...current, running: false, error: errorMessage(error) } };
      drainQueuedPrompt(sessionId);
    }
  }

  function drainQueuedPrompt(sessionId: string) {
    const queue = queuedPrompts[sessionId] || [];
    if (!queue.length) return;
    const [next, ...remaining] = queue;
    const updated = { ...queuedPrompts };
    if (remaining.length) updated[sessionId] = remaining;
    else delete updated[sessionId];
    queuedPrompts = updated;
    void startPrompt(sessionId, next);
  }

  function removeQueuedPrompt(sessionId: string, index: number) {
    const queue = queuedPrompts[sessionId] || [];
    if (index < 0 || index >= queue.length) return;
    const remaining = queue.filter((_, itemIndex) => itemIndex !== index);
    const updated = { ...queuedPrompts };
    if (remaining.length) updated[sessionId] = remaining;
    else delete updated[sessionId];
    queuedPrompts = updated;
    showToast("Queued follow-up removed");
  }

  async function cancel() {
    if (!activeSessionId) return;
    await api.cancelPrompt(activeSessionId);
  }

  async function setMode(mode: Mode) {
    if (!activeConversation) return;
    const sessionId = activeConversation.id;
    const previousMode = activeConversation.mode;
    const previousConfiguration = sessionConfigurations[sessionId];
    projects = projects.map((project) => ({ ...project, sessions: project.sessions.map((session) => session.id === sessionId ? { ...session, mode } : session) }));
    setOptimisticConfig(sessionId, "mode", mode === "default" ? "ask" : mode);
    try {
      sessionConfigurations = { ...sessionConfigurations, [sessionId]: await api.setMode(sessionId, mode) };
    } catch (error) {
      if (previousConfiguration) sessionConfigurations = { ...sessionConfigurations, [sessionId]: previousConfiguration };
      projects = projects.map((project) => ({ ...project, sessions: project.sessions.map((session) => session.id === sessionId ? { ...session, mode: previousMode } : session) }));
      showToast(errorMessage(error));
    }
  }

  async function loadSessionConfiguration(sessionId: string) {
    if (!sessionId || (api.isNative && !data?.environment.acpAvailable)) return;
    sessionConfigLoading = { ...sessionConfigLoading, [sessionId]: true };
    try {
      sessionConfigurations = { ...sessionConfigurations, [sessionId]: await api.sessionConfiguration(sessionId) };
    } catch (error) {
      showToast(errorMessage(error));
    } finally {
      sessionConfigLoading = { ...sessionConfigLoading, [sessionId]: false };
    }
  }

  async function setSessionConfig(configId: string, value: string) {
    if (!activeSessionId) return;
    const sessionId = activeSessionId;
    const previousConfiguration = sessionConfigurations[sessionId];
    setOptimisticConfig(sessionId, configId, value);
    try {
      sessionConfigurations = { ...sessionConfigurations, [sessionId]: await api.setSessionConfigOption(sessionId, configId, value) };
      const selected = sessionConfigurations[sessionId].options.find((option) => option.id === configId)?.options.find((option) => option.value === value)?.name || value;
      showToast(`${selected} selected`);
    } catch (error) {
      if (previousConfiguration) sessionConfigurations = { ...sessionConfigurations, [sessionId]: previousConfiguration };
      showToast(errorMessage(error));
    }
  }

  function setOptimisticConfig(sessionId: string, configId: string, value: string) {
    const current = sessionConfigurations[sessionId];
    if (!current) return;
    sessionConfigurations = { ...sessionConfigurations, [sessionId]: { ...current, options: current.options.map((option) => option.id === configId ? { ...option, currentValue: value } : option) } };
  }

  async function changeTheme(nextTheme: Theme) {
    const previousTheme = theme;
    theme = nextTheme;
    applyTheme(theme);
    if (data) data = { ...data, theme };
    try {
      await api.setTheme(theme);
      showToast(`${theme === "system" ? "System" : theme === "light" ? "Light" : "Dark"} appearance selected`);
    } catch (error) {
      theme = previousTheme;
      applyTheme(theme);
      if (data) data = { ...data, theme };
      showToast(errorMessage(error));
    }
  }

  async function changeAccentTheme(nextTheme: AccentTheme) {
    const previousTheme = accentTheme;
    accentTheme = nextTheme;
    applyAccentTheme(accentTheme);
    if (data) data = { ...data, accentTheme };
    try {
      await api.setAccentTheme(accentTheme);
      const label = accentTheme[0].toUpperCase() + accentTheme.slice(1);
      showToast(`${label} colour theme selected`);
    } catch (error) {
      accentTheme = previousTheme;
      applyAccentTheme(accentTheme);
      if (data) data = { ...data, accentTheme };
      showToast(errorMessage(error));
    }
  }

  async function beginAccountSetup() {
    if (!data) return;
    const wasConfigured = data.environment.account.configured;
    try {
      await api.openVibeSetup();
      showToast("Finish Mistral sign-in in Terminal");
      if (!wasConfigured) watchAccountSetup();
    } catch (error) {
      showToast(errorMessage(error));
    }
  }

  function watchAccountSetup() {
    if (accountPollTimer) window.clearInterval(accountPollTimer);
    accountPollAttempts = 0;
    accountPollTimer = window.setInterval(() => {
      accountPollAttempts += 1;
      void refreshAccount(true);
      if (accountPollAttempts >= 48 && accountPollTimer) {
        window.clearInterval(accountPollTimer);
        accountPollTimer = 0;
      }
    }, 2500);
  }

  async function refreshAccount(silent = false) {
    if (!data || accountRefreshing) return;
    accountRefreshing = true;
    try {
      const environment = await api.refreshEnvironment();
      const connected = !data.environment.account.configured && environment.account.configured;
      data = { ...data, environment };
      if (environment.account.configured && accountPollTimer) {
        window.clearInterval(accountPollTimer);
        accountPollTimer = 0;
      }
      if (connected) showToast("Mistral account is ready");
      else if (!silent) showToast(environment.account.configured ? "Mistral credential found" : "Finish setup in Terminal, then refresh");
    } catch (error) {
      if (!silent) showToast(errorMessage(error));
    } finally {
      accountRefreshing = false;
    }
  }

  function openAccount() {
    if (!data) return;
    if (data.environment.account.configured) openView("settings");
    else void beginAccountSetup();
  }

  function runComposerCommand(name: string) {
    switch (name) {
      case "mcp": openView("plugins"); break;
      case "libraries": openView("libraries"); break;
      case "skills": openView("skills"); break;
      case "settings": openView("settings"); break;
      case "new": void newConversation(); break;
      case "terminal": void openTerminal(); break;
      case "editor": void openCodeEditor(); break;
      case "vscodium": void openCodeEditor(); break;
    }
  }

  async function openMistralPage(destination: MistralDestination) {
    try {
      await api.openMistralPage(mistralLinks[destination]);
    } catch (error) {
      showToast(errorMessage(error));
    }
  }

  function applyTheme(preference: Theme) {
    const resolved = preference === "system"
      ? (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light")
      : preference;
    document.documentElement.dataset.theme = preference;
    document.documentElement.dataset.resolvedTheme = resolved;
    document.documentElement.style.colorScheme = resolved;
    void api.setResolvedTheme(resolved);
  }

  function normaliseTheme(value: string): Theme {
    return value === "light" || value === "system" ? value : "dark";
  }

  function applyAccentTheme(preference: AccentTheme) {
    document.documentElement.dataset.accentTheme = preference;
  }

  function normaliseAccentTheme(value: string): AccentTheme {
    return value === "tide" || value === "grove" || value === "cobalt" || value === "orchid" ? value : "mistral";
  }

  function handleStream(event: StreamEvent) {
    if (event.type === "conversation-title") {
      const title = event.text?.trim();
      if (title) {
        projects = projects.map((project) => ({
          ...project,
          sessions: project.sessions.map((session) => session.id === event.sessionId ? { ...session, title } : session)
        }));
      }
      return;
    }
    const current = streams[event.sessionId] || emptyStream();
    const activityEvent = event.type === "text" || event.type === "thought" || event.type === "tool-call" || event.type === "tool-update" || event.type === "plan" || event.type === "permission";
    let shouldDrainQueue = false;
    let next = { ...current, running: event.type === "complete" || event.type === "error" ? false : activityEvent ? true : current.running };
    if (activityEvent && next.startedAt === null) next.startedAt = Date.now();
    switch (event.type) {
      case "text": next.answer += event.text || ""; break;
      case "thought": next.thought += event.text || ""; break;
      case "tool-call": {
        const tool = event.data as ToolState;
        next.tools = [...next.tools.filter((item) => item.id !== tool.id), { ...tool, status: event.status || tool.status || "in_progress" }];
        break;
      }
      case "tool-update": next.tools = next.tools.map((tool) => tool.id === event.data?.id ? { ...tool, ...event.data, status: event.status || tool.status } : tool); break;
      case "plan": next.plan = (event.data?.entries || []) as PlanEntry[]; break;
      case "permission": next.permission = event.data as PermissionRequest; break;
      case "usage": next.usage = { used: Number(event.data?.used || 0), size: Number(event.data?.size || 0) }; break;
      case "config-options": {
        const configuration = sessionConfigurations[event.sessionId] || { options: [], commands: [] };
        sessionConfigurations = { ...sessionConfigurations, [event.sessionId]: { ...configuration, options: (event.data?.options || []) as SessionConfiguration["options"] } };
        break;
      }
      case "commands": {
        const configuration = sessionConfigurations[event.sessionId] || { options: [], commands: [] };
        sessionConfigurations = { ...sessionConfigurations, [event.sessionId]: { ...configuration, commands: (event.data?.commands || []) as SessionConfiguration["commands"] } };
        break;
      }
      case "mode": {
        const mode = String(event.data?.mode || "");
        if (mode) {
          setOptimisticConfig(event.sessionId, "mode", mode);
          projects = projects.map((project) => ({ ...project, sessions: project.sessions.map((session) => session.id === event.sessionId ? { ...session, mode } : session) }));
        }
        break;
      }
      case "message-saved": {
        const saved = event.data?.message as Message | undefined;
        const savedAnswer = saved?.content.trim() || "";
        const streamedAnswer = current.answer.trim();
        if (!current.running || saved?.role !== "assistant" || !streamedAnswer || savedAnswer !== streamedAnswer) return;
        if (activeSessionId === event.sessionId && !messages.some((message) => message.id === saved.id)) {
          messages = [...messages, saved];
        }
        next = { ...emptyStream(), usage: current.usage };
        shouldDrainQueue = true;
        break;
      }
      case "error":
        next.error = event.text || "Unknown ACP error";
        next.durationMs = Number(event.data?.durationMs || (current.startedAt ? Date.now() - current.startedAt : 0));
        shouldDrainQueue = true;
        break;
      case "complete": {
        const durationMs = Number(event.data?.durationMs || (current.startedAt ? Date.now() - current.startedAt : 0));
        if (current.answer && activeSessionId === event.sessionId) {
          messages = [...messages, { id: `local_${Date.now()}`, sessionId: event.sessionId, role: "assistant", kind: "text", content: current.answer, status: "complete", metadata: { durationMs }, createdAt: new Date().toISOString() }];
        }
        next = { ...emptyStream(), usage: current.usage };
        shouldDrainQueue = true;
        break;
      }
    }
    streams = { ...streams, [event.sessionId]: next };
    if (shouldDrainQueue) drainQueuedPrompt(event.sessionId);
  }

  async function respondPermission(requestId: string, optionId: string) {
    await api.respondPermission(requestId, optionId);
    const current = streams[activeSessionId] || emptyStream();
    streams = { ...streams, [activeSessionId]: { ...current, permission: null } };
  }

  async function toggleChanges() {
    if (activeProject?.kind !== "code") return;
    changesOpen = !changesOpen;
    if (changesOpen) await refreshChanges();
  }

  async function refreshChanges() {
    if (!activeProject) return;
    changesLoading = true;
    try { changes = await api.gitStatus(activeProject.id); } catch (error) { showToast(errorMessage(error)); }
    finally { changesLoading = false; }
  }

  async function openTerminal() {
    if (!activeProject || activeProject.kind !== "code") return;
    terminalOpen = !terminalOpen;
  }

  function resizeTerminal(height: number) {
    terminalHeight = Math.round(height);
    window.localStorage.setItem("vibe-terminal-height", String(terminalHeight));
  }

  async function openExternalTerminal() {
    if (!activeProject || activeProject.kind !== "code") return;
    try {
      await api.openProjectTerminal(activeProject.id);
      showToast(`Opened external Terminal in ${activeProject.name}`);
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function openCodeEditor() {
    if (!activeProject || activeProject.kind !== "code") return;
    await openProjectInCodeEditor(activeProject.id);
  }

  async function openProjectInCodeEditor(projectId: string) {
    const project = projects.find((item) => item.id === projectId);
    if (!project || project.kind !== "code") return;
    try {
      await api.openProjectInEditor(project.id);
      showToast(`Opened ${project.name} in ${codeEditor.name}`);
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function changeCodeEditor(nextEditorId: CodeEditor["id"]) {
    const nextEditor = editors.find((editor) => editor.id === nextEditorId);
    if (!nextEditor || !nextEditor.available) return;
    try {
      await api.setCodeEditor(nextEditorId);
      editorId = nextEditorId;
      if (data) data = { ...data, editor: nextEditorId };
      showToast(`${nextEditor.name} is now your code editor`);
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function copyContextText(text: string, label: string) {
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      showToast(`${label} copied`);
    } catch (error) {
      showToast(`Couldn’t copy ${label.toLowerCase()}: ${errorMessage(error)}`);
    }
  }

  async function handleSidebarContextAction(detail: { action: string; projectId: string; sessionId?: string; text?: string; label?: string }) {
    const project = projects.find((item) => item.id === detail.projectId);
    switch (detail.action) {
      case "new":
        await newConversation(detail.projectId);
        break;
      case "customize":
        if (project) openManagedProjectAppearance(project);
        break;
      case "terminal":
        if (project?.kind === "code") {
          await selectProject(project.id);
          terminalOpen = true;
        }
        break;
      case "editor":
        await openProjectInCodeEditor(detail.projectId);
        break;
      case "open":
        if (detail.sessionId) await selectSession(detail.projectId, detail.sessionId);
        break;
      case "copy":
        await copyContextText(detail.text || "", detail.label || "Text");
        break;
      case "remove": {
        const session = project?.sessions.find((item) => item.id === detail.sessionId);
        if (!project || !session) break;
        if (streams[session.id]?.running) {
          showToast("Stop Vibe before removing this conversation");
          break;
        }
        deletingConversation = { projectId: project.id, session };
        break;
      }
    }
  }

  async function savePlugin(plugin: Plugin) {
    try {
      const saved = await api.savePlugin(plugin);
      plugins = plugins.some((item) => item.id === saved.id) ? plugins.map((item) => item.id === saved.id ? saved : item) : [saved, ...plugins];
      showToast(`${saved.name} saved`);
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function refreshMCPInventory() {
    mcpLoading = true;
    try {
      mcpInventory = await api.mcpInventory(activeProjectId);
    } catch (error) {
      showToast(errorMessage(error));
    } finally {
      mcpLoading = false;
    }
  }

  async function setConnectorEnabled(name: string, enabled: boolean) {
    const source = mcpInventory.sources.find((item) => item.kind === "connector" && item.name === name);
    connectorUpdating = name;
    try {
      mcpInventory = await api.setConnectorEnabled(activeProjectId, name, enabled);
      showToast(`${source?.displayName || name} ${enabled ? "enabled in Vibe" : "disabled in Vibe"}`);
    } catch (error) {
      showToast(errorMessage(error));
    } finally {
      connectorUpdating = "";
    }
  }

  async function refreshSkills(silent = false) {
    skillsLoading = true;
    try {
      skillInventory = await api.skills(activeProjectId);
    } catch (error) {
      if (!silent) showToast(errorMessage(error));
    } finally {
      skillsLoading = false;
    }
  }

  async function activateSkillChanges(message: string) {
    if (Object.values(streams).some((stream) => stream.running)) {
      showToast(`${message} · reload after the active task finishes`);
      return;
    }
    try {
      await api.reloadSkills();
      if (activeSessionId) await loadSessionConfiguration(activeSessionId);
      showToast(message);
    } catch (error) {
      showToast(errorMessage(error));
    }
  }

  async function saveSkill(skill: Skill) {
    try {
      const saved = await api.saveSkill(skill);
      await refreshSkills(true);
      await activateSkillChanges(`${saved.name} saved`);
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function toggleSkill(skill: Skill, enabled: boolean) {
    try {
      await api.setSkillEnabled(skill.scope, skill.scope === "project" ? skill.projectId || activeProjectId : "", skill.name, enabled);
      skillInventory = { ...skillInventory, skills: skillInventory.skills.map((item) => item.scope === skill.scope && item.name === skill.name ? { ...item, enabled } : item) };
      await activateSkillChanges(`${skill.name} ${enabled ? "enabled" : "disabled"}`);
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function deleteSkill(skill: Skill) {
    try {
      await api.deleteSkill(skill.scope, skill.scope === "project" ? skill.projectId || activeProjectId : "", skill.name);
      await refreshSkills(true);
      await activateSkillChanges(`${skill.name} deleted`);
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function importSkill(scope: Skill["scope"]) {
    try {
      const sourcePath = await api.pickSkillFolder();
      if (!sourcePath) return;
      const imported = await api.importSkill(sourcePath, scope, scope === "project" ? activeProjectId : "");
      await refreshSkills(true);
      await activateSkillChanges(`${imported.name} imported`);
    } catch (error) { showToast(errorMessage(error)); }
  }

  async function reloadSkills() {
    if (anyStreamRunning) return;
    skillsLoading = true;
    try {
      await api.reloadSkills();
      skillInventory = await api.skills(activeProjectId);
      if (activeSessionId) await loadSessionConfiguration(activeSessionId);
      showToast("Vibe reloaded Skills and slash commands");
    } catch (error) {
      showToast(errorMessage(error));
    } finally {
      skillsLoading = false;
    }
  }

  function openView(nextView: "chat" | "libraries" | "skills" | "plugins" | "settings") {
    view = nextView;
    changesOpen = false;
    if (nextView === "plugins") void refreshMCPInventory();
    if (nextView === "skills") void refreshSkills();
  }

  function keyboardShortcuts(event: KeyboardEvent) {
    if (!(event.metaKey || event.ctrlKey)) {
      if (event.key === "Escape") {
        paletteOpen = false;
        managedProjectModal = false;
        if (!deletingConversationBusy) deletingConversation = null;
      }
      return;
    }
    if (event.key.toLowerCase() === "n") { event.preventDefault(); void newConversation(); }
    if (event.key.toLowerCase() === "b") { event.preventDefault(); toggleSidebar(); }
    if (event.key.toLowerCase() === "k" || (event.shiftKey && event.key.toLowerCase() === "p")) { event.preventDefault(); paletteOpen = true; paletteQuery = ""; }
    if (event.key === ",") { event.preventDefault(); openView("plugins"); }
  }

  function toggleSidebar() {
    sidebarCollapsed = !sidebarCollapsed;
    window.localStorage.setItem("vibe-sidebar-collapsed", String(sidebarCollapsed));
  }

  function handleViewportScroll() {
    if (!messageViewport) return;
    stickToBottom = messageViewport.scrollHeight - messageViewport.scrollTop - messageViewport.clientHeight < 120;
  }

  function showToast(message: string) {
    toast = message;
    window.setTimeout(() => { if (toast === message) toast = ""; }, 2800);
  }

  function emptyStream(): StreamState {
    return { running: false, startedAt: null, durationMs: 0, activePrompt: "", answer: "", thought: "", tools: [], plan: [], permission: null, error: "", usage: null };
  }

  function errorMessage(error: unknown) {
    return error instanceof Error ? error.message : String(error);
  }
</script>

{#if loading}
  <div class="app-loading"><div class="brand-mark large"><VibeDockMark size={28} /></div><LoaderCircle class="spin" size={18} /><span>Opening VibeDock…</span></div>
{:else if fatalError || !data}
  <div class="fatal-state"><div class="error-orbit"><X size={22} /></div><h1>VibeDock couldn’t start</h1><p>{fatalError}</p><button class="primary-button" on:click={() => location.reload()}><RotateCcw size={14} /> Try again</button></div>
{:else}
  <div class:with-panel={changesOpen && view === "chat"} class:sidebar-collapsed={sidebarCollapsed} class="app-shell">
    <div class="window-dragbar"></div>
    <Sidebar {projects} {activeProjectId} {activeSessionId} {view} {workspaceKind} {codeEditor} {runningSessionIds} collapsed={sidebarCollapsed} environment={data.environment}
      on:collapse={toggleSidebar}
      on:workspace={(event) => switchWorkspace(event.detail.kind)}
      on:selectProject={(event) => selectProject(event.detail.projectId)}
      on:selectSession={(event) => selectSession(event.detail.projectId, event.detail.sessionId)}
      on:newConversation={(event) => newConversation(event.detail?.projectId)}
      on:addProject={addProject}
      on:customize={(event) => openManagedProjectAppearance(event.detail.project)}
      on:contextAction={(event) => handleSidebarContextAction(event.detail)}
      on:view={(event) => openView(event.detail.view)}
      on:account={openAccount} />

    <main class="main-surface">
      <div class="future-ambient" aria-hidden="true">
        <span class="ambient-orb ambient-orb-one"></span>
        <span class="ambient-orb ambient-orb-two"></span>
        <span class="ambient-grid"></span>
        <span class="ambient-beam"></span>
      </div>
      {#if view === "libraries"}
        <LibrariesView {libraries} busy={libraryBusy}
          on:create={(event) => createLibrary(event.detail.name, event.detail.description)}
          on:delete={(event) => deleteLibrary(event.detail.libraryId)}
          on:upload={(event) => uploadLibraryDocuments(event.detail.libraryId)}
          on:webpage={(event) => addLibraryWebpage(event.detail.libraryId, event.detail.url)}
          on:deleteDocument={(event) => deleteLibraryDocument(event.detail.libraryId, event.detail.documentId)}
          on:external={(event) => openMistralPage(event.detail.destination)} />
      {:else if view === "skills"}
        <SkillsView inventory={skillInventory} project={activeProject} loading={skillsLoading} runtimeBusy={anyStreamRunning}
          on:refresh={() => refreshSkills()}
          on:reload={reloadSkills}
          on:save={(event) => saveSkill(event.detail.skill)}
          on:toggle={(event) => toggleSkill(event.detail.skill, event.detail.enabled)}
          on:delete={(event) => deleteSkill(event.detail.skill)}
          on:import={(event) => importSkill(event.detail.scope)} />
      {:else if view === "plugins"}
        <PluginsView {plugins} inventory={mcpInventory} loading={mcpLoading} {connectorUpdating} on:save={(event) => savePlugin(event.detail.plugin)} on:refresh={refreshMCPInventory} on:toggleConnector={(event) => setConnectorEnabled(event.detail.name, event.detail.enabled)} on:external={(event) => openMistralPage(event.detail.destination)} />
      {:else if view === "settings"}
        <SettingsView environment={data.environment} {theme} {accentTheme} {accountRefreshing} {editors} {codeEditor} on:theme={(event) => changeTheme(event.detail.theme)} on:accent={(event) => changeAccentTheme(event.detail.theme)} on:editor={(event) => changeCodeEditor(event.detail.editorId)} on:setup={beginAccountSetup} on:refreshAccount={() => refreshAccount()} on:external={(event) => openMistralPage(event.detail.destination)} />
      {:else if activeProject && activeConversation}
        <div class:terminal-open={terminalOpen && activeProject.kind === "code"} class="chat-surface" style={`--terminal-height:${terminalHeight}px`}>
          <ChatHeader project={activeProject} conversation={activeConversation} stream={activeStream} {changesOpen} {terminalOpen} {codeEditor} on:changes={toggleChanges} on:terminal={openTerminal} on:editor={openCodeEditor} on:palette={() => paletteOpen = true} />
          <div class="messages-viewport" bind:this={messageViewport} on:scroll={handleViewportScroll}>
            <div class="chat-column">
              {#if messages.length === 0 && !activeStream.running}
                {#if activeProject.kind === "chat"}
                  <div class="conversation-welcome"><span class="welcome-icon chat"><Lightbulb size={18} /></span><h2>What’s on your mind?</h2><p>Think through an idea, ask a question, or use your configured connectors without opening a codebase.</p><div class="prompt-suggestions"><button on:click={() => send("Help me think through an idea and identify the key decisions.")}><Lightbulb size={15} /><span>Explore an idea</span></button><button on:click={() => send("Help me compare the options I’m considering.")}><Scale size={15} /><span>Compare options</span></button><button on:click={() => send("Turn my rough thoughts into a clear plan.")}><ListChecks size={15} /><span>Make a plan</span></button></div></div>
                {:else if activeProject.kind === "work"}
                  <div class="conversation-welcome"><span class="welcome-icon work"><BriefcaseBusiness size={18} /></span><h2>What outcome should Vibe deliver?</h2><p>Work can combine Libraries, connectors, files, and the web for longer multi-step tasks with visible progress.</p><div class="prompt-suggestions"><button on:click={() => send("Research this topic using my attached Libraries and create a concise brief.")}><FileSearch size={15} /><span>Create a brief</span></button><button on:click={() => send("Compare the attached source material and highlight the important differences.")}><GitCompareArrows size={15} /><span>Compare sources</span></button><button on:click={() => send("Turn this context into an actionable plan with owners and next steps.")}><ListChecks size={15} /><span>Build an action plan</span></button></div><button class="online-work-link" on:click={() => openMistralPage("work")}>Open cloud Work <span>↗</span></button></div>
                {:else}
                  <div class="conversation-welcome"><span class="welcome-icon code"><Code2 size={18} /></span><h2>What should we work on?</h2><p>Vibe can inspect the project, edit files, run commands, and explain its work.</p><div class="prompt-suggestions"><button on:click={() => send("Review this project and suggest the highest-impact improvements.")}><SearchCode size={15} /><span>Review this project</span></button><button on:click={() => send("Find and fix the most important bug in this codebase.")}><Bug size={15} /><span>Find a bug to fix</span></button><button on:click={() => send("Explain how this project is structured.")}><Network size={15} /><span>Explain the architecture</span></button></div></div>
                {/if}
              {/if}
              <Messages {messages} stream={activeStream} on:permission={(event) => respondPermission(event.detail.requestId, event.detail.optionId)} />
            </div>
          </div>
          <Composer mode={activeConversation.mode} projectKind={activeProject.kind} {libraries} commands={activeConfiguration.commands} editorName={codeEditor.name} attachedLibraryIds={activeConversation.libraryIds || []} running={activeStream.running} activePrompt={activeStream.activePrompt} queuedPrompts={queuedPrompts[activeSessionId] || []} acpAvailable={data.environment.acpAvailable || !api.isNative} configOptions={activeConfiguration.options} configLoading={sessionConfigLoading[activeSessionId] || false} on:send={(event) => send(event.detail.text)} on:removeQueued={(event) => removeQueuedPrompt(activeSessionId, event.detail.index)} on:command={(event) => runComposerCommand(event.detail.name)} on:cancel={cancel} on:mode={(event) => setMode(event.detail.mode)} on:config={(event) => setSessionConfig(event.detail.configId, event.detail.value)} on:libraries={(event) => setConversationLibraries(event.detail.libraryIds)} on:manageLibraries={() => openView("libraries")} />
          {#if terminalOpen && activeProject.kind === "code"}
            {#key activeProject.id}
              <TerminalPanel project={activeProject} height={terminalHeight} on:resize={(event) => resizeTerminal(event.detail.height)} on:close={() => terminalOpen = false} on:external={openExternalTerminal} />
            {/key}
          {/if}
        </div>
      {:else if activeProject}
        <div class="project-empty">
          <ProjectAvatar name={activeProject.name} icon={activeProject.icon || ""} color={activeProject.color} size="large" />
          <span class="page-kicker">{activeProject.kind === "code" ? activeProject.path : `${activeProject.kind} project`}</span><h1>{activeProject.name}</h1><p>{activeProject.kind === "chat" ? "Keep related conversations together without attaching a code folder." : activeProject.kind === "work" ? "Group multi-step tasks that reuse Libraries, connectors, and source material." : "Start a conversation to explore, build, or fix this project with Vibe."}</p>
          <button class="primary-button" on:click={() => newConversation()}><MessageSquarePlus size={16} /> {activeProject.kind === "chat" ? "New chat" : activeProject.kind === "work" ? "New task" : "New conversation"}</button>
        </div>
      {:else}
        {#if workspaceKind === "chat"}
          <div class="project-empty first-project"><div class="empty-project-glyph neutral"><MessagesSquare size={24} /></div><span class="page-kicker">Chat with Vibe</span><h1>Create a chat project</h1><p>Group related conversations without giving them access to one of your code folders.</p><button class="primary-button" on:click={addProject}><MessagesSquare size={16} /> New chat project</button></div>
        {:else if workspaceKind === "work"}
          <div class="project-empty first-project"><div class="empty-project-glyph neutral"><BriefcaseBusiness size={24} /></div><span class="page-kicker">Work with Vibe</span><h1>Create a work project</h1><p>Group longer tasks and reuse Libraries, connectors, and source material across conversations.</p><button class="primary-button" on:click={addProject}><BriefcaseBusiness size={16} /> New work project</button></div>
        {:else}
          <div class="project-empty first-project"><div class="empty-project-glyph neutral"><FolderPlus size={24} /></div><span class="page-kicker">Code with Vibe</span><h1>Bring in a project</h1><p>Add a local folder. Vibe will work inside that project and keep each conversation separate.</p><button class="primary-button" on:click={addProject}><FolderPlus size={16} /> Add code project</button></div>
        {/if}
      {/if}
    </main>

    {#if changesOpen && view === "chat" && activeProject?.kind === "code"}<ChangesPanel project={activeProject} files={changes} loading={changesLoading} on:close={() => changesOpen = false} on:refresh={refreshChanges} />{/if}
  </div>
{/if}

{#if paletteOpen}
  <div class="modal-backdrop palette-backdrop" role="presentation" on:click={(event) => event.target === event.currentTarget && (paletteOpen = false)}>
    <div class="command-palette">
      <label><Search size={17} /><input bind:this={paletteInput} bind:value={paletteQuery} placeholder="Jump to a conversation…" /><kbd>esc</kbd></label>
      <div class="palette-results">
        {#each paletteResults as result (result.session.id)}
          <button on:click={() => { selectSession(result.project.id, result.session.id); paletteOpen = false; }}><ProjectAvatar name={result.project.name} icon={result.project.icon || ""} color={result.project.color} size="small" /><div><strong>{result.session.title}</strong><span>{result.project.name} · {result.session.preview || "No messages yet"}</span></div></button>
        {/each}
        {#if paletteResults.length === 0}<div class="palette-empty">No conversations match “{paletteQuery}”</div>{/if}
      </div>
      <div class="palette-foot"><span>↵ open</span><span>⌘N new conversation</span></div>
    </div>
  </div>
{/if}

{#if managedProjectModal}
  <div class="modal-backdrop" role="presentation" on:click={(event) => event.target === event.currentTarget && (managedProjectModal = false)}>
    <form class="chat-project-dialog" on:submit|preventDefault={submitManagedProject}>
      <div class="modal-head"><div><span class="page-kicker">{managedProjectKind === "work" ? "Work workspace" : "Chat workspace"}</span><h2>{managedProjectEditId ? `Customize ${managedProjectName}` : `New ${managedProjectKind} project`}</h2></div><button type="button" class="icon-button" title="Close" on:click={() => managedProjectModal = false}><X size={17} /></button></div>
      {#if !managedProjectEditId}<label><span>Name</span><input bind:this={managedProjectInput} bind:value={managedProjectName} maxlength="120" placeholder={managedProjectKind === "work" ? "Market launch, Client research…" : "Ideas, Research, Planning…"} /></label>{/if}
      <ProjectAppearancePicker name={managedProjectName} icon={managedProjectIcon} color={managedProjectColor} on:change={(event) => { managedProjectIcon = event.detail.icon; managedProjectColor = event.detail.color; }} />
      <p>{managedProjectEditId ? "Choose an icon and color that make this group easy to spot." : managedProjectKind === "work" ? "Work projects group longer tasks with reusable Libraries and connectors. Vibe uses a private app-managed workspace behind the scenes." : "Chat projects group quick conversations without attaching a code folder. Vibe uses a private app-managed workspace behind the scenes."}</p>
      <div class="modal-actions"><button type="button" class="secondary-button" on:click={() => managedProjectModal = false}>Cancel</button><button class="primary-button" disabled={!managedProjectName.trim()}>{#if managedProjectEditId}<Sparkles size={14} /> Save appearance{:else if managedProjectKind === "work"}<BriefcaseBusiness size={14} /> Create project{:else}<MessagesSquare size={14} /> Create project{/if}</button></div>
    </form>
  </div>
{/if}

{#if deletingConversation}
  <div class="modal-backdrop" role="presentation" on:click={(event) => event.target === event.currentTarget && !deletingConversationBusy && (deletingConversation = null)}>
    <div class="chat-project-dialog confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="remove-conversation-title">
      <div class="modal-head"><div><span class="page-kicker">Remove conversation</span><h2 id="remove-conversation-title">Remove “{deletingConversation.session.title}”?</h2></div><button type="button" class="icon-button" title="Close" disabled={deletingConversationBusy} on:click={() => deletingConversation = null}><X size={17} /></button></div>
      <p>This permanently removes the conversation, its messages, and its Library attachments from this app. This cannot be undone.</p>
      <div class="modal-actions"><button type="button" class="secondary-button" disabled={deletingConversationBusy} on:click={() => deletingConversation = null}>Cancel</button><button type="button" class="danger-button" disabled={deletingConversationBusy} on:click={removeConversation}>{#if deletingConversationBusy}<LoaderCircle class="spin" size={14} /> Removing…{:else}<Trash2 size={14} /> Remove conversation{/if}</button></div>
    </div>
  </div>
{/if}

{#if toast}<div class="toast"><span class="status-dot"></span>{toast}</div>{/if}
