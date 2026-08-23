import type { Bootstrap, ChangedFile, MCPInventory, Message, SessionConfiguration, SkillInventory } from "./types";

const now = new Date().toISOString();
const yesterday = new Date(Date.now() - 86_400_000).toISOString();

export const demoBootstrap: Bootstrap = {
  projects: [
    {
      id: "prj_vibe",
      name: "vibe-app",
      path: "/Users/kevin/Repositories/home/vibe-app",
      kind: "code",
      color: "#ff7417",
      pinned: true,
      lastOpened: now,
      createdAt: yesterday,
      sessions: [
        { id: "ses_shell", projectId: "prj_vibe", title: "Build the native app shell", mode: "default", preview: "The project shell and persistence layer are ready.", updatedAt: now, createdAt: yesterday, libraryIds: ["lib_product"] },
        { id: "ses_acp", projectId: "prj_vibe", title: "Wire Vibe ACP streaming", mode: "plan", preview: "Map protocol updates into focused UI events.", updatedAt: yesterday, createdAt: yesterday, libraryIds: [] },
        { id: "ses_design", projectId: "prj_vibe", title: "Polish the project sidebar", mode: "default", preview: "Tighten spacing and project navigation.", updatedAt: yesterday, createdAt: yesterday, libraryIds: [] }
      ]
    },
    {
      id: "prj_atlas", name: "atlas-api", path: "/Users/kevin/Repositories/atlas-api", kind: "code", color: "#4fa78f", pinned: false, lastOpened: yesterday, createdAt: yesterday,
      sessions: [{ id: "ses_atlas", projectId: "prj_atlas", title: "Fix flaky integration tests", mode: "default", preview: "Inspect the database fixture lifecycle.", updatedAt: yesterday, createdAt: yesterday, libraryIds: [] }]
    },
    {
      id: "prj_notes", name: "field-notes", path: "/Users/kevin/Repositories/field-notes", kind: "code", color: "#db7d62", pinned: false, lastOpened: yesterday, createdAt: yesterday, sessions: []
    },
    {
      id: "prj_ideas", name: "Ideas", path: "/private/vibe-chat/ideas", kind: "chat", icon: "lightbulb", color: "#e86b2f", pinned: false, lastOpened: yesterday, createdAt: yesterday,
      sessions: [{ id: "ses_ideas", projectId: "prj_ideas", title: "Explore a product idea", mode: "default", preview: "Let’s shape the idea and decide what to do next.", updatedAt: yesterday, createdAt: yesterday, libraryIds: [] }]
    },
    {
      id: "prj_market", name: "Market launch", path: "/private/vibe-work/market-launch", kind: "work", icon: "rocket", color: "#4fa78f", pinned: false, lastOpened: yesterday, createdAt: yesterday,
      sessions: [{ id: "ses_brief", projectId: "prj_market", title: "Prepare launch brief", mode: "default", preview: "Combine product notes and market research.", updatedAt: yesterday, createdAt: yesterday, libraryIds: ["lib_product", "lib_research"] }]
    }
  ],
  plugins: [
    { id: "mcp_files", name: "Filesystem", description: "Scoped file access for the active project", transport: "stdio", command: "npx", args: ["-y", "@modelcontextprotocol/server-filesystem"], env: {}, enabled: true, scope: "project", updatedAt: now },
    { id: "mcp_github", name: "GitHub", description: "Issues, pull requests, and repository context", transport: "stdio", command: "npx", args: ["-y", "@modelcontextprotocol/server-github"], env: { GITHUB_TOKEN: "GITHUB_TOKEN" }, enabled: true, scope: "global", updatedAt: now },
    { id: "mcp_postgres", name: "PostgreSQL", description: "Explore development databases", transport: "stdio", command: "npx", args: ["-y", "@modelcontextprotocol/server-postgres"], env: {}, enabled: false, scope: "global", updatedAt: yesterday }
  ],
  libraries: [
    { id: "lib_product", name: "Product knowledge", description: "Specs, release notes, and product positioning.", color: "#ff7417", createdAt: yesterday, updatedAt: now, documents: [
      { id: "doc_spec", libraryId: "lib_product", name: "product-spec.md", kind: "file", source: "/Users/kevin/Documents/product-spec.md", localPath: "/Users/kevin/Documents/product-spec.md", size: 48210, status: "ready", createdAt: yesterday },
      { id: "doc_release", libraryId: "lib_product", name: "Release notes", kind: "webpage", source: "https://example.com/releases", size: 0, status: "ready", createdAt: yesterday }
    ] },
    { id: "lib_research", name: "Market research", description: "Reports and competitor research reused across Work tasks.", color: "#4fa78f", createdAt: yesterday, updatedAt: yesterday, documents: [
      { id: "doc_report", libraryId: "lib_research", name: "market-report.pdf", kind: "file", source: "/Users/kevin/Documents/market-report.pdf", localPath: "/Users/kevin/Documents/market-report.pdf", size: 3842000, status: "ready", createdAt: yesterday }
    ] }
  ],
  environment: { vibeAvailable: true, vibeVersion: "vibe 2.24.3", acpAvailable: true, acpPath: "/Users/kevin/.local/bin/vibe-acp", gitAvailable: true, platform: "darwin", account: { available: true, configured: false, detail: "Sign in with Mistral or configure an API key" } },
  editors: [
    { id: "vscodium", name: "VSCodium", icon: "vscodium", available: true },
    { id: "vscode", name: "Visual Studio Code", icon: "vscode", available: true },
    { id: "cursor", name: "Cursor", icon: "cursor", available: true },
    { id: "zed", name: "Zed", icon: "zed", available: false },
    { id: "sublime", name: "Sublime Text", icon: "sublime", available: false }
  ],
  editor: "vscodium",
  lastProject: "prj_vibe",
  workspace: "code",
  theme: "dark",
  accentTheme: "mistral"
};

