<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { Brain, Check, ChevronDown, Gauge, Sparkles, Zap } from "@lucide/svelte";
  import type { SessionConfigChoice, SessionConfigOption } from "../lib/types";
  import BrandIcon from "./BrandIcon.svelte";

  export let modelConfig: SessionConfigOption | undefined;
  export let speedConfig: SessionConfigOption | undefined;
  export let disabled = false;

  const dispatch = createEventDispatcher<{ change: { configId: string; value: string }; open: void }>();
  let open = false;
  let root: HTMLDivElement;

  $: selectedModel = modelConfig?.options.find((choice) => choice.value === modelConfig?.currentValue) ?? modelConfig?.options[0];
  $: selectedSpeed = speedConfig?.options.find((choice) => choice.value === speedConfig?.currentValue) ?? speedConfig?.options[0];

  export function openPicker() {
    if (!disabled && modelConfig) {
      open = true;
      dispatch("open");
    }
  }

  export function closePicker() {
    open = false;
  }

  function toggle(event: MouseEvent) {
    event.stopPropagation();
    if (!disabled) {
      open = !open;
      if (open) dispatch("open");
    }
  }

  function choose(config: SessionConfigOption | undefined, choice: SessionConfigChoice) {
    if (config && choice.value !== config.currentValue) dispatch("change", { configId: config.id, value: choice.value });
  }

  function speedIcon(value: string, index: number, total: number) {
    const normalized = value.toLowerCase();
    if (normalized.includes("off") || normalized.includes("low") || normalized.includes("fast") || index === 0) return Zap;
    if (normalized.includes("high") || normalized.includes("max") || normalized.includes("deep") || index === total - 1) return Brain;
    return Gauge;
  }

  function closeOutside(event: MouseEvent) {
    if (open && !root?.contains(event.target as Node)) open = false;
  }
</script>

<svelte:window on:click={closeOutside} on:keydown={(event) => event.key === "Escape" && (open = false)} />

<div bind:this={root} class:open class="model-picker">
  <button type="button" class="model-picker-trigger" title="Choose Mistral model and response speed" {disabled} aria-haspopup="dialog" aria-expanded={open} on:click={toggle}>
    <span class="model-trigger-logo"><BrandIcon name="Mistral AI" size={13} /></span>
    <span class="model-trigger-name">{selectedModel?.name || "Mistral model"}</span>
    {#if selectedSpeed}<span class="speed-badge"><Zap size={9} /> {selectedSpeed.name}</span>{/if}
    <ChevronDown class="config-chevron" size={11} />
  </button>

  {#if open}
    <div class="model-picker-popover" role="dialog" aria-label="Choose model and speed" tabindex="-1">
      <div class="model-picker-hero">
        <span class="model-orbit"><BrandIcon name="Mistral AI" size={22} /></span>
        <div><span>Your Mistral</span><strong>Pick the right mind for the moment</strong></div>
        <Sparkles size={16} />
      </div>

      <div class="model-section-title"><span>Model</span><span>Live from Vibe</span></div>
      <div class="model-choice-list" role="listbox" aria-label={modelConfig?.name || "Model"}>
        {#each modelConfig?.options || [] as choice, index}
          <button type="button" class:selected={choice.value === modelConfig?.currentValue} role="option" aria-selected={choice.value === modelConfig?.currentValue} on:click={() => choose(modelConfig, choice)}>
            <span class="model-choice-glyph">{index === 0 ? "M" : index + 1}</span>
            <div><strong>{choice.name}</strong><span>{choice.description || choice.value}</span></div>
            {#if choice.value === modelConfig?.currentValue}<span class="selected-check"><Check size={12} /></span>{/if}
          </button>
        {/each}
      </div>

      {#if speedConfig?.options.length}
        <div class="model-section-title speed-title"><span>Response pace</span><span>Thinking depth</span></div>
        <div class="speed-choice-list" style={`--speed-count:${Math.min(speedConfig.options.length, 5)}`}>
          {#each speedConfig.options as choice, index}
            <button type="button" class:selected={choice.value === speedConfig?.currentValue} title={choice.description || choice.name} on:click={() => choose(speedConfig, choice)}>
              <svelte:component this={speedIcon(choice.value, index, speedConfig?.options.length || 0)} size={13} />
              <span>{choice.name}</span>
            </button>
          {/each}
        </div>
        <p class="speed-explainer"><Zap size={11} /> Faster replies use less thinking; deeper settings spend more time reasoning before answering.</p>
      {/if}
    </div>
  {/if}
</div>
