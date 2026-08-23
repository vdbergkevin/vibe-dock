<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { BookOpen, Check, LibraryBig, Settings2 } from "@lucide/svelte";
  import type { Library } from "../lib/types";

  export let libraries: Library[] = [];
  export let attachedIds: string[] = [];
  export let disabled = false;

  const dispatch = createEventDispatcher();
  let open = false;

  $: attached = new Set(attachedIds || []);

  function toggleMenu(event: MouseEvent) {
    event.stopPropagation();
    if (!disabled) open = !open;
  }

  function toggleLibrary(libraryId: string) {
    const next = new Set(attached);
    next.has(libraryId) ? next.delete(libraryId) : next.add(libraryId);
    dispatch("change", { libraryIds: [...next] });
  }

  function manage() {
    open = false;
    dispatch("manage");
  }
</script>

<svelte:window on:click={() => (open = false)} on:keydown={(event) => event.key === "Escape" && (open = false)} />

<div class:open class="library-menu">
  <button type="button" class="composer-icon library-trigger" class:attached={attached.size > 0} title="Attach Libraries" aria-haspopup="dialog" aria-expanded={open} {disabled} on:click={toggleMenu}>
    <BookOpen size={15} />
    {#if attached.size}<span>{attached.size}</span>{/if}
  </button>
  {#if open}
    <div class="library-popover" role="dialog" aria-label="Attach Libraries">
      <div class="library-popover-head"><div><LibraryBig size={14} /><strong>Libraries</strong></div><span>Reusable context</span></div>
      <div class="library-picker-list">
        {#each libraries as library (library.id)}
          <button type="button" class:selected={attached.has(library.id)} aria-pressed={attached.has(library.id)} on:click={() => toggleLibrary(library.id)}>
            <span class="library-dot" style={`--library-color:${library.color}`}></span>
            <div><strong>{library.name}</strong><span>{library.documents.length} {library.documents.length === 1 ? "source" : "sources"}</span></div>
            {#if attached.has(library.id)}<Check size={13} />{/if}
          </button>
        {/each}
        {#if libraries.length === 0}<div class="library-picker-empty">Create a Library to reuse documents and webpages across tasks.</div>{/if}
      </div>
      <button type="button" class="manage-libraries" on:click={manage}><Settings2 size={13} /> Manage Libraries</button>
    </div>
  {/if}
</div>