export const demoMessages: Record<string, Message[]> = {
  ses_shell: [
    { id: "msg_1", sessionId: "ses_shell", role: "user", kind: "text", content: "Build the first native shell and wire project persistence. Keep it feeling as quick and focused as the Codex macOS app.", status: "complete", createdAt: yesterday },
    { id: "msg_2", sessionId: "ses_shell", role: "assistant", kind: "thought", content: "I’ll start with the navigation and data boundary so the visual shell is backed by real state. Then I’ll make the chat path stream-ready.", status: "complete", createdAt: yesterday },
    { id: "msg_3", sessionId: "ses_shell", role: "assistant", kind: "tool", content: "Created the application foundation", status: "complete", metadata: { title: "Created 14 files", kind: "edit", locations: ["main.go", "internal/store/store.go", "frontend/src/App.svelte"] }, createdAt: yesterday },
    { id: "msg_4", sessionId: "ses_shell", role: "assistant", kind: "text", content: "The first vertical slice is in place. Projects and conversations persist in SQLite, the window uses native macOS chrome, and the chat surface is ready for ACP events.\n\nThe important boundary is `Supervisor`: the UI never parses terminal text. It receives typed updates for messages, reasoning, plans, tools, and permissions.", status: "complete", createdAt: now }
  ],
  ses_acp: [], ses_design: [], ses_atlas: [], ses_ideas: []
};

export const demoChanges: ChangedFile[] = [
  { path: "frontend/src/App.svelte", status: "M", additions: 164, deletions: 23 },
  { path: "internal/agent/supervisor.go", status: "A", additions: 287, deletions: 0 },
  { path: "internal/store/store.go", status: "A", additions: 312, deletions: 0 },
  { path: "main.go", status: "M", additions: 38, deletions: 6 },
  { path: "README.md", status: "M", additions: 31, deletions: 2 }
];

export const demoMCPInventory: MCPInventory = {
  refreshedAt: now,
  cacheUpdatedAt: now,
  cacheAvailable: true,
  cacheStale: false,
  errors: [],
  sources: [
    { id: "connector:github_app", name: "github_app", displayName: "GitHub", kind: "connector", transport: "connector", status: "connected", connected: true, enabled: true, scope: "global", tools: [
      { name: "search_repositories", description: "Search repositories available to the connected GitHub account", enabled: true },
      { name: "get_pull_request", description: "Get pull request details and review context", enabled: true },
      { name: "create_issue", description: "Create an issue in a GitHub repository", enabled: false }
    ] },
    { id: "connector:linear", name: "linear", displayName: "Linear", kind: "connector", transport: "connector", status: "needs_auth", connected: false, enabled: false, scope: "global", tools: [] },
    { id: "connector:notion", name: "notion", displayName: "Notion", kind: "connector", transport: "connector", status: "disabled", connected: false, enabled: false, scope: "managed", tools: [] },
    { id: "connector:slack", name: "slack", displayName: "Slack", kind: "connector", transport: "connector", status: "disabled", connected: false, enabled: false, scope: "managed", tools: [] },
    { id: "server:docs", name: "docs", displayName: "Docs", kind: "server", transport: "stdio", status: "enabled", connected: false, enabled: true, scope: "project", tools: [] }
  ]
};

