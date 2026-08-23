<script lang="ts">
  import { createEventDispatcher, onMount, tick } from "svelte";
  import { BriefcaseBusiness, Check, ChevronDown, ChevronRight, Clipboard, Code2, FolderPlus, LibraryBig, LogIn, MessageCircle, MessageSquarePlus, MessagesSquare, Paintbrush, PanelLeftClose, PanelLeftOpen, Plug, Search, Settings, SquareTerminal, Trash2, WandSparkles } from "@lucide/svelte";
  import type { CodeEditor, Conversation, Environment, Project, WorkspaceKind } from "../lib/types";
  import BrandIcon from "./BrandIcon.svelte";
  import ProjectAvatar from "./ProjectAvatar.svelte";
  import VibeDockMark from "./VibeDockMark.svelte";

  export let projects: Project[] = [];
  export let activeProjectId = "";
  export let activeSessionId = "";
  export let view = "chat";
  export let environment: Environment;
  export let workspaceKind: WorkspaceKind = "code";
  export let collapsed = false;
  export let codeEditor: CodeEditor;
  export let runningSessionIds: Set<string> = new Set();

  const dispatch = createEventDispatcher();
  let query = "";
  let expanded = new Set<string>();
  let contextMenu: { kind: "project" | "session"; project: Project; session?: Conversation } | null = null;
  let contextMenuElement: HTMLDivElement;
  let menuX = 0;
  let menuY = 0;

  $: if (activeProjectId && !expanded.has(activeProjectId)) expanded = new Set([...expanded, activeProjectId]);
  $: workspaceProjects = projects.filter((project) => project.kind === workspaceKind);
  $: filtered = workspaceProjects.filter((project) => project.name.toLowerCase().includes(query.toLowerCase()));
  $: runningProjectIds = new Set(projects.filter((project) => project.sessions.some((session) => runningSessionIds.has(session.id))).map((project) => project.id));

  function toggle(projectId: string) {
    const next = new Set(expanded);
    next.has(projectId) ? next.delete(projectId) : next.add(projectId);
    expanded = next;
  }

  function workspaceLabel(kind: WorkspaceKind) {
    return kind === "work" ? "Work" : kind === "chat" ? "Chat" : "Code";
  }

  function openContextMenu(event: MouseEvent, project: Project, session?: Conversation) {
    menuX = event.clientX;
    menuY = event.clientY;
    contextMenu = { kind: session ? "session" : "project", project, session };
    void tick().then(() => {
      if (!contextMenuElement) return;
      const bounds = contextMenuElement.getBoundingClientRect();
      menuX = Math.max(8, Math.min(menuX, window.innerWidth - bounds.width - 8));
      menuY = Math.max(8, Math.min(menuY, window.innerHeight - bounds.height - 8));
      contextMenuElement.querySelector<HTMLButtonElement>("[role='menuitem']")?.focus();
    });
  }

  function closeContextMenu() {
    contextMenu = null;
  }

  function runContextAction(action: string, detail: Record<string, string> = {}) {
    if (!contextMenu) return;
    const project = contextMenu.project;
    const session = contextMenu.session;
    closeContextMenu();
    dispatch("contextAction", { action, projectId: project.id, sessionId: session?.id || "", ...detail });
  }

  function handleMenuKeydown(event: KeyboardEvent) {
    if (!contextMenuElement) return;
    const items = Array.from(contextMenuElement.querySelectorAll<HTMLButtonElement>("[role='menuitem']"));
    const current = items.indexOf(document.activeElement as HTMLButtonElement);
    if (event.key === "ArrowDown") {
      event.preventDefault();
      items[(current + 1 + items.length) % items.length]?.focus();
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      items[(current - 1 + items.length) % items.length]?.focus();
    } else if (event.key === "Home") {
      event.preventDefault();
      items[0]?.focus();
    } else if (event.key === "End") {
      event.preventDefault();
      items.at(-1)?.focus();
    } else if (event.key === "Escape") {
      event.preventDefault();
      closeContextMenu();
    }
  }

  onMount(() => {
    const dismissOutside = (event: PointerEvent) => {
      if (contextMenuElement && !contextMenuElement.contains(event.target as Node)) closeContextMenu();
    };
    const dismissOnResize = () => closeContextMenu();
    const dismissOnScroll = () => closeContextMenu();
    window.addEventListener("pointerdown", dismissOutside);
    window.addEventListener("resize", dismissOnResize);
    window.addEventListener("scroll", dismissOnScroll, true);
    return () => {
      window.removeEventListener("pointerdown", dismissOutside);
      window.removeEventListener("resize", dismissOnResize);
      window.removeEventListener("scroll", dismissOnScroll, true);
    };
  });
</script>

