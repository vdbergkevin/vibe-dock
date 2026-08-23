<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { ArrowUp, AtSign, BookOpen, ChevronDown, Command, Cpu, FolderCog, LibraryBig, Paperclip, Plug, Settings, Square, TerminalSquare, WandSparkles } from "@lucide/svelte";
  import LibraryMenu from "./LibraryMenu.svelte";
  import ModelPicker from "./ModelPicker.svelte";
  import PromptQueue from "./PromptQueue.svelte";
  import type { Library, Mode, SessionConfigOption, SlashCommand, WorkspaceKind } from "../lib/types";

  export let mode: Mode = "default";
  export let running = false;
  export let activePrompt = "";
  export let queuedPrompts: string[] = [];
  export let disabled = false;
  export let acpAvailable = true;
  export let configOptions: SessionConfigOption[] = [];
  export let commands: SlashCommand[] = [];
  export let configLoading = false;
  export let projectKind: WorkspaceKind = "code";
  export let libraries: Library[] = [];
  export let attachedLibraryIds: string[] = [];
  export let editorName = "VSCodium";
  const dispatch = createEventDispatcher();
  let value = "";
  let textarea: HTMLTextAreaElement;
  let modelPicker: ModelPicker;
  let composing = false;
  let commandDismissed = false;
  let commandIndex = 0;
  let desktopCommands: SlashCommand[] = [];
  let allCommands: SlashCommand[] = [];

  const fallbackModes = [
    { value: "default", name: "Default" },
    { value: "plan", name: "Plan" },
    { value: "accept-edits", name: "Accept edits" },
    { value: "auto-approve", name: "Auto approve" }
  ];

  $: modeConfig = configOptions.find((option) => option.id === "mode" || option.category === "mode");
  $: modelConfig = configOptions.find((option) => option.id === "model" || option.category === "model");
  $: thinkingConfig = configOptions.find((option) => option.id === "thinking" || option.category === "thinking" || option.category === "thought_level");
  $: modeChoices = modeConfig?.options.length ? modeConfig.options : fallbackModes;
  $: selectedMode = modeConfig?.currentValue || mode;
  $: promptPlaceholder = projectKind === "work" ? "Describe the outcome you want…" : projectKind === "chat" ? "Ask Vibe anything…" : "Ask Vibe to build, explain, or fix…";
  $: desktopCommands = [
    { name: "model", description: "Choose a Mistral model and response pace", source: "desktop" as const },
    { name: "mcp", description: "Open MCP servers and Mistral connectors", source: "desktop" as const },
    { name: "libraries", description: "Manage reusable context Libraries", source: "desktop" as const },
    { name: "skills", description: "Manage reusable Vibe workflows", source: "desktop" as const },
    { name: "settings", description: "Open VibeDock settings", source: "desktop" as const },
    { name: "new", description: "Start a new conversation", source: "desktop" as const },
    ...(projectKind === "code" ? [
      { name: "terminal", description: "Open this project in Terminal", source: "desktop" as const },
      { name: "editor", description: `Open this project in ${editorName}`, source: "desktop" as const }
    ] : [])
  ];
  $: allCommands = [...desktopCommands, ...commands.filter((command) => !desktopCommands.some((local) => local.name === command.name))];
  $: slashMatch = value.match(/^\/([^\s]*)$/);
  $: slashQuery = slashMatch?.[1]?.toLowerCase() || "";
  $: commandResults = slashMatch ? allCommands.filter((command) => `${command.name} ${command.description}`.toLowerCase().includes(slashQuery)).slice(0, 8) : [];
  $: commandOpen = !commandDismissed && Boolean(slashMatch) && commandResults.length > 0;
  $: if (commandIndex >= commandResults.length) commandIndex = 0;

  function resize() {
    if (!textarea) return;
    textarea.style.height = "0px";
    textarea.style.height = `${Math.min(180, Math.max(52, textarea.scrollHeight))}px`;
  }

  function input() {
    resize();
    modelPicker?.closePicker();
    commandDismissed = false;
    commandIndex = 0;
  }

  function send() {
    const text = value.trim();
    if (!text || disabled || !acpAvailable) return;
    dispatch("send", { text });
    value = "";
    requestAnimationFrame(resize);
  }

  function keydown(event: KeyboardEvent) {
    const enter = event.key === "Enter" || event.code === "Enter" || event.code === "NumpadEnter" || event.keyCode === 13;
    if (commandOpen) {
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        commandIndex = (commandIndex + (event.key === "ArrowDown" ? 1 : -1) + commandResults.length) % commandResults.length;
        return;
      }
      if (event.key === "Escape") {
        event.preventDefault();
        commandDismissed = true;
        return;
      }
      if ((enter || event.key === "Tab") && !event.isComposing && !composing) {
        event.preventDefault();
        chooseCommand(commandResults[commandIndex]);
        return;
      }
    }
    if (!enter || event.shiftKey || event.isComposing || composing || event.keyCode === 229) return;
    event.preventDefault();
    send();
  }

  function chooseCommand(command: SlashCommand | undefined) {
    if (!command) return;
    commandDismissed = true;
    if (command.source === "desktop") {
      value = "";
      if (command.name === "model") modelPicker?.openPicker();
      else dispatch("command", { name: command.name });
      requestAnimationFrame(() => { resize(); textarea?.focus(); });
      return;
    }
    value = `/${command.name}${command.inputHint ? " " : ""}`;
    requestAnimationFrame(() => { resize(); textarea?.focus(); });
    if (!command.inputHint) send();
  }

