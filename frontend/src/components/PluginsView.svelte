<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { Box, Check, ChevronDown, ChevronRight, CircleHelp, ExternalLink, Globe2, KeyRound, LoaderCircle, Plus, Plug, RefreshCw, Search, Server, ShieldCheck, Terminal, TriangleAlert, Wrench, X } from "@lucide/svelte";
  import BrandIcon from "./BrandIcon.svelte";
  import type { MCPInventory, MCPSource, Plugin } from "../lib/types";

  export let plugins: Plugin[] = [];
  export let inventory: MCPInventory = { sources: [], refreshedAt: "", cacheAvailable: false, cacheStale: false, errors: [] };
  export let loading = false;
  export let connectorUpdating = "";

  const dispatch = createEventDispatcher();
  let tab: "connectors" | "servers" = "connectors";
  let query = "";
  let expandedSource = "";
  let editing: Plugin | null = null;
  let argsText = "";
  let envText = "";

  $: connectors = inventory.sources.filter((source) => source.kind === "connector");
  $: vibeServers = inventory.sources.filter((source) => source.kind === "server");
  $: filteredConnectors = connectors.filter(matchesQuery);
  $: connectorSections = [
    {
      key: "connected",
      title: "Connected",
      description: "Authenticated in Mistral — choose which ones Vibe may use",
      sources: filteredConnectors.filter((source) => source.connected)
    },
    {
      key: "available",
      title: "Available to connect",
      description: "Provided by your Mistral workspace",
      sources: filteredConnectors.filter((source) => !source.connected)
    }
  ];
  $: filteredServers = vibeServers.filter(matchesQuery);
  $: filteredPlugins = plugins.filter((plugin) => `${plugin.name} ${plugin.description} ${plugin.transport}`.toLowerCase().includes(query.toLowerCase()));
  $: connectedConnectors = connectors.filter((source) => source.connected).length;
  $: activeConnectors = connectors.filter((source) => source.connected && source.enabled).length;

  function matchesQuery(source: MCPSource) {
    return `${source.displayName} ${source.name} ${source.status} ${source.tools.map((tool) => tool.name).join(" ")}`.toLowerCase().includes(query.toLowerCase());
  }

  function selectTab(next: "connectors" | "servers") {
    tab = next;
    query = "";
    expandedSource = "";
  }

  function edit(plugin?: Plugin) {
    editing = plugin ? structuredClone(plugin) : { id: "", name: "", description: "", transport: "stdio", command: "", args: [], env: {}, enabled: true, scope: "global", updatedAt: new Date().toISOString() };
    argsText = editing.args.join(" ");
    envText = Object.entries(editing.env).map(([key, value]) => `${key}=${value}`).join("\n");
  }

  function save() {
    if (!editing) return;
    editing.args = argsText.trim() ? argsText.match(/(?:[^\s"]+|"[^"]*")+/g)?.map((value) => value.replace(/^"|"$/g, "")) || [] : [];
    editing.env = Object.fromEntries(envText.split("\n").map((line) => line.trim()).filter(Boolean).map((line) => { const index = line.indexOf("="); return index > 0 ? [line.slice(0, index), line.slice(index + 1)] : [line, ""]; }));
    dispatch("save", { plugin: editing });
    editing = null;
  }

  function statusLabel(status: MCPSource["status"]) {
    return ({ connected: "Connected", enabled: "Enabled", disabled: "Disabled", needs_auth: "Needs sign-in", needs_setup: "Needs setup", unavailable: "Unavailable" })[status];
  }

  function sourceSummary(source: MCPSource) {
    if (source.kind === "server") return `${source.transport} MCP server from ${source.scope === "project" ? "this project" : "your global Vibe config"}`;
    if (source.connected && source.enabled) return `${source.tools.filter((tool) => tool.enabled).length} of ${source.tools.length} ${source.tools.length === 1 ? "tool" : "tools"} available to Vibe`;
    if (source.connected) return "Connected in Mistral · enable it for VibeDock";
    if (source.status === "needs_auth") return "Connect this account in Mistral before its tools can be used";
    if (source.status === "needs_setup") return "Credentials still need to be configured in Vibe";
    if (source.status === "disabled") return "Available from your Mistral workspace";
    return "Vibe could not currently reach this integration";
  }

  function detailHint(source: MCPSource) {
    if (source.status === "needs_auth") return "Connect the account in Mistral, then refresh this inventory.";
    if (source.status === "needs_setup") return "Finish the connector’s credentials and permissions in Mistral.";
    if (source.connected && !source.enabled) return "Enable this connector below so its tools are available to new Vibe prompts.";
    if (source.status === "disabled") return "Connect this integration in Mistral, then refresh here.";
    if (source.status === "unavailable") return "Refresh after Vibe has rediscovered this source.";
    if (source.kind === "server") return "Tool names become available after Vibe starts this MCP server in a session.";
    return "Vibe has not reported any tools for this connector yet.";
  }

  function cacheLabel(value?: string) {
    if (!value) return "Not discovered yet";
    return `Vibe discovery ${new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit" }).format(new Date(value))}`;
  }
</script>

<section class="page-view plugins-view">
  <header class="page-header integration-header">
    <div><span class="page-kicker">Configure</span><h1><Plug size={22} /> MCP &amp; connectors</h1><p>Mirror Mistral-managed connectors and configure local MCP servers.</p></div>
    <div class="page-actions">
      <button class="secondary-button" on:click={() => dispatch("external", { destination: tab === "connectors" ? "connectors" : "connectorDebugger" })}><ExternalLink size={14} /> {tab === "connectors" ? "Manage in Mistral" : "Remote debugger"}</button>
      <button class="secondary-button" disabled={loading} on:click={() => dispatch("refresh")}><RefreshCw class={loading ? "spin" : ""} size={14} /> Refresh</button>
      {#if tab === "servers"}<button class="primary-button" on:click={() => edit()}><Plus size={15} /> Add server</button>{/if}
    </div>
  </header>

  <div class="integration-tabs" role="tablist" aria-label="MCP source type">
    <button class:active={tab === "connectors"} role="tab" aria-selected={tab === "connectors"} on:click={() => selectTab("connectors")}><Plug size={14} /> Connectors <span>{connectors.length}</span></button>
    <button class:active={tab === "servers"} role="tab" aria-selected={tab === "servers"} on:click={() => selectTab("servers")}><Server size={14} /> MCP servers <span>{vibeServers.length + plugins.length}</span></button>
  </div>

  {#if tab === "connectors"}
    <div class:warning={inventory.cacheStale || !inventory.cacheAvailable} class="connector-sync">
      {#if inventory.cacheStale || !inventory.cacheAvailable}<TriangleAlert size={17} />{:else}<ShieldCheck size={17} />{/if}
      <div>
        <strong>{inventory.cacheStale ? "Connector discovery is stale" : inventory.cacheAvailable ? "Synced with Vibe" : "No connector discovery yet"}</strong>
        <span>{inventory.cacheAvailable ? `${cacheLabel(inventory.cacheUpdatedAt)}. Credentials stay in Mistral; local Vibe access is controlled with the switches below.` : "Configure hosted connectors in Mistral, use /mcp once in Vibe, then refresh this inventory."}</span>
      </div>
      <span class="read-only-pill">Cloud metadata</span>
    </div>

    {#each inventory.errors as error}<div class="inventory-error"><TriangleAlert size={14} /><span>{error}</span></div>{/each}

    <div class="view-toolbar">
      <label class="page-search"><Search size={15} /><input bind:value={query} placeholder="Search connectors or tools" /></label>
      <span>{activeConnectors} enabled · {connectedConnectors} connected</span>
    </div>

    {#if loading && connectors.length === 0}
      <div class="inventory-loading"><LoaderCircle class="spin" size={18} /><span>Reading Vibe connector metadata…</span></div>
    {:else}
      {#each connectorSections as section (section.key)}
        {#if section.sources.length}
          <section class="connector-section" aria-labelledby={`connector-section-${section.key}`}>
            <div class="integration-section-head connector-section-head">
              <div><span id={`connector-section-${section.key}`}>{section.title}</span><small>{section.description}</small></div>
              <span class:connected-count={section.key === "connected"}>{section.sources.length}</span>
            </div>
            <div class="integration-grid" data-connector-section={section.key}>
              {#each section.sources as source (source.id)}
                <article class:disabled={!source.connected && source.status === "disabled"} class:expanded={expandedSource === source.id} class="source-card">
                  <button class="source-main" aria-expanded={expandedSource === source.id} on:click={() => expandedSource = expandedSource === source.id ? "" : source.id}>
                    <div class="source-logo branded"><BrandIcon name={source.name || source.displayName} size={20} /></div>
                    <div class="source-copy">
                      <div><strong>{source.displayName}</strong><code>{source.name}</code></div>
                      <p>{sourceSummary(source)}</p>
                    </div>
                    <span class={`source-status ${source.connected ? "connected" : source.status}`}>{#if source.connected}<Check size={11} />{:else if source.status === "needs_auth"}<KeyRound size={11} />{:else if source.status === "needs_setup"}<Wrench size={11} />{/if}{source.connected ? "Connected" : statusLabel(source.status)}</span>
                    <ChevronDown class={expandedSource === source.id ? "rotated" : ""} size={15} />
                  </button>
                  <div class="source-foot">
                    <span><Box size={13} /> {source.tools.length} {source.tools.length === 1 ? "tool" : "tools"}</span>
                    {#if source.connected}
                      <label class="connector-access" title={source.enabled ? "Disable this connector in Vibe" : "Enable this connector in Vibe"}>
                        <span>{connectorUpdating === source.name ? "Updating…" : source.enabled ? "Enabled in Vibe" : "Enable in Vibe"}</span>
                        {#if connectorUpdating === source.name}<LoaderCircle class="spin" size={13} />{/if}
                        <span class="switch connector-switch"><input aria-label={`${source.enabled ? "Disable" : "Enable"} ${source.displayName} in Vibe`} type="checkbox" checked={source.enabled} disabled={connectorUpdating !== ""} on:change={(event) => dispatch("toggleConnector", { name: source.name, enabled: event.currentTarget.checked })} /><span></span></span>
                      </label>
                    {:else}
                      <span>Mistral managed</span>
                    {/if}
                  </div>
                  {#if expandedSource === source.id}
                    <div class="source-details">
                      {#if source.error}<div class="source-error"><TriangleAlert size={13} />{source.error}</div>{/if}
                      {#if source.connected && !source.enabled}
                        <div class="connector-activation-note"><Plug size={15} /><div><strong>Connected, but not available to Vibe</strong><span>Turn on “Enable in Vibe” above. VibeDock will reload its idle ACP sessions so the next prompt can use these tools.</span></div></div>
                      {/if}
                      {#if source.tools.length}
                        <div class="tool-list-head"><span>Available tools</span><span>{source.tools.filter((tool) => tool.enabled).length} enabled</span></div>
                        <div class="connector-tools">
                          {#each source.tools as tool (tool.name)}
                            <div class:disabled={!tool.enabled} class="connector-tool"><Wrench size={13} /><div><strong>{tool.name}</strong><span>{tool.description || "No description reported by Vibe"}</span></div>{#if !tool.enabled}<span class="tool-disabled">off</span>{/if}</div>
                          {/each}
                        </div>
                      {:else}
                        <div class="no-source-tools"><CircleHelp size={16} /><span>{detailHint(source)}</span></div>
                      {/if}
                      <div class="dashboard-handoff"><div><strong>Hosted by Mistral</strong><span>Credentials, sharing, and tool permissions are managed online.</span></div><button on:click={() => dispatch("external", { destination: "connectors" })}>Open dashboard <ExternalLink size={12} /></button></div>
                    </div>
                  {/if}
                </article>
              {/each}
            </div>
          </section>
        {/if}
      {/each}
      {#if filteredConnectors.length === 0}<div class="no-plugin-results connector-empty"><Plug size={28} /><strong>{connectors.length ? "No connectors found" : "No connectors discovered"}</strong><span>{connectors.length ? "Try another search." : "Manage connectors in Mistral, use /mcp once in Vibe, then refresh."}</span></div>{/if}
    {/if}
  {:else}
    <div class="plugin-security"><ShieldCheck size={18} /><div><strong>Secrets stay out of the database</strong><span>Environment bindings store variable names only. Values are resolved from the app process when a Vibe session starts.</span></div><button title="Learn more"><CircleHelp size={15} /></button></div>

    {#each inventory.errors as error}<div class="inventory-error"><TriangleAlert size={14} /><span>{error}</span></div>{/each}

    <div class="view-toolbar">
      <label class="page-search"><Search size={15} /><input bind:value={query} placeholder="Search MCP servers" /></label>
      <span>{plugins.filter((plugin) => plugin.enabled).length + vibeServers.filter((source) => source.status !== "disabled").length} active</span>
    </div>

    {#if filteredServers.length}
      <div class="integration-section-head"><span>Configured in Vibe</span><span>{filteredServers.length}</span></div>
      <div class="integration-grid native-server-grid">
        {#each filteredServers as source (source.id)}
          <article class:disabled={source.status === "disabled"} class="source-card compact">
            <button class="source-main" on:click={() => expandedSource = expandedSource === source.id ? "" : source.id}>
              <div class="source-logo server"><Server size={18} /></div>
              <div class="source-copy"><div><strong>{source.displayName}</strong><code>{source.name}</code></div><p>{sourceSummary(source)}</p></div>
              <span class={`source-status ${source.status}`}>{statusLabel(source.status)}</span>
              <ChevronRight class={expandedSource === source.id ? "rotated-right" : ""} size={15} />
            </button>
            {#if expandedSource === source.id}<div class="source-details"><div class="no-source-tools"><CircleHelp size={16} /><span>{detailHint(source)}</span></div></div>{/if}
          </article>
        {/each}
      </div>
    {/if}

    <div class="integration-section-head custom-head"><span>Desktop-managed servers</span><span>{filteredPlugins.length}</span></div>
    <div class="plugin-grid">
      {#each filteredPlugins as plugin (plugin.id)}
        <article class:disabled={!plugin.enabled} class="plugin-card">
          <button class="plugin-main" on:click={() => edit(plugin)}>
            <div class="plugin-logo">{#if plugin.transport === "stdio"}<Terminal size={19} />{:else}<Globe2 size={19} />{/if}</div>
            <div class="plugin-copy"><div><strong>{plugin.name}</strong><span class="scope-pill">{plugin.scope}</span></div><p>{plugin.description || "Custom MCP integration"}</p><code>{plugin.transport === "stdio" ? [plugin.command, ...plugin.args].join(" ") : plugin.command}</code></div>
            <ChevronRight size={16} />
          </button>
          <div class="plugin-foot"><span><Box size={13} /> {plugin.transport}</span><label class="switch"><input type="checkbox" checked={plugin.enabled} on:change={(event) => dispatch("save", { plugin: { ...plugin, enabled: event.currentTarget.checked } })} /><span></span></label></div>
        </article>
      {/each}
      {#if filteredPlugins.length === 0 && filteredServers.length === 0}<div class="no-plugin-results"><Box size={28} /><strong>No MCP servers found</strong><span>Try another search or add a custom MCP server.</span></div>{/if}
    </div>
  {/if}
</section>

{#if editing}
  <div class="modal-backdrop" role="presentation" on:click={(event) => event.target === event.currentTarget && (editing = null)}>
    <form class="plugin-editor" on:submit|preventDefault={save}>
      <div class="modal-head"><div><span class="page-kicker">MCP server</span><h2>{editing.id ? "Edit server" : "Add server"}</h2></div><button type="button" class="icon-button" on:click={() => editing = null}><X size={16} /></button></div>
      <label><span>Name</span><input bind:value={editing.name} placeholder="Filesystem" required /></label>
      <label><span>Description</span><input bind:value={editing.description} placeholder="What this server makes available" /></label>
      <div class="field-pair">
        <label><span>Transport</span><select bind:value={editing.transport}><option value="stdio">stdio</option><option value="http">HTTP</option><option value="sse">SSE</option></select></label>
        <label><span>Scope</span><select bind:value={editing.scope}><option value="global">Global</option><option value="project">Project</option></select></label>
      </div>
      <label><span>{editing.transport === "stdio" ? "Command" : "URL"}</span><input bind:value={editing.command} placeholder={editing.transport === "stdio" ? "npx" : "https://mcp.example.com"} required /></label>
      {#if editing.transport !== "stdio"}<div class="remote-debug-hint"><div><strong>Remote MCP server</strong><span>Validate reachability, authentication, and tool discovery with Mistral Studio.</span></div><button type="button" on:click={() => dispatch("external", { destination: "connectorDebugger" })}>Open debugger <ExternalLink size={12} /></button></div>{/if}
      {#if editing.transport === "stdio"}<label><span>Arguments</span><input bind:value={argsText} placeholder='-y "@modelcontextprotocol/server-filesystem"' /></label>{/if}
      <label><span>Environment references <small>one MCP_KEY=SOURCE_VARIABLE per line</small></span><textarea bind:value={envText} placeholder="GITHUB_TOKEN=GITHUB_TOKEN"></textarea></label>
      <div class="modal-actions"><button type="button" class="secondary-button" on:click={() => editing = null}>Cancel</button><button class="primary-button"><Check size={15} /> Save server</button></div>
    </form>
  </div>
{/if}
