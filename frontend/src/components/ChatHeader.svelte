<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { BookOpen, ChevronsUpDown, GitBranch, MoreHorizontal, PanelRight, TerminalSquare } from "@lucide/svelte";
  import BrandIcon from "./BrandIcon.svelte";
  import type { CodeEditor, Conversation, Project, StreamState } from "../lib/types";

  export let project: Project;
  export let conversation: Conversation;
  export let stream: StreamState;
  export let changesOpen = false;
  export let terminalOpen = false;
  export let codeEditor: CodeEditor;
  const dispatch = createEventDispatcher();

  $: usagePercent = stream.usage?.size ? Math.min(100, Math.round((stream.usage.used / stream.usage.size) * 100)) : 18;
</script>

<header class="chat-header">
  <button class="conversation-title" on:click={() => dispatch("palette")}>
    <span class="mistral-chat-mark"><BrandIcon name="Mistral AI" size={18} /></span>
    <div>
      <div class="header-eyebrow"><span class="mistral-chat-label">Mistral AI</span><span class="slash">·</span><span>{project.name}</span><span class="slash">/</span><span>{project.kind}</span></div>
      <strong>{conversation.title}</strong>
    </div>
    <ChevronsUpDown size={14} />
  </button>
  <div class="header-actions">
    {#if project.kind === "code"}<span class="branch"><GitBranch size={13} /> main</span>{/if}
    {#if conversation.libraryIds?.length}<span class="library-context" title="Attached Libraries"><BookOpen size={12} /> {conversation.libraryIds.length}</span>{/if}
    <button class="context-ring" title={`${usagePercent}% context used`} style={`--usage:${usagePercent * 3.6}deg`}><span>{usagePercent}</span></button>
    {#if project.kind === "code"}
      <button class="editor-button" title={`Open project in ${codeEditor.name}`} on:click={() => dispatch("editor")}><BrandIcon name={codeEditor.icon} size={14} /><span>{codeEditor.name}</span></button>
      <button class:active={terminalOpen} class="icon-button" title={terminalOpen ? "Close integrated terminal" : "Open integrated terminal"} aria-label={terminalOpen ? "Close integrated terminal" : "Open integrated terminal"} on:click={() => dispatch("terminal")}><TerminalSquare size={16} /></button>
      <button class:active={changesOpen} class="icon-button" title="Toggle changes" on:click={() => dispatch("changes")}><PanelRight size={16} /></button>
    {/if}
    <button class="icon-button" title="More actions"><MoreHorizontal size={17} /></button>
  </div>
</header>
