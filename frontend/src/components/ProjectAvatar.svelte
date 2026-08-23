<script lang="ts">
  import { projectIconComponent } from "../lib/project-icons";

  export let name: string;
  export let icon = "";
  export let color = "#ff7417";
  export let size: "normal" | "small" | "large" = "normal";

  $: Icon = projectIconComponent(icon);
  $: iconSize = size === "large" ? 25 : size === "small" ? 14 : 12;
</script>

<span class:small={size === "small"} class:large={size === "large"} class="project-glyph project-avatar" style={`--project-color:${color}`} aria-hidden="true">
  {#if Icon}
    <svelte:component this={Icon} size={iconSize} strokeWidth={2.05} />
  {:else}
    {name.slice(0, 1).toUpperCase()}
  {/if}
</span>
