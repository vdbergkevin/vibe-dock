<script lang="ts">
  import { createEventDispatcher, onMount } from "svelte";
  import { Copy, ExternalLink, LoaderCircle, RotateCw, TerminalSquare, Trash2, X } from "@lucide/svelte";
  import type { Terminal as XTerminalInstance } from "@xterm/xterm";
  import type { FitAddon as FitAddonInstance } from "@xterm/addon-fit";
  import "@xterm/xterm/css/xterm.css";
  import { api } from "../lib/api";
  import type { Project, TerminalSession } from "../lib/types";

  export let project: Project;
  export let height = 270;

  const dispatch = createEventDispatcher<{ close: void; external: void; resize: { height: number } }>();
  let host: HTMLDivElement;
  let terminal: XTerminalInstance | undefined;
  let fitAddon: FitAddonInstance | undefined;
  let session: TerminalSession | null = null;
  let starting = true;
  let exited = false;
  let error = "";
  let copied = false;
  let destroyed = false;
  let writeQueue: Promise<void> = Promise.resolve();
  let cleanupResize = () => {};

  function clampHeight(value: number) {
    const minimum = Math.min(180, Math.max(130, window.innerHeight - 360));
    const maximum = Math.max(minimum, window.innerHeight - 220);
    return Math.max(minimum, Math.min(maximum, value));
  }

  function beginResize(event: PointerEvent) {
    event.preventDefault();
    cleanupResize();
    const startY = event.clientY;
    const startHeight = height;
    const move = (nextEvent: PointerEvent) => {
      dispatch("resize", { height: clampHeight(startHeight + startY - nextEvent.clientY) });
    };
    const stop = () => cleanupResize();
    document.body.classList.add("terminal-resizing");
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", stop, { once: true });
    window.addEventListener("pointercancel", stop, { once: true });
    cleanupResize = () => {
      document.body.classList.remove("terminal-resizing");
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", stop);
      window.removeEventListener("pointercancel", stop);
      cleanupResize = () => {};
    };
  }

  function resizeWithKeyboard(event: KeyboardEvent) {
    if (event.key !== "ArrowUp" && event.key !== "ArrowDown") return;
    event.preventDefault();
    dispatch("resize", { height: clampHeight(height + (event.key === "ArrowUp" ? 24 : -24)) });
  }

  function xtermTheme() {
    const light = document.documentElement.dataset.resolvedTheme === "light";
    return light ? {
      background: "#fbfbf9", foreground: "#373438", cursor: "#dc5514", cursorAccent: "#fbfbf9",
      selectionBackground: "#f1c9b2", black: "#2b292d", red: "#b95755", green: "#388266", yellow: "#956b25",
      blue: "#487fa8", magenta: "#9c5f92", cyan: "#38828a", white: "#ddd9d5", brightBlack: "#77737a",
      brightRed: "#d36a5f", brightGreen: "#4c987a", brightYellow: "#b9822e", brightBlue: "#6096bd",
      brightMagenta: "#b677a9", brightCyan: "#50a0a8", brightWhite: "#ffffff"
    } : {
      background: "#0d0d0f", foreground: "#d8d7da", cursor: "#ff7417", cursorAccent: "#0d0d0f",
      selectionBackground: "#4b2c1e", black: "#171719", red: "#e2817f", green: "#65b996", yellow: "#d5a85d",
      blue: "#6c9bc0", magenta: "#b784ad", cyan: "#62a8ae", white: "#d8d7da", brightBlack: "#737178",
      brightRed: "#f0928d", brightGreen: "#7bc9a7", brightYellow: "#e5ba70", brightBlue: "#82afd0",
      brightMagenta: "#c998bf", brightCyan: "#78bdc3", brightWhite: "#f2f1ee"
    };
  }

  function fit() {
    if (!terminal || !fitAddon || !host?.clientWidth || !host?.clientHeight) return;
    try {
      fitAddon.fit();
      if (session) void api.resizeProjectTerminal(session.id, terminal.cols, terminal.rows).catch(() => {});
    } catch {
      // The panel may be between layout states while opening or closing.
    }
  }

  async function start() {
    if (!terminal) return;
    starting = true;
    exited = false;
    error = "";
    try {
      fit();
      const next = await api.startProjectTerminal(project.id, terminal.cols || 100, terminal.rows || 20);
      if (destroyed) {
        await api.stopProjectTerminal(next.id);
        return;
      }
      session = next;
      await api.attachProjectTerminal(next.id);
      fit();
      terminal.focus();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
      terminal.writeln(`\r\n\u001b[31mCould not start terminal: ${error}\u001b[0m`);
    } finally {
      starting = false;
    }
  }

  async function restart() {
    if (!terminal) return;
    const previous = session;
    session = null;
    if (previous) await api.stopProjectTerminal(previous.id).catch(() => {});
    terminal.reset();
    await start();
  }

  function clearTerminal() {
    if (!terminal) return;
    terminal.clear();
    terminal.write("\u001b[2J\u001b[H");
    terminal.focus();
  }

  async function copyTerminal() {
    if (!terminal) return;
    const hadSelection = terminal.hasSelection();
    if (!hadSelection) terminal.selectAll();
    const text = terminal.getSelection();
    if (!hadSelection) terminal.clearSelection();
    if (!text) return;
    try {
      await navigator.clipboard.writeText(text);
      copied = true;
      window.setTimeout(() => copied = false, 1200);
    } catch {
      error = "Clipboard access is unavailable";
    }
  }

  onMount(() => {
    let dataDisposable: { dispose(): void } | undefined;
    let unsubscribe = () => {};
    let resizeObserver: ResizeObserver | undefined;
    let themeObserver: MutationObserver | undefined;
    void (async () => {
      const [{ Terminal: XTerminal }, { FitAddon }] = await Promise.all([import("@xterm/xterm"), import("@xterm/addon-fit")]);
      if (destroyed) return;
      fitAddon = new FitAddon();
      terminal = new XTerminal({
        allowTransparency: false,
        cursorBlink: true,
        cursorStyle: "bar",
        cursorWidth: 2,
        fontFamily: "SFMono-Regular, ui-monospace, Menlo, Monaco, Consolas, monospace",
        fontSize: 11,
        fontWeight: "400",
        letterSpacing: 0,
        lineHeight: 1.2,
        scrollback: 6000,
        theme: xtermTheme()
      });
      terminal.loadAddon(fitAddon);
      terminal.open(host);
      dataDisposable = terminal.onData((data) => {
        if (!session || exited) return;
        const sessionId = session.id;
        writeQueue = writeQueue
          .then(() => api.writeProjectTerminal(sessionId, data))
          .catch((cause) => { error = cause instanceof Error ? cause.message : String(cause); });
      });
      unsubscribe = api.subscribeTerminal(
        (event) => { if (event.sessionId === session?.id) terminal?.write(event.data); },
        (event) => {
          if (event.sessionId !== session?.id) return;
          exited = true;
          const detail = event.error ? ` · ${event.error}` : "";
          terminal?.writeln(`\r\n\u001b[38;2;141;140;145m[process exited ${event.exitCode}${detail}]\u001b[0m`);
        }
      );
      resizeObserver = new ResizeObserver(() => fit());
      resizeObserver.observe(host);
      themeObserver = new MutationObserver(() => {
        if (terminal) terminal.options.theme = xtermTheme();
      });
      themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ["data-resolved-theme"] });
      await start();
    })().catch((cause) => {
      error = cause instanceof Error ? cause.message : String(cause);
      starting = false;
    });
    return () => {
      destroyed = true;
      cleanupResize();
      const activeSession = session;
      session = null;
      if (activeSession) void api.stopProjectTerminal(activeSession.id).catch(() => {});
      resizeObserver?.disconnect();
      themeObserver?.disconnect();
      unsubscribe();
      dataDisposable?.dispose();
      terminal?.dispose();
    };
  });
