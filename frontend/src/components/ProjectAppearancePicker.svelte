<script lang="ts">
  import { createEventDispatcher } from "svelte";
  import { Check } from "@lucide/svelte";
  import { projectColorOptions, projectIconOptions } from "../lib/project-icons";
  import ProjectAvatar from "./ProjectAvatar.svelte";

  export let name = "";
  export let icon = "messages";
  export let color = "#ff7417";

  const dispatch = createEventDispatcher<{ change: { icon: string; color: string } }>();

  function chooseIcon(nextIcon: string) {
    icon = nextIcon;
    dispatch("change", { icon, color });
  }

  function chooseColor(nextColor: string) {
    color = nextColor;
    dispatch("change", { icon, color });
  }
</script>

<div class="appearance-picker">
  <div class="appearance-preview">
    <ProjectAvatar name={name || "New group"} {icon} {color} size="large" />
    <div><span>Group appearance</span><strong>{name || "New group"}</strong></div>
  </div>
  <fieldset>
    <legend>Icon</legend>
    <div class="project-icon-grid">
      {#each projectIconOptions as option}
        <button type="button" class:selected={icon === option.id} title={option.label} aria-label={`${option.label} icon`} aria-pressed={icon === option.id} on:click={() => chooseIcon(option.id)}>
          <svelte:component this={option.component} size={16} strokeWidth={2} />
        </button>
      {/each}
    </div>
  </fieldset>
  <fieldset>
    <legend>Color</legend>
    <div class="project-color-grid">
      {#each projectColorOptions as option}
        <button type="button" class:selected={color === option.value} style={`--swatch:${option.value}`} title={option.label} aria-label={`${option.label} color`} aria-pressed={color === option.value} on:click={() => chooseColor(option.value)}>
          {#if color === option.value}<Check size={13} strokeWidth={2.6} />{/if}
        </button>
      {/each}
    </div>
  </fieldset>
</div>
