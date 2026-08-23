<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { BookOpen, ExternalLink, FileText, Globe2, LibraryBig, Link2, Plus, Search, ShieldCheck, Trash2, Upload, X } from "@lucide/svelte";
  import type { Library, LibraryDocument } from "../lib/types";

  export let libraries: Library[] = [];
  export let busy = "";

  const dispatch = createEventDispatcher();
  let query = "";
  let selectedId = "";
  let creating = false;
  let addingWebpage = false;
  let deletingLibrary = "";
  let deletingDocument: LibraryDocument | null = null;
  let name = "";
  let description = "";
  let webpage = "";

  $: filtered = libraries.filter((library) => `${library.name} ${library.description} ${library.documents.map((document) => document.name).join(" ")}`.toLowerCase().includes(query.toLowerCase()));
  $: if (!selectedId && libraries.length) selectedId = libraries[0].id;
  $: selected = libraries.find((library) => library.id === selectedId) || filtered[0];

  function openCreate() {
    name = "";
    description = "";
    creating = true;
  }

  function create() {
    if (!name.trim()) return;
    dispatch("create", { name: name.trim(), description: description.trim() });
    creating = false;
  }

  function addWebpage() {
    if (!selected || !webpage.trim()) return;
    dispatch("webpage", { libraryId: selected.id, url: webpage.trim() });
    addingWebpage = false;
    webpage = "";
  }

  function formatSize(bytes: number) {
    if (!bytes) return "Web page";
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
</script>

<section class="page-view libraries-view">
  <header class="page-header">
    <div><span class="page-kicker">Reusable context</span><h1><LibraryBig size={22} /> Libraries</h1><p>Group documents and webpages once, then attach them to any Chat, Work, or Code conversation.</p></div>
    <div class="page-actions">
      <button class="secondary-button" on:click={() => dispatch("external", { destination: "libraries" })}><ExternalLink size={14} /> Mistral Libraries</button>
      <button class="primary-button" on:click={openCreate}><Plus size={15} /> New Library</button>
    </div>
  </header>

  <div class="library-sync"><ShieldCheck size={17} /><div><strong>Local and private by default</strong><span>Desktop Libraries are copied into app storage and attached through ACP resource links. Cloud sharing and indexed citations remain available in Mistral.</span></div><span class="read-only-pill">Reusable</span></div>

  <div class="library-layout">
    <aside class="library-list-panel">
      <label class="page-search"><Search size={14} /><input bind:value={query} placeholder="Search Libraries" /></label>
      <div class="library-list">
        {#each filtered as library (library.id)}
          <button class:active={selected?.id === library.id} on:click={() => (selectedId = library.id)}>
            <span class="library-glyph" style={`--library-color:${library.color}`}><LibraryBig size={15} /></span>
            <div><strong>{library.name}</strong><span>{library.documents.length} {library.documents.length === 1 ? "source" : "sources"}</span></div>
          </button>
        {/each}
        {#if filtered.length === 0}<div class="library-list-empty"><BookOpen size={20} /><span>{libraries.length ? "No Libraries match your search." : "Create your first reusable knowledge base."}</span></div>{/if}
      </div>
    </aside>

    <div class="library-detail">
      {#if selected}
        <div class="library-detail-head">
          <div><span class="library-glyph large" style={`--library-color:${selected.color}`}><LibraryBig size={19} /></span><div><h2>{selected.name}</h2><p>{selected.description || "Reusable documents and webpages"}</p></div></div>
          <div><button class="secondary-button" disabled={busy === selected.id} on:click={() => dispatch("upload", { libraryId: selected.id })}><Upload size={14} /> Add files</button><button class="secondary-button" on:click={() => { webpage = ""; addingWebpage = true; }}><Link2 size={14} /> Webpage</button><button class="icon-button danger" title="Delete Library" on:click={() => (deletingLibrary = selected.id)}><Trash2 size={15} /></button></div>
        </div>
        <div class="document-list-head"><span>Sources</span><span>{selected.documents.length}</span></div>
        <div class="document-list">
          {#each selected.documents as document (document.id)}
            <div class="document-row">
              <span class:web={document.kind === "webpage"} class="document-icon">{#if document.kind === "webpage"}<Globe2 size={15} />{:else}<FileText size={15} />{/if}</span>
              <div><strong>{document.name}</strong><span>{document.kind === "webpage" ? document.source : formatSize(document.size)}</span></div>
              <span class="document-status">{document.status}</span>
              <button class="icon-button small" title="Remove source" on:click={() => (deletingDocument = document)}><X size={14} /></button>
            </div>
          {/each}
          {#if selected.documents.length === 0}<div class="empty-library"><BookOpen size={26} /><strong>This Library is empty</strong><span>Add local documents or a webpage. Attach the Library to a conversation when it is ready.</span><button class="primary-button" on:click={() => dispatch("upload", { libraryId: selected.id })}><Upload size={14} /> Add documents</button></div>{/if}
        </div>
      {:else}
        <div class="empty-library no-selection"><LibraryBig size={28} /><strong>No Library selected</strong><span>Create a Library to collect reusable context.</span><button class="primary-button" on:click={openCreate}><Plus size={14} /> New Library</button></div>
      {/if}
    </div>
  </div>
</section>

{#if creating}
  <div class="modal-backdrop" role="presentation" on:click={(event) => event.target === event.currentTarget && (creating = false)}>
    <form class="library-dialog" on:submit|preventDefault={create}>
      <div class="modal-head"><div><span class="page-kicker">Persistent knowledge</span><h2>New Library</h2></div><button type="button" class="icon-button" on:click={() => (creating = false)}><X size={16} /></button></div>
      <label><span>Name</span><input bind:value={name} maxlength="120" placeholder="Product knowledge" required /></label>
      <label><span>Description</span><textarea bind:value={description} maxlength="500" placeholder="Specs, release notes, and product positioning"></textarea></label>
      <div class="modal-actions"><button type="button" class="secondary-button" on:click={() => (creating = false)}>Cancel</button><button class="primary-button" disabled={!name.trim()}><Plus size={14} /> Create Library</button></div>
    </form>
  </div>
{/if}

{#if addingWebpage && selected}
  <div class="modal-backdrop" role="presentation" on:click={(event) => event.target === event.currentTarget && (addingWebpage = false)}>
    <form class="library-dialog" on:submit|preventDefault={addWebpage}>
      <div class="modal-head"><div><span class="page-kicker">{selected.name}</span><h2>Add webpage</h2></div><button type="button" class="icon-button" on:click={() => (addingWebpage = false)}><X size={16} /></button></div>
      <label><span>URL</span><input type="url" bind:value={webpage} placeholder="https://docs.example.com/product" required /></label>
      <p>Vibe receives the page as an attached ACP resource. Use Mistral Libraries when you need cloud indexing, sharing, and cited passages.</p>
      <div class="modal-actions"><button type="button" class="secondary-button" on:click={() => (addingWebpage = false)}>Cancel</button><button class="primary-button" disabled={!webpage.trim()}><Globe2 size={14} /> Add webpage</button></div>
    </form>
  </div>
{/if}

{#if deletingLibrary && selected}
  <div class="modal-backdrop" role="presentation">
    <div class="library-dialog confirm-dialog"><div class="modal-head"><div><span class="page-kicker">Delete Library</span><h2>Remove {selected.name}?</h2></div></div><p>This removes its local copies and detaches it from every conversation. This cannot be undone.</p><div class="modal-actions"><button class="secondary-button" on:click={() => (deletingLibrary = "")}>Cancel</button><button class="danger-button" on:click={() => { dispatch("delete", { libraryId: deletingLibrary }); deletingLibrary = ""; selectedId = ""; }}><Trash2 size={14} /> Delete Library</button></div></div>
  </div>
{/if}

{#if deletingDocument && selected}
  <div class="modal-backdrop" role="presentation">
    <div class="library-dialog confirm-dialog"><div class="modal-head"><div><span class="page-kicker">Remove source</span><h2>Remove {deletingDocument.name}?</h2></div></div><p>The source will no longer be available when this Library is attached.</p><div class="modal-actions"><button class="secondary-button" on:click={() => (deletingDocument = null)}>Cancel</button><button class="danger-button" on:click={() => { dispatch("deleteDocument", { libraryId: selected.id, documentId: deletingDocument?.id }); deletingDocument = null; }}><Trash2 size={14} /> Remove source</button></div></div>
  </div>
{/if}
