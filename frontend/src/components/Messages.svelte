<script lang="ts">
  import { createEventDispatcher, onMount } from "svelte";
  import { Check, ChevronDown, Circle, CircleAlert, FileCode2, LoaderCircle, ShieldAlert, Terminal, Wrench, X } from "@lucide/svelte";
  import { markdown } from "../lib/markdown";
  import BrandIcon from "./BrandIcon.svelte";
  import type { Message, StreamState, ToolState } from "../lib/types";

  export let messages: Message[] = [];
  export let stream: StreamState;
  const dispatch = createEventDispatcher();
  let now = Date.now();

  const workingMessages = [
    "Getting the bearings",
    "Reading the room—and the repo",
    "Connecting the dots",
    "Turning ideas into diffs",
    "Checking the sharp edges",
    "Polishing the result",
    "Giving it one last look"
  ];

  $: liveDurationMs = stream.durationMs || (stream.startedAt ? Math.max(0, now - stream.startedAt) : 0);
  $: workingMessage = workingMessages[Math.floor(liveDurationMs / 5000) % workingMessages.length];

  onMount(() => {
    const timer = window.setInterval(() => {
      if (stream.running) now = Date.now();
    }, 1000);
    return () => window.clearInterval(timer);
  });

  function locationName(value: unknown) {
    return String(value).split("/").pop();
  }

  function toolIcon(tool: ToolState) {
    return tool.kind === "execute" ? Terminal : tool.kind === "edit" || tool.kind === "read" ? FileCode2 : Wrench;
  }

  function messageDuration(message: Message) {
    const duration = Number(message.metadata?.durationMs || 0);
    return Number.isFinite(duration) && duration > 0 ? duration : 0;
  }

  function formatDuration(durationMs: number) {
    const seconds = Math.max(1, Math.round(durationMs / 1000));
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;
    if (minutes < 60) return `${minutes}m${remainingSeconds ? ` ${remainingSeconds}s` : ""}`;
    const hours = Math.floor(minutes / 60);
    const remainingMinutes = minutes % 60;
    return `${hours}h${remainingMinutes ? ` ${remainingMinutes}m` : ""}`;
  }
</script>

<div class="message-list">
  {#each messages as message (message.id)}
    {#if message.role === "user"}
      <article class="message user-message">
        <div class="user-bubble">{message.content}</div>
        <div class="message-meta">You</div>
      </article>
    {:else if message.kind === "thought"}
      <details class="thought-block">
        <summary><span class="thought-spark">✦</span><span>Reasoning</span><ChevronDown size={13} /></summary>
        <div>{message.content}</div>
      </details>
    {:else if message.kind === "tool"}
      <article class="persisted-tool tool-card complete">
        <div class="tool-icon"><Check size={14} /></div>
        <div class="tool-copy">
          <strong>{String(message.metadata?.title || message.content)}</strong>
          {#if Array.isArray(message.metadata?.locations)}
            <div class="tool-locations">{#each message.metadata.locations as location}<span>{locationName(location)}</span>{/each}</div>
          {/if}
        </div>
      </article>
    {:else}
      <article class:failed={message.kind === "error"} class="message assistant-message">
        <div class="assistant-meta">
          <span class="mistral-author"><BrandIcon name="Mistral AI" size={11} /><span>Mistral AI</span></span>
          {#if messageDuration(message)}<span class="worked-time">Worked for {formatDuration(messageDuration(message))}</span>{/if}
        </div>
        <div class="assistant-body prose">{@html markdown(message.content)}</div>
      </article>
    {/if}
  {/each}

  {#if stream.running || stream.answer || stream.thought || stream.tools.length || stream.error}
    <section class="live-response" aria-live="polite">
      {#if stream.thought}
        <details class="thought-block live" open={!stream.answer}>
          <summary><span class="thought-spark">✦</span><span>{stream.answer ? "Reasoning" : "Thinking"}</span><ChevronDown size={13} /></summary>
          <div>{stream.thought}</div>
        </details>
      {/if}

      {#if stream.plan.length}
        <div class="plan-card">
          <div class="plan-title"><span>Plan</span><span>{stream.plan.filter((item) => item.status === "completed").length}/{stream.plan.length}</span></div>
          {#each stream.plan as item}
            <div class="plan-item">
              {#if item.status === "completed"}<Check size={14} />{:else if item.status === "in_progress"}<LoaderCircle class="spin" size={14} />{:else}<Circle size={12} />{/if}
              <span>{item.content}</span>
            </div>
          {/each}
        </div>
      {/if}

      {#each stream.tools as tool (tool.id)}
        <article class:complete={tool.status === "completed"} class:failed={tool.status === "failed"} class="tool-card">
          <div class="tool-icon">
            {#if tool.status === "completed"}<Check size={14} />{:else if tool.status === "failed"}<X size={14} />{:else}<svelte:component this={toolIcon(tool)} size={14} />{/if}
          </div>
          <div class="tool-copy">
            <strong>{tool.title || "Using tool"}</strong>
            {#if tool.locations?.length}<div class="tool-locations">{#each tool.locations as location}<span>{locationName(location.path)}</span>{/each}</div>{/if}
          </div>
          {#if tool.status === "in_progress"}<LoaderCircle class="spin tool-spinner" size={14} />{/if}
        </article>
      {/each}

      {#if stream.permission}
        <div class="permission-card">
          <div class="permission-icon"><ShieldAlert size={18} /></div>
          <div class="permission-content">
            <strong>{stream.permission.title}</strong>
            <p>Vibe needs permission to run this {stream.permission.kind} operation.</p>
            <div class="permission-actions">
              {#each stream.permission.options as option}
                <button class:primary={option.kind.startsWith("allow")} on:click={() => dispatch("permission", { requestId: stream.permission?.requestId, optionId: option.id })}>{option.name}</button>
              {/each}
            </div>
          </div>
        </div>
      {/if}

      {#if stream.answer}
        <article class="message assistant-message live-message">
          <div class="assistant-meta">
            <span class="mistral-author"><BrandIcon name="Mistral AI" size={11} /><span>Mistral AI</span></span>
          </div>
          <div class="assistant-body prose">{@html markdown(stream.answer)}{#if stream.running}<span class="stream-caret"></span>{/if}</div>
          {#if stream.running}
            <div class="working-row response-working-status"><span class="thinking-dots" aria-hidden="true"><i></i><i></i><i></i></span><span>{workingMessage}</span><span class="working-elapsed">{formatDuration(liveDurationMs)} elapsed</span></div>
          {/if}
        </article>
      {:else if stream.running && !stream.permission}
        <div class="working-row"><span class="thinking-dots" aria-hidden="true"><i></i><i></i><i></i></span><span>{workingMessage}</span><span class="working-elapsed">{formatDuration(liveDurationMs)} elapsed</span></div>
      {/if}

      {#if stream.error}
        <div class="stream-error"><CircleAlert size={16} /><div><strong>Vibe stopped</strong><span>{stream.error}</span></div></div>
      {/if}
    </section>
  {/if}
</div>
