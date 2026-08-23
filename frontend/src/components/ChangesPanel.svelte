<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { CheckCheck, FileCode2, GitPullRequest, MoreHorizontal, RefreshCw, X } from "@lucide/svelte";
  import type { ChangedFile, Project } from "../lib/types";
  export let project: Project;
  export let files: ChangedFile[] = [];
  export let loading = false;
  const dispatch = createEventDispatcher();
  $: additions = files.reduce((sum, file) => sum + file.additions, 0);
  $: deletions = files.reduce((sum, file) => sum + file.deletions, 0);
</script>

<aside class="changes-panel">
  <div class="changes-head">
    <div><span>Changes</span><span class="count-badge">{files.length}</span></div>
    <div class="panel-actions">
      <button class="icon-button small" class:spin={loading} title="Refresh" on:click={() => dispatch("refresh")}><RefreshCw size={14} /></button>
      <button class="icon-button small" title="Close" on:click={() => dispatch("close")}><X size={15} /></button>
    </div>
  </div>
  <div class="changes-summary">
    <div class="summary-icon"><GitPullRequest size={17} /></div>
    <div><strong>{project.name}</strong><span>Working tree</span></div>
    <div class="diff-total"><span>+{additions}</span><span>−{deletions}</span></div>
  </div>
  <div class="file-list">
    {#each files as file (file.path)}
      <button class="file-row">
        <FileCode2 size={15} />
        <div class="file-copy"><strong>{file.path.split("/").pop()}</strong>{#if file.path.includes("/")}<span>{file.path.split("/").slice(0, -1).join("/")}</span>{/if}</div>
        <div class="file-counts"><span class="add">+{file.additions}</span><span class="del">−{file.deletions}</span></div>
        <span class={`status status-${file.status.toLowerCase()}`}>{file.status}</span>
      </button>
    {/each}
    {#if !loading && files.length === 0}<div class="clean-state"><CheckCheck size={26} /><strong>Working tree clean</strong><span>No local changes in this project.</span></div>{/if}
  </div>
  <div class="changes-foot">
    <button class="review-button" disabled={!files.length}><GitPullRequest size={13} /> Review changes</button>
    <button class="icon-button"><MoreHorizontal size={16} /></button>
  </div>
</aside>