</script>

<div class="composer-wrap">
  <PromptQueue {activePrompt} {queuedPrompts} on:remove={(event) => dispatch("removeQueued", event.detail)} />
  {#if !acpAvailable}<div class="acp-warning">`vibe-acp` was not found. You can explore projects, but sending requires Mistral Vibe.</div>{/if}
  <div class:focused={value.length > 0} class="composer">
    {#if commandOpen}
      <div class="slash-command-popover" role="listbox" aria-label="Slash commands">
        <div class="slash-command-head"><Command size={13} /><span>Commands</span><kbd>↑↓ navigate</kbd></div>
        {#each commandResults as command, index (command.source + command.name)}
          <button type="button" class:selected={index === commandIndex} role="option" aria-selected={index === commandIndex} on:mousedown|preventDefault={() => chooseCommand(command)}>
            <span class="slash-command-icon">
              {#if command.name === "model"}<Cpu size={14} />{:else if command.name === "mcp"}<Plug size={14} />{:else if command.name === "libraries"}<LibraryBig size={14} />{:else if command.name === "skills"}<WandSparkles size={14} />{:else if command.name === "settings"}<Settings size={14} />{:else if command.name === "terminal"}<TerminalSquare size={14} />{:else if command.name === "editor"}<FolderCog size={14} />{:else if command.name === "new"}<BookOpen size={14} />{:else}<Command size={14} />{/if}
            </span>
            <div><strong>/{command.name}</strong><span>{command.description}</span></div>
            {#if command.inputHint}<code>{command.inputHint}</code>{/if}
            <span class:desktop={command.source === "desktop"} class="command-source">{command.source === "desktop" ? "App" : "Vibe"}</span>
          </button>
        {/each}
        <div class="slash-command-foot"><span><kbd>tab</kbd> complete</span><span><kbd>esc</kbd> close</span></div>
      </div>
    {/if}
    <textarea bind:this={textarea} bind:value on:input={input} on:keydown={keydown} on:compositionstart={() => (composing = true)} on:compositionend={() => (composing = false)} placeholder={promptPlaceholder} aria-label="Message Vibe" enterkeyhint="send" disabled={disabled}></textarea>
    <div class="composer-toolbar">
      <div class="composer-left">
        <button class="composer-icon" title="Attach files"><Paperclip size={15} /></button>
        <LibraryMenu {libraries} attachedIds={attachedLibraryIds} disabled={disabled || running} on:change={(event) => dispatch("libraries", event.detail)} on:manage={() => dispatch("manageLibraries")} />
        <button class="composer-icon" title="Mention files"><AtSign size={15} /></button>
        {#if projectKind === "code"}
          <label class="mode-select" title="Active Vibe agent">
            <WandSparkles size={14} />
            <select value={selectedMode} disabled={disabled || running || configLoading || !acpAvailable} aria-label="Active Vibe agent" on:change={(event) => dispatch("mode", { mode: event.currentTarget.value })}>
              {#each modeChoices as choice}<option value={choice.value}>{choice.name}</option>{/each}
            </select>
            <ChevronDown size={12} />
          </label>
        {/if}
        {#if modelConfig}
          <ModelPicker bind:this={modelPicker} {modelConfig} speedConfig={thinkingConfig} disabled={disabled || running || configLoading || !acpAvailable} on:open={() => (commandDismissed = true)} on:change={(event) => dispatch("config", event.detail)} />
        {:else}
          <span class:loading={configLoading} class="model-pill"><Cpu size={13} /> {configLoading ? "Loading models…" : "Vibe model"}</span>
        {/if}
      </div>
      {#if running}
        <button class="send-button stop" title="Stop" on:click={() => dispatch("cancel")}><Square size={12} fill="currentColor" /></button>
      {:else}
        <button class="send-button" class:ready={value.trim().length > 0} disabled={!value.trim() || disabled || !acpAvailable} title="Send message" on:click={send}><ArrowUp size={16} /></button>
      {/if}
    </div>
  </div>
  <div class="composer-hint">{queuedPrompts.length ? "Follow-ups run in order · Enter to add another" : running ? "Enter to queue a follow-up · Shift+Enter for a new line" : "Enter to send · Shift+Enter for a new line"}</div>
</div>
