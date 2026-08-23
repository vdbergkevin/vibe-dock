<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { Check, Command, Copy, FileText, FolderGit2, Globe2, Pencil, Plus, RefreshCw, Search, ShieldAlert, ShieldCheck, TerminalSquare, Trash2, Upload, WandSparkles, X } from "@lucide/svelte";
  import type { Project, Skill, SkillInventory } from "../lib/types";

  export let inventory: SkillInventory = { skills: [], globalPath: "", errors: [] };
  export let project: Project | undefined;
  export let loading = false;
  export let runtimeBusy = false;

  const dispatch = createEventDispatcher<{
    refresh: void;
    reload: void;
    save: { skill: Skill };
    toggle: { skill: Skill; enabled: boolean };
    delete: { skill: Skill };
    import: { scope: Skill["scope"] };
  }>();

  let query = "";
  let scope: "all" | Skill["scope"] = "all";
  let editor: Skill | null = null;
  let toolsText = "";
  let deleting: Skill | null = null;
  let importOpen = false;
  let importScope: Skill["scope"] = "global";

  $: filtered = inventory.skills.filter((skill) => (scope === "all" || skill.scope === scope) && `${skill.name} ${skill.description} ${skill.allowedTools.join(" ")}`.toLowerCase().includes(query.toLowerCase()));
  $: globalCount = inventory.skills.filter((skill) => skill.scope === "global").length;
  $: projectCount = inventory.skills.filter((skill) => skill.scope === "project").length;
  $: enabledCount = inventory.skills.filter((skill) => skill.enabled).length;

  function blankSkill(preferredScope: Skill["scope"]): Skill {
    return {
      id: "",
      name: "",
      description: "",
      instructions: "# Workflow\n\nDescribe the steps Vibe should follow.",
      scope: preferredScope,
      source: "vibe",
      projectId: preferredScope === "project" ? project?.id : undefined,
      path: "",
      userInvocable: true,
      allowedTools: ["read_file"],
      enabled: true,
      editable: true,
      risk: "limited",
      updatedAt: new Date().toISOString()
    };
  }

  function createSkill() {
    const preferred = scope === "project" && project ? "project" : "global";
    editor = blankSkill(preferred);
    toolsText = editor.allowedTools.join(", ");
  }

  function editSkill(skill: Skill) {
    if (!skill.editable) return;
    editor = structuredClone(skill);
    toolsText = skill.allowedTools.join(", ");
  }

  function duplicateSkill(skill: Skill) {
    editor = { ...structuredClone(skill), id: "", name: `${skill.name}-copy`, originalName: undefined, source: "vibe", path: "", editable: true, enabled: true, updatedAt: new Date().toISOString() };
    toolsText = editor.allowedTools.join(", ");
  }

  function save() {
    if (!editor) return;
    const allowedTools = [...new Set(toolsText.split(/[\s,]+/).map((tool) => tool.trim()).filter(Boolean))];
    const skill = { ...editor, name: editor.name.trim().toLowerCase(), description: editor.description.trim(), instructions: editor.instructions.trim(), projectId: editor.scope === "project" ? project?.id : undefined, allowedTools, risk: riskFor(allowedTools) };
    dispatch("save", { skill });
    editor = null;
  }

  function riskFor(tools: string[]): Skill["risk"] {
    const normalized = tools.map((tool) => tool.toLowerCase());
    if (normalized.some((tool) => tool.includes("bash") || tool.includes("shell") || tool.includes("terminal"))) return "shell";
    if (normalized.some((tool) => tool.includes("write") || tool.includes("edit") || tool.includes("delete") || tool.includes("replace"))) return "write";
    return "limited";
  }

  function sourceLabel(skill: Skill) {
    return skill.source === "agents" ? "Agent Skills" : skill.source === "custom" ? "Custom path" : "Vibe";
  }

  function riskLabel(skill: Skill) {
    return skill.risk === "shell" ? "Shell access" : skill.risk === "write" ? "Can edit" : "Scoped tools";
  }
</script>

