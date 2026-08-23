<script lang="ts">
  import { CheckCircle2, CircleAlert, CreditCard, Cpu, Database, ExternalLink, Gauge, GitBranch, KeyRound, Keyboard, LogIn, Moon, Palette, Receipt, RefreshCw, Settings2, Shield } from "@lucide/svelte";
  import { createEventDispatcher } from "svelte";
  import type { MistralDestination } from "../lib/mistral-links";
  import type { AccentTheme, CodeEditor, Environment, Theme } from "../lib/types";
  import BrandIcon from "./BrandIcon.svelte";
  export let environment: Environment;
  export let theme: Theme;
  export let accentTheme: AccentTheme;
  export let accountRefreshing = false;
  export let editors: CodeEditor[] = [];
  export let codeEditor: CodeEditor;
  const dispatch = createEventDispatcher<{ theme: { theme: Theme }; accent: { theme: AccentTheme }; editor: { editorId: CodeEditor["id"] }; external: { destination: MistralDestination }; setup: void; refreshAccount: void }>();

  const themeDescription: Record<Theme, string> = {
    dark: "Dark appearance",
    light: "Light appearance",
    system: "Follows the macOS appearance"
  };

  const accentThemes: Array<{ id: AccentTheme; name: string; start: string; end: string }> = [
    { id: "mistral", name: "Mistral", start: "#ffb51b", end: "#ff7417" },
    { id: "tide", name: "Tide", start: "#6dd5c5", end: "#2f9fc3" },
    { id: "grove", name: "Grove", start: "#9bcf77", end: "#4fa77b" },
    { id: "cobalt", name: "Cobalt", start: "#8dc4ff", end: "#5c91d8" },
    { id: "orchid", name: "Orchid", start: "#e4a5e8", end: "#a879d6" }
  ];

  $: accentName = accentThemes.find((item) => item.id === accentTheme)?.name || "Mistral";
</script>

<section class="page-view settings-view">
  <header class="page-header"><div><span class="page-kicker">VibeDock</span><h1><Settings2 size={22} /> Settings</h1><p>Runtime, appearance, and safety preferences.</p></div></header>
  <div class="settings-columns">
    <div class="settings-group">
      <h2>Mistral account</h2>
      <div class:ready={environment.account.configured} class="account-hero">
        <span class="account-hero-logo"><BrandIcon name="Mistral AI" size={24} /></span>
        <div><span class="account-state">{environment.account.configured ? "Connected to Vibe" : "Account setup"}</span><strong>{environment.account.configured ? "Mistral is ready" : "Sign in to Mistral AI"}</strong><p>{environment.account.detail}</p></div>
        <div class="account-actions">
          <button class="primary-button" disabled={!environment.account.available} on:click={() => dispatch("setup")}><LogIn size={13} /> {environment.account.configured ? "Change account" : "Sign in"}</button>
          <button class="icon-button" disabled={accountRefreshing} title="Refresh account status" on:click={() => dispatch("refreshAccount")}><RefreshCw class={accountRefreshing ? "spin" : ""} size={14} /></button>
        </div>
      </div>
      <div class="setting-row"><div class="setting-icon"><KeyRound size={17} /></div><div><strong>API keys</strong><span>Create and rotate your personal keys in Studio</span></div><button class="setting-value external-setting" on:click={() => dispatch("external", { destination: "apiKeys" })}>Open <ExternalLink size={12} /></button></div>
      <div class="setting-row"><div class="setting-icon"><Gauge size={17} /></div><div><strong>Usage &amp; limits</strong><span>Review Vibe, model, token, and connector usage</span></div><button class="setting-value external-setting" on:click={() => dispatch("external", { destination: "usage" })}>Open <ExternalLink size={12} /></button></div>
      <div class="setting-row"><div class="setting-icon"><CreditCard size={17} /></div><div><strong>Plan &amp; pay-as-you-go</strong><span>Manage your subscription and included usage</span></div><button class="setting-value external-setting" on:click={() => dispatch("external", { destination: "subscription" })}>Open <ExternalLink size={12} /></button></div>
      <div class="setting-row"><div class="setting-icon"><Receipt size={17} /></div><div><strong>Billing &amp; spending</strong><span>Payment methods, credits, limits, and invoices</span></div><button class="setting-value external-setting" on:click={() => dispatch("external", { destination: "billing" })}>Open <ExternalLink size={12} /></button></div>
    </div>
    <div class="settings-group">
      <h2>Runtime</h2>
      <div class="setting-row"><div class="setting-icon"><Cpu size={17} /></div><div><strong>Mistral Vibe</strong><span>{environment.vibeVersion || "Not detected"}</span></div>{#if environment.vibeAvailable}<CheckCircle2 class="ok" size={17} />{:else}<CircleAlert class="warn" size={17} />{/if}</div>
      <div class="setting-row"><div class="setting-icon"><Database size={17} /></div><div><strong>Agent Client Protocol</strong><span>{environment.acpPath || "vibe-acp is not on PATH"}</span></div>{#if environment.acpAvailable}<CheckCircle2 class="ok" size={17} />{:else}<CircleAlert class="warn" size={17} />{/if}</div>
      <div class="setting-row"><div class="setting-icon"><GitBranch size={17} /></div><div><strong>Git integration</strong><span>{environment.gitAvailable ? "Available" : "Git is not on PATH"}</span></div>{#if environment.gitAvailable}<CheckCircle2 class="ok" size={17} />{:else}<CircleAlert class="warn" size={17} />{/if}</div>
    </div>
    <div class="settings-group">
      <h2>Preferences</h2>
      <div class="setting-row"><div class="setting-icon"><Moon size={17} /></div><div><strong>Appearance</strong><span>{themeDescription[theme]}</span></div><select class="setting-value theme-select" value={theme} aria-label="Appearance" on:change={(event) => dispatch("theme", { theme: event.currentTarget.value as Theme })}><option value="system">System</option><option value="dark">Dark</option><option value="light">Light</option></select></div>
      <div class="setting-row colour-theme-setting">
        <div class="setting-icon"><Palette size={17} /></div>
        <div><strong>Colour theme</strong><span>{accentName} accents across VibeDock</span></div>
        <div class="accent-theme-options" role="radiogroup" aria-label="Colour theme">
          {#each accentThemes as item}
            <button role="radio" aria-checked={accentTheme === item.id} class:active={accentTheme === item.id} title={item.name} style={`--swatch-start:${item.start};--swatch-end:${item.end}`} on:click={() => dispatch("accent", { theme: item.id })}><span></span></button>
          {/each}
        </div>
      </div>
      <div class="setting-row"><div class="setting-icon editor-setting-icon"><BrandIcon name={codeEditor.icon} size={17} /></div><div><strong>Code editor</strong><span>{codeEditor.available ? `Open every code project in ${codeEditor.name}` : `${codeEditor.name} is not installed`}</span></div><select class="setting-value editor-select" value={codeEditor.id} aria-label="Code editor" on:change={(event) => dispatch("editor", { editorId: event.currentTarget.value as CodeEditor["id"] })}>{#each editors as editor}<option value={editor.id} disabled={!editor.available}>{editor.name}{editor.available ? "" : " · Not installed"}</option>{/each}</select></div>
      <div class="setting-row"><div class="setting-icon"><Shield size={17} /></div><div><strong>Default permissions</strong><span>Ask before commands and edits</span></div><button class="setting-value">Default</button></div>
      <div class="setting-row"><div class="setting-icon"><Keyboard size={17} /></div><div><strong>Keyboard shortcuts</strong><span>Native macOS command bindings</span></div><button class="setting-value">View</button></div>
    </div>
  </div>
</section>
