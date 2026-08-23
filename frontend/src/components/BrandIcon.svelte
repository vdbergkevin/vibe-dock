<script lang="ts">
  import {
    siAtlassian,
    siBox,
    siCursor,
    siGithub,
    siGmail,
    siGooglebigquery,
    siGooglecalendar,
    siGoogledrive,
    siLinear,
    siMistralai,
    siNotion,
    siStripe,
    siSublimetext,
    siVscodium,
    siZedindustries,
    type SimpleIcon
  } from "simple-icons";

  export let name = "";
  export let size = 20;

  const icons: Record<string, SimpleIcon> = {
    atlassian: siAtlassian,
    bigquery: siGooglebigquery,
    googlebigquery: siGooglebigquery,
    box: siBox,
    cursor: siCursor,
    github: siGithub,
    githubapp: siGithub,
    gmail: siGmail,
    googlecalendar: siGooglecalendar,
    googledrive: siGoogledrive,
    googledrivemcp: siGoogledrive,
    linear: siLinear,
    mistral: siMistralai,
    mistralai: siMistralai,
    notion: siNotion,
    stripe: siStripe,
    sublime: siSublimetext,
    sublimetext: siSublimetext,
    vscodium: siVscodium,
    zed: siZedindustries,
    zedindustries: siZedindustries
  };

  $: key = normalise(name);
  $: icon = icons[key];
  $: custom = key === "slack" ? "slack" : key === "vscode" || key === "visualstudiocode" ? "vscode" : key.startsWith("outlook") ? "outlook" : key.startsWith("sharepoint") ? "sharepoint" : "";
  $: fallback = initials(name);

  function normalise(value: string) {
    return value.toLowerCase().replace(/[^a-z0-9]/g, "");
  }

  function initials(value: string) {
    const words = value.trim().split(/[_\s-]+/).filter(Boolean);
    return (words.length > 1 ? words.slice(0, 2).map((word) => word[0]).join("") : value.slice(0, 2)).toUpperCase() || "MCP";
  }

  function colour(brand: SimpleIcon) {
    return brand.hex === "000000" || brand.hex === "181717" ? "var(--brand-monochrome)" : `#${brand.hex}`;
  }
</script>

{#if icon}
  <svg class="brand-icon" width={size} height={size} viewBox="0 0 24 24" aria-hidden="true" style={`color:${colour(icon)}`}>
    <path fill="currentColor" d={icon.path} />
  </svg>
{:else if custom === "slack"}
  <svg class="brand-icon" width={size} height={size} viewBox="0 0 24 24" aria-hidden="true">
    <path fill="#36C5F0" d="M5.2 13.7a2.2 2.2 0 1 1 0-4.4h2.2v4.4H5.2Zm3.3 0a2.2 2.2 0 0 1 4.4 0v5.5a2.2 2.2 0 1 1-4.4 0v-5.5Z" />
    <path fill="#2EB67D" d="M10.3 5.2a2.2 2.2 0 1 1 4.4 0v2.2h-4.4V5.2Zm0 3.3a2.2 2.2 0 0 1 0 4.4H4.8a2.2 2.2 0 1 1 0-4.4h5.5Z" />
    <path fill="#ECB22E" d="M18.8 10.3a2.2 2.2 0 1 1 0 4.4h-2.2v-4.4h2.2Zm-3.3 0a2.2 2.2 0 0 1-4.4 0V4.8a2.2 2.2 0 1 1 4.4 0v5.5Z" />
    <path fill="#E01E5A" d="M13.7 18.8a2.2 2.2 0 1 1-4.4 0v-2.2h4.4v2.2Zm0-3.3a2.2 2.2 0 0 1 0-4.4h5.5a2.2 2.2 0 1 1 0 4.4h-5.5Z" />
  </svg>
{:else if custom === "vscode"}
  <svg class="brand-icon" width={size} height={size} viewBox="0 0 24 24" aria-hidden="true">
    <path fill="#23A8F2" d="M17.1 2.2 9 9.6 4.1 5.9 1.8 7.1v9.8l2.3 1.2L9 14.4l8.1 7.4 5.1-2.1V4.3l-5.1-2.1ZM4.2 14.5v-5L7 12l-2.8 2.5Zm12.6 1.4L11.5 12l5.3-3.9v7.8Z" />
  </svg>
{:else if custom === "outlook"}
  <svg class="brand-icon" width={size} height={size} viewBox="0 0 24 24" aria-hidden="true">
    <path fill="#0A64AD" d="M9.3 5.1h11.2c.8 0 1.5.7 1.5 1.5v10.8c0 .8-.7 1.5-1.5 1.5H9.3V5.1Z" />
    <path fill="#28A8EA" d="m9.3 8 6.2 4.3L22 7.8v9.6c0 .8-.7 1.5-1.5 1.5H9.3V8Z" />
    <path fill="#1473E6" d="M2 3.5 13.1 2v20L2 20.2V3.5Z" />
    <path fill="#fff" d="M4.5 11.9c0-2.5 1.2-4.1 3.2-4.1 1.9 0 3.1 1.5 3.1 3.9 0 2.5-1.2 4.1-3.2 4.1-1.9 0-3.1-1.5-3.1-3.9Zm1.8-.1c0 1.5.5 2.4 1.4 2.4.9 0 1.4-.9 1.4-2.4s-.5-2.4-1.4-2.4c-.9 0-1.4.9-1.4 2.4Z" />
  </svg>
{:else if custom === "sharepoint"}
  <svg class="brand-icon" width={size} height={size} viewBox="0 0 24 24" aria-hidden="true">
    <circle cx="15.7" cy="7" r="5" fill="#37C6D0" />
    <circle cx="16.6" cy="16" r="6" fill="#038387" />
    <circle cx="8.2" cy="12" r="6.2" fill="#036C70" />
    <path fill="#fff" d="M5.2 15.1c.8.5 1.8.8 2.8.8.9 0 1.4-.3 1.4-.9 0-.5-.4-.8-1.6-1.2-1.7-.6-2.8-1.5-2.8-3 0-1.8 1.5-3 3.8-3 1.2 0 2.1.2 2.8.6l-.6 1.8c-.5-.3-1.3-.6-2.3-.6-.9 0-1.3.4-1.3.8 0 .6.5.8 1.8 1.3 1.8.7 2.6 1.6 2.6 3 0 1.7-1.3 3.1-4 3.1-1.1 0-2.3-.3-2.9-.7l.3-2Z" />
  </svg>
{:else}
  <span class="brand-fallback" aria-hidden="true" style={`width:${size}px;height:${size}px;font-size:${Math.max(7, size * 0.38)}px`}>{fallback}</span>
{/if}
