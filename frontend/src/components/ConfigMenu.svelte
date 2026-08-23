<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { Brain, Check, ChevronDown, Cpu } from "@lucide/svelte";
  import type { SessionConfigOption } from "../lib/types";

  export let config: SessionConfigOption;
  export let kind: "model" | "thinking" = "model";
  export let disabled = false;

  const dispatch = createEventDispatcher<{ change: { value: string } }>();
  let open = false;

  $: selected = config.options.find((option) => option.value === config.currentValue) ?? config.options[0];

  function toggle(event: MouseEvent) {
    event.stopPropagation();
    if (!disabled) open = !open;
  }

  function choose(value: string) {
    open = false;
    if (value !== config.currentValue) dispatch("change", { value });
  }

  function closeOnKey(event: KeyboardEvent) {
    if (event.key === "Escape") open = false;
  }
</script>

<svelte:window on:click={() => (open = false)} on:keydown={closeOnKey} />

<div class:open class:thinking={kind === "thinking"} class="config-menu">
  <button
    type="button"
    class="config-trigger"
    title={kind === "model" ? "Active Vibe model" : "Vibe thinking level"}
    disabled={disabled}
    aria-haspopup="listbox"
    aria-expanded={open}
    on:click={toggle}
  >
    {#if kind === "model"}<Cpu size={13} />{:else}<Brain size={13} />{/if}
    <span>{selected?.name || config.currentValue || (kind === "model" ? "Model" : "Thinking")}</span>
    <ChevronDown class="config-chevron" size={11} />
  </button>

  {#if open}
    <div class="config-popover" role="listbox" aria-label={config.name}>
      <div class="config-popover-title">{kind === "model" ? "Choose model" : "Thinking level"}</div>
      {#each config.options as choice}
        <button
          type="button"
          class:selected={choice.value === config.currentValue}
          role="option"
          aria-selected={choice.value === config.currentValue}
          on:click={() => choose(choice.value)}
        >
          <div>
            <strong>{choice.name}</strong>
            {#if choice.description}<span>{choice.description}</span>{/if}
          </div>
          {#if choice.value === config.currentValue}<Check size={13} />{/if}
        </button>
      {/each}
    </div>
  {/if}
</div>