export const demoSkillInventory: SkillInventory = {
  projectId: "prj_vibe",
  projectName: "vibe-app",
  globalPath: "/Users/kevin/.vibe/skills",
  projectPath: "/Users/kevin/Repositories/home/vibe-app/.vibe/skills",
  errors: [],
  skills: [
    { id: "project:vibe:code-review", name: "code-review", originalName: "code-review", description: "Review a change for correctness, regressions, and missing tests.", instructions: "# Code review\n\nInspect the diff, verify behavior, and report findings by severity.", scope: "project", source: "vibe", projectId: "prj_vibe", path: "/Users/kevin/Repositories/home/vibe-app/.vibe/skills/code-review/SKILL.md", userInvocable: true, allowedTools: ["read_file", "grep"], enabled: true, editable: true, risk: "limited", updatedAt: now },
    { id: "project:vibe:release-check", name: "release-check", originalName: "release-check", description: "Run the project release checklist before publishing.", instructions: "# Release check\n\nRun tests, inspect the changelog, and summarize release risk.", scope: "project", source: "vibe", projectId: "prj_vibe", path: "/Users/kevin/Repositories/home/vibe-app/.vibe/skills/release-check/SKILL.md", userInvocable: true, allowedTools: ["read_file", "bash"], enabled: false, editable: true, risk: "shell", updatedAt: yesterday },
    { id: "global:vibe:decision-brief", name: "decision-brief", originalName: "decision-brief", description: "Turn rough context into a concise decision brief.", instructions: "# Decision brief\n\nState the decision, options, tradeoffs, recommendation, and next steps.", scope: "global", source: "vibe", path: "/Users/kevin/.vibe/skills/decision-brief/SKILL.md", userInvocable: true, allowedTools: ["read_file"], enabled: true, editable: true, risk: "limited", updatedAt: now },
    { id: "global:agents:security-audit", name: "security-audit", originalName: "security-audit", description: "Inspect a codebase for common security weaknesses.", instructions: "# Security audit\n\nFollow the bundled audit methodology.", scope: "global", source: "agents", path: "/Users/kevin/.agents/skills/security-audit/SKILL.md", userInvocable: false, allowedTools: ["read_file", "grep"], enabled: true, editable: false, risk: "limited", updatedAt: yesterday }
  ]
};

export const demoSessionConfiguration: SessionConfiguration = {
  commands: [
    { name: "help", description: "Show available commands and keyboard shortcuts", source: "vibe" },
    { name: "compact", description: "Compact conversation history", inputHint: "Optional summary instructions", source: "vibe" },
    { name: "reload", description: "Reload configuration, instructions, and skills", source: "vibe" },
    { name: "teleport", description: "Teleport this session to Vibe Code Web", source: "vibe" }
  ],
  options: [
    { id: "mode", name: "Session Mode", category: "mode", currentValue: "accept-edits", options: [
      { value: "ask", name: "Default", description: "Ask before running tools" },
      { value: "plan", name: "Plan", description: "Explore and plan without editing" },
      { value: "accept-edits", name: "Accept edits", description: "Automatically approve file edits" },
      { value: "auto-approve", name: "Auto approve", description: "Automatically approve all tools" }
    ] },
    { id: "model", name: "Model", category: "model", currentValue: "mistral-medium-3.5", options: [
      { value: "mistral-medium-3.5", name: "Mistral Medium 3.5", description: "mistral-vibe-cli-latest" },
      { value: "devstral-small", name: "Devstral Small", description: "devstral-small-latest" },
      { value: "local", name: "Devstral local", description: "devstral" }
    ] },
    { id: "thinking", name: "Thinking", category: "thinking", currentValue: "high", options: [
      { value: "off", name: "Off" }, { value: "low", name: "Low" }, { value: "medium", name: "Medium" }, { value: "high", name: "High" }, { value: "max", name: "Max" }
    ] }
  ]
};