<aside class:collapsed class="sidebar">
  <div class="traffic-space"></div>
  <div class="sidebar-head">
    <div class="brand-row">
      <div class="brand-mark"><VibeDockMark size={16} /></div>
      <span>VibeDock</span>
      <button class="sidebar-collapse" title={collapsed ? "Expand sidebar (⌘B)" : "Collapse sidebar (⌘B)"} aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"} aria-pressed={collapsed} on:click={() => dispatch("collapse")}>
        {#if collapsed}<PanelLeftOpen size={14} />{:else}<PanelLeftClose size={14} />{/if}
      </button>
    </div>
    <div class="workspace-switch" aria-label="Workspace">
      <button title="Chat" class:active={workspaceKind === "chat"} aria-pressed={workspaceKind === "chat"} on:click={() => dispatch("workspace", { kind: "chat" })}><MessageCircle size={13} /><span>Chat</span></button>
      <button title="Work" class:active={workspaceKind === "work"} aria-pressed={workspaceKind === "work"} on:click={() => dispatch("workspace", { kind: "work" })}><BriefcaseBusiness size={13} /><span>Work</span></button>
      <button title="Code" class:active={workspaceKind === "code"} aria-pressed={workspaceKind === "code"} on:click={() => dispatch("workspace", { kind: "code" })}><Code2 size={13} /><span>Code</span></button>
    </div>
    <button class="new-chat" title={workspaceKind === "chat" ? "New chat" : workspaceKind === "work" ? "New task" : "New conversation"} on:click={() => dispatch("newConversation")} disabled={workspaceKind === "code" && !activeProjectId}>
      <MessageSquarePlus size={15} />
      <span>{workspaceKind === "chat" ? "New chat" : workspaceKind === "work" ? "New task" : "New conversation"}</span>
      <kbd>⌘N</kbd>
    </button>
    {#if collapsed}
      <button type="button" class="search-box collapsed-search" title="Expand to search projects" aria-label="Expand to search projects" on:click={() => dispatch("collapse")}><Search size={14} /></button>
    {:else}
      <label class="search-box" title="Search projects">
        <Search size={14} />
        <input bind:value={query} placeholder={`Search ${workspaceLabel(workspaceKind).toLowerCase()} projects`} aria-label="Search projects" />
        <kbd>⌘K</kbd>
      </label>
    {/if}
  </div>

  <div class="project-scroll">
    <div class="section-label">
      <span>{workspaceLabel(workspaceKind)} projects</span>
      <button class="icon-button tiny" title={workspaceKind === "code" ? "Add folder" : `New ${workspaceLabel(workspaceKind).toLowerCase()} project`} on:click={() => dispatch("addProject")}>
        {#if workspaceKind === "code"}<FolderPlus size={14} />{:else if workspaceKind === "work"}<BriefcaseBusiness size={14} />{:else}<MessagesSquare size={14} />{/if}
      </button>
    </div>
    {#if filtered.length === 0}
      <button class="empty-projects" on:click={() => dispatch("addProject")}>
        {#if workspaceKind === "code"}<FolderPlus size={18} />{:else if workspaceKind === "work"}<BriefcaseBusiness size={18} />{:else}<MessagesSquare size={18} />{/if}
        <span>{workspaceProjects.length ? "No matching projects" : workspaceKind === "code" ? "Add your first code project" : `Create your first ${workspaceLabel(workspaceKind).toLowerCase()} project`}</span>
      </button>
    {/if}
    {#each filtered as project (project.id)}
      <div class="project-group">
        <div class:active={project.id === activeProjectId && view === "chat"} class="project-row" role="group" aria-label={`${project.name} project`} on:contextmenu|preventDefault|stopPropagation={(event) => openContextMenu(event, project)}>
          <button class="disclosure" title="Toggle conversations" on:click={() => toggle(project.id)}>
            {#if expanded.has(project.id)}<ChevronDown size={13} />{:else}<ChevronRight size={13} />{/if}
          </button>
          <button class="project-main" title={project.name} on:click={() => dispatch("selectProject", { projectId: project.id })}>
            <ProjectAvatar name={project.name} icon={project.icon || ""} color={project.color} />
            <span class="project-name">{project.name}</span>
          </button>
          {#if runningProjectIds.has(project.id)}<span class="project-running-indicator" role="status" aria-label={`Vibe is working in ${project.name}`}><i></i></span>{/if}
          {#if project.kind !== "code"}<button class="project-customize" title="Customize group" aria-label={`Customize ${project.name}`} on:click={() => dispatch("customize", { project })}><Paintbrush size={12} /></button>{/if}
          <button class="project-add" title="New conversation" aria-label="New conversation" on:click={() => dispatch("newConversation", { projectId: project.id })}><MessageSquarePlus size={13} /></button>
        </div>
        {#if expanded.has(project.id)}
          <div class="session-list">
            {#each project.sessions as session (session.id)}
              <button class:active={session.id === activeSessionId && view === "chat"} class="session-row" on:click={() => dispatch("selectSession", { projectId: project.id, sessionId: session.id })} on:contextmenu|preventDefault|stopPropagation={(event) => openContextMenu(event, project, session)}>
                <MessageCircle class="session-icon" size={11} />
                <span class="session-title">{session.title}</span>
                {#if runningSessionIds.has(session.id)}<span class="sidebar-thinking-dots" role="status" aria-label="Vibe is working"><i></i><i></i><i></i></span>{/if}
                {#if session.mode === "plan"}<span class="mode-dot" title="Plan mode"></span>{/if}
              </button>
            {/each}
            {#if project.sessions.length === 0}<div class="no-sessions">No conversations yet</div>{/if}
          </div>
        {/if}
      </div>
    {/each}
  </div>

  <div class="sidebar-foot">
    <button title="Libraries" class:active={view === "libraries"} class="nav-row" on:click={() => dispatch("view", { view: "libraries" })}>
      <LibraryBig size={15} /><span>Libraries</span>
    </button>
    <button title="Skills" class:active={view === "skills"} class="nav-row" on:click={() => dispatch("view", { view: "skills" })}>
      <WandSparkles size={15} /><span>Skills</span>
    </button>
    <button title="MCP & connectors" class:active={view === "plugins"} class="nav-row" on:click={() => dispatch("view", { view: "plugins" })}>
      <Plug size={15} /><span>MCP &amp; connectors</span><kbd>⌘,</kbd>
    </button>
    <button title="Settings" class:active={view === "settings"} class="nav-row" on:click={() => dispatch("view", { view: "settings" })}>
      <Settings size={15} /><span>Settings</span>
    </button>
    <button title="Mistral account" class:configured={environment.account.configured} class="account-nav" on:click={() => dispatch("account")}>
      <span class="account-logo"><BrandIcon name="Mistral AI" size={15} /></span>
      <span class="account-copy"><strong>Mistral account</strong><span>{environment.account.configured ? "Vibe credential ready" : "Sign in or add an API key"}</span></span>
      {#if environment.account.configured}<Check size={13} />{:else}<LogIn size={13} />{/if}
    </button>
    <div class="runtime-state" title={environment.acpPath || "vibe-acp not found"}>
      <span class:offline={!environment.acpAvailable} class="status-dot"></span>
      <span>{environment.acpAvailable ? environment.vibeVersion || "Vibe connected" : "Vibe setup needed"}</span>
    </div>
  </div>
</aside>

{#if contextMenu}
  <div bind:this={contextMenuElement} class="context-menu" role="menu" tabindex="-1" aria-label={contextMenu.kind === "project" ? `${contextMenu.project.name} project actions` : `${contextMenu.session?.title} conversation actions`} style={`left:${menuX}px;top:${menuY}px`} on:keydown={handleMenuKeydown}>
    <div class="context-menu-heading">
      {#if contextMenu.kind === "project"}
        <ProjectAvatar name={contextMenu.project.name} icon={contextMenu.project.icon || ""} color={contextMenu.project.color} size="small" />
        <span>{contextMenu.project.name}</span>
      {:else}
        <span class="context-menu-heading-icon"><MessageCircle size={13} /></span>
        <span>{contextMenu.session?.title}</span>
      {/if}
    </div>
    <div class="context-menu-separator"></div>
    {#if contextMenu.kind === "project"}
      <button role="menuitem" on:click={() => runContextAction("new")}><MessageSquarePlus size={14} /><span>{contextMenu.project.kind === "chat" ? "New chat" : contextMenu.project.kind === "work" ? "New task" : "New conversation"}</span></button>
      {#if contextMenu.project.kind !== "code"}
        <button role="menuitem" on:click={() => runContextAction("customize")}><Paintbrush size={14} /><span>Customize appearance</span></button>
      {:else}
        <button role="menuitem" on:click={() => runContextAction("terminal")}><SquareTerminal size={14} /><span>Open integrated terminal</span></button>
        <button role="menuitem" on:click={() => runContextAction("editor")}><BrandIcon name={codeEditor.icon} size={14} /><span>Open in {codeEditor.name}</span></button>
      {/if}
      <div class="context-menu-separator"></div>
      <button role="menuitem" on:click={() => runContextAction("copy", { text: contextMenu?.project.kind === "code" ? contextMenu.project.path : contextMenu?.project.name || "", label: contextMenu?.project.kind === "code" ? "Project path" : "Group name" })}><Clipboard size={14} /><span>{contextMenu.project.kind === "code" ? "Copy project path" : "Copy group name"}</span><kbd>⌘C</kbd></button>
    {:else}
      <button role="menuitem" on:click={() => runContextAction("open")}><MessageCircle size={14} /><span>Open conversation</span></button>
      <button role="menuitem" on:click={() => runContextAction("new")}><MessageSquarePlus size={14} /><span>{contextMenu.project.kind === "chat" ? "New chat in group" : contextMenu.project.kind === "work" ? "New task in project" : "New conversation in project"}</span></button>
      <div class="context-menu-separator"></div>
      <button role="menuitem" on:click={() => runContextAction("copy", { text: contextMenu?.session?.title || "", label: "Conversation title" })}><Clipboard size={14} /><span>Copy title</span><kbd>⌘C</kbd></button>
      <div class="context-menu-separator"></div>
      <button class="danger" role="menuitem" on:click={() => runContextAction("remove")}><Trash2 size={14} /><span>Remove conversation…</span></button>
    {/if}
  </div>
{/if}