<section class="page-view skills-view">
  <header class="page-header">
    <div><span class="page-kicker">Reusable workflows</span><h1><WandSparkles size={22} /> Skills</h1><p>Teach Vibe repeatable methods and expose them as slash commands.</p></div>
    <div class="page-actions">
      <button class="secondary-button" disabled={loading || runtimeBusy} title={runtimeBusy ? "Wait for the active task to finish" : "Reload configuration and Skills"} on:click={() => dispatch("reload")}><RefreshCw class={loading ? "spin" : ""} size={14} /> Reload Vibe</button>
      <button class="secondary-button" on:click={() => { importScope = scope === "project" && project ? "project" : "global"; importOpen = true; }}><Upload size={14} /> Import</button>
      <button class="primary-button" on:click={createSkill}><Plus size={15} /> New Skill</button>
    </div>
  </header>

  <div class="skill-explainer"><WandSparkles size={18} /><div><strong>Skills describe how Vibe should work</strong><span>They can use Libraries for context and connectors for external actions. Project skills are loaded only from trusted folders.</span></div><span>{enabledCount} active</span></div>

  {#each inventory.errors as error}<div class="inventory-error"><ShieldAlert size={14} /><span>{error}</span></div>{/each}

  <div class="skills-toolbar">
    <div class="skill-scope-tabs" role="tablist" aria-label="Skill scope">
      <button class:active={scope === "all"} role="tab" aria-selected={scope === "all"} on:click={() => (scope = "all")}>All <span>{inventory.skills.length}</span></button>
      <button class:active={scope === "global"} role="tab" aria-selected={scope === "global"} on:click={() => (scope = "global")}><Globe2 size={13} /> Global <span>{globalCount}</span></button>
      <button class:active={scope === "project"} role="tab" aria-selected={scope === "project"} on:click={() => (scope = "project")}><FolderGit2 size={13} /> {project?.name || "Project"} <span>{projectCount}</span></button>
    </div>
    <label class="page-search"><Search size={14} /><input bind:value={query} placeholder="Search skills" aria-label="Search skills" /></label>
    <button class="icon-button" class:spin={loading} title="Refresh Skills" on:click={() => dispatch("refresh")}><RefreshCw size={14} /></button>
  </div>

  {#if loading && inventory.skills.length === 0}
    <div class="skills-empty"><RefreshCw class="spin" size={22} /><strong>Discovering Skills…</strong><span>Reading Vibe and Agent Skills locations.</span></div>
  {:else if filtered.length === 0}
    <div class="skills-empty"><WandSparkles size={25} /><strong>{inventory.skills.length ? "No matching Skills" : "No Skills installed yet"}</strong><span>{inventory.skills.length ? "Try another search or scope." : "Create a reusable workflow or import a folder containing SKILL.md."}</span><button class="primary-button" on:click={createSkill}><Plus size={14} /> New Skill</button></div>
  {:else}
    <div class="skill-grid">
      {#each filtered as skill (skill.id + skill.path)}
        <article class:disabled={!skill.enabled} class="skill-card">
          <div class="skill-card-head">
            <span class="skill-logo"><WandSparkles size={17} /></span>
            <div><div><strong>{skill.name}</strong>{#if skill.userInvocable}<code>/{skill.name}</code>{/if}</div><p>{skill.description || "No description"}</p></div>
            <button class:enabled={skill.enabled} class="skill-switch" role="switch" aria-checked={skill.enabled} title={skill.enabled ? "Disable Skill" : "Enable Skill"} on:click={() => dispatch("toggle", { skill, enabled: !skill.enabled })}><span></span></button>
          </div>
          <div class="skill-meta"><span class:project={skill.scope === "project"}>{skill.scope === "project" ? project?.name || "Project" : "Global"}</span><span>{sourceLabel(skill)}</span><span class:risk={skill.risk !== "limited"}><svelte:component this={skill.risk === "shell" ? TerminalSquare : skill.risk === "write" ? ShieldAlert : ShieldCheck} size={11} /> {riskLabel(skill)}</span></div>
          <div class="skill-tools">
            {#each skill.allowedTools.slice(0, 4) as tool}<code>{tool}</code>{/each}
            {#if skill.allowedTools.length > 4}<span>+{skill.allowedTools.length - 4}</span>{/if}
            {#if skill.allowedTools.length === 0}<span>Default tool policy</span>{/if}
          </div>
          <div class="skill-card-foot">
            <span title={skill.path}><FileText size={12} /> {skill.source === "vibe" ? "SKILL.md" : sourceLabel(skill)}</span>
            <div>
              {#if skill.editable}<button class="icon-button small" title="Edit Skill" on:click={() => editSkill(skill)}><Pencil size={13} /></button>{/if}
              <button class="icon-button small" title="Duplicate Skill" on:click={() => duplicateSkill(skill)}><Copy size={13} /></button>
              {#if skill.editable}<button class="icon-button small danger" title="Delete Skill" on:click={() => (deleting = skill)}><Trash2 size={13} /></button>{/if}
            </div>
          </div>
        </article>
      {/each}
    </div>
  {/if}
</section>

{#if editor}
  <div class="modal-backdrop" role="presentation" on:click={(event) => event.target === event.currentTarget && (editor = null)}>
    <form class="skill-editor" on:submit|preventDefault={save}>
      <div class="modal-head"><div><span class="page-kicker">{editor.originalName ? "Edit workflow" : "Create workflow"}</span><h2><WandSparkles size={18} /> {editor.originalName ? editor.name : "New Skill"}</h2></div><button type="button" class="icon-button" title="Close" on:click={() => (editor = null)}><X size={16} /></button></div>
      <div class="field-pair">
        <label><span>Name <small>slash-command name</small></span><input bind:value={editor.name} required maxlength="64" pattern="[a-z0-9][a-z0-9-]*" placeholder="code-review" /></label>
        <label><span>Scope</span><select bind:value={editor.scope} disabled={Boolean(editor.originalName)} on:change={() => { if (editor) editor.projectId = editor.scope === "project" ? project?.id : undefined; }}><option value="global">Global</option><option value="project" disabled={!project}>Project{project ? ` · ${project.name}` : " unavailable"}</option></select></label>
      </div>
      <label><span>Description <small>used for automatic discovery</small></span><input bind:value={editor.description} required maxlength="500" placeholder="Review code for correctness and missing tests" /></label>
      <label><span>Allowed tools <small>comma separated</small></span><input bind:value={toolsText} placeholder="read_file, grep" /></label>
      <label class="skill-invocable"><input type="checkbox" bind:checked={editor.userInvocable} /><span><Command size={14} /><strong>Expose as /{editor.name || "skill-name"}</strong><small>Show this Skill in slash-command autocomplete.</small></span></label>
      <label><span>Instructions <small>Markdown body of SKILL.md</small></span><textarea class="skill-instructions" bind:value={editor.instructions} required spellcheck="false"></textarea></label>
      <div class:risk={riskFor(toolsText.split(/[\s,]+/).filter(Boolean)) !== "limited"} class="skill-safety-preview"><svelte:component this={riskFor(toolsText.split(/[\s,]+/).filter(Boolean)) === "limited" ? ShieldCheck : ShieldAlert} size={15} /><span>{riskFor(toolsText.split(/[\s,]+/).filter(Boolean)) === "shell" ? "This Skill can request shell commands; Vibe’s active permission mode still applies." : riskFor(toolsText.split(/[\s,]+/).filter(Boolean)) === "write" ? "This Skill can request edits; Vibe’s active permission mode still applies." : "This Skill has a narrow, read-oriented tool set."}</span></div>
      <div class="modal-actions"><button type="button" class="secondary-button" on:click={() => (editor = null)}>Cancel</button><button class="primary-button" disabled={!editor.name.trim() || !editor.description.trim() || !editor.instructions.trim()}><Check size={14} /> Save Skill</button></div>
    </form>
  </div>
{/if}

{#if importOpen}
  <div class="modal-backdrop" role="presentation" on:click={(event) => event.target === event.currentTarget && (importOpen = false)}>
    <div class="skill-import-dialog"><div class="modal-head"><div><span class="page-kicker">Agent Skills format</span><h2><Upload size={18} /> Import Skill</h2></div><button class="icon-button" title="Close" on:click={() => (importOpen = false)}><X size={16} /></button></div><p>Choose a folder containing a valid <code>SKILL.md</code>. Supporting references and templates are copied with it.</p><label><span>Install for</span><select bind:value={importScope}><option value="global">All projects</option><option value="project" disabled={!project}>{project ? project.name : "No project selected"}</option></select></label><div class="skill-import-note"><ShieldCheck size={14} /><span>Symbolic links and imports larger than 10 MB are rejected.</span></div><div class="modal-actions"><button class="secondary-button" on:click={() => (importOpen = false)}>Cancel</button><button class="primary-button" on:click={() => { dispatch("import", { scope: importScope }); importOpen = false; }}><Upload size={14} /> Choose folder</button></div></div>
  </div>
{/if}

{#if deleting}
  <div class="modal-backdrop" role="presentation" on:click={(event) => event.target === event.currentTarget && (deleting = null)}>
    <div class="skill-import-dialog confirm-dialog"><div class="modal-head"><div><span class="page-kicker">Delete Skill</span><h2>Remove {deleting.name}?</h2></div></div><p>This removes the managed Skill folder and its bundled files. It cannot be undone.</p><div class="modal-actions"><button class="secondary-button" on:click={() => (deleting = null)}>Cancel</button><button class="danger-button" on:click={() => { if (deleting) dispatch("delete", { skill: deleting }); deleting = null; }}><Trash2 size={14} /> Delete Skill</button></div></div>
  </div>
{/if}
