<script lang="ts">
  import { ChevronDown, ListOrdered, X } from "@lucide/svelte";
  import { createEventDispatcher } from "svelte";

  export let activePrompt = "";
  export let queuedPrompts: string[] = [];

  const dispatch = createEventDispatcher();
  let expanded = false;

  $: if (!activePrompt) expanded = false;
</script>

{#if activePrompt || queuedPrompts.length}
  <section class="prompt-queue" aria-label="Prompt queue">
    {#if activePrompt}
      <div class="prompt-queue-current">
        <span class="prompt-queue-activity" aria-hidden="true"><i></i><i></i><i></i></span>
        <button type="button" class="prompt-queue-current-copy" aria-expanded={expanded} title={expanded ? "Collapse active prompt" : "Show full active prompt"} on:click={() => (expanded = !expanded)}>
          <span class="prompt-queue-kicker">Working on</span>
          <span class:expanded class="prompt-queue-text">{activePrompt}</span>
        </button>
        <div class="prompt-queue-meta">
          {#if queuedPrompts.length}<span>{queuedPrompts.length} queued</span>{/if}
          <span class:expanded class="prompt-queue-chevron"><ChevronDown size={13} /></span>
        </div>
      </div>
    {/if}

    {#if queuedPrompts.length}
      <div class="prompt-queue-list">
        <div class="prompt-queue-list-head"><ListOrdered size={12} /><span>Up next</span><span>Runs in order</span></div>
        {#each queuedPrompts as prompt, index (`${index}:${prompt}`)}
          <div class="prompt-queue-item">
            <span class="prompt-queue-index">{index + 1}</span>
            <span class="prompt-queue-item-text" title={prompt}>{prompt}</span>
            <button type="button" title="Remove queued prompt" aria-label={`Remove queued prompt ${index + 1}`} on:click={() => dispatch("remove", { index })}><X size={12} /></button>
          </div>
        {/each}
      </div>
    {/if}
  </section>
{/if}