</script>

<section class="terminal-panel" aria-label="Integrated terminal">
  <button type="button" class="terminal-resize-handle" aria-label="Resize terminal" title="Drag or use arrow keys to resize terminal" on:pointerdown={beginResize} on:keydown={resizeWithKeyboard}><span></span></button>
  <header class="terminal-panel-head">
    <div class="terminal-panel-title">
      <span class:offline={exited || !!error} class="terminal-live-dot"></span>
      <TerminalSquare size={14} />
      <strong>Terminal</strong>
      <span>{session?.shell || "shell"}</span>
      <code title={session?.cwd || project.path}>{project.name}</code>
    </div>
    <div class="terminal-panel-actions">
      {#if error}<span class="terminal-inline-error" title={error}>{error}</span>{/if}
      <button title={copied ? "Copied" : "Copy selection or terminal"} aria-label="Copy terminal" disabled={!terminal} on:click={copyTerminal}><Copy size={13} /></button>
      <button title="Clear terminal" aria-label="Clear terminal" disabled={!terminal} on:click={clearTerminal}><Trash2 size={13} /></button>
      <button title="Restart shell" aria-label="Restart shell" disabled={starting} on:click={restart}>{#if starting}<LoaderCircle class="spin" size={13} />{:else}<RotateCw size={13} />{/if}</button>
      <button title="Open in external Terminal" aria-label="Open external terminal" on:click={() => dispatch("external")}><ExternalLink size={13} /></button>
      <span class="terminal-action-separator"></span>
      <button title="Close terminal" aria-label="Close terminal" on:click={() => dispatch("close")}><X size={14} /></button>
    </div>
  </header>
  <div class="terminal-host" bind:this={host}></div>
</section>
