<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { invoke } from "@tauri-apps/api/core";
  import { listen, type UnlistenFn } from "@tauri-apps/api/event";
  import { Terminal } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";
  import "@xterm/xterm/css/xterm.css";

  let containerEl: HTMLDivElement;
  let term: Terminal;
  let fit: FitAddon;
  const tabId = "main";
  let unlistenOutput: UnlistenFn | null = null;
  let unlistenExit: UnlistenFn | null = null;

  onMount(async () => {
    term = new Terminal({
      fontFamily: 'Menlo, "D2Coding", monospace',
      fontSize: 14,
      cursorBlink: true,
      scrollback: 10000,
      allowProposedApi: true,
      theme: { background: "#000000", foreground: "#00ff66", cursor: "#00ff66" },
    });
    fit = new FitAddon();
    term.loadAddon(fit);
    term.open(containerEl);
    fit.fit();

    term.onData((data) => {
      invoke("pty_write", { tabId, data }).catch((e) => console.warn("write:", e));
    });

    unlistenOutput = await listen<string>(`pty:output:${tabId}`, (e) => {
      term.write(e.payload);
    });
    unlistenExit = await listen(`pty:exit:${tabId}`, () => {
      term.write("\r\n\x1b[33m[exit]\x1b[0m\r\n");
    });

    const { rows, cols } = term;
    await invoke("pty_start", {
      tabId,
      cmd: "/bin/zsh",
      args: ["-l"],
      cwd: null,
      rows,
      cols,
    });

    const ro = new ResizeObserver(() => {
      try { fit.fit(); } catch {}
      invoke("pty_resize", { tabId, rows: term.rows, cols: term.cols }).catch(() => {});
    });
    ro.observe(containerEl);
    term.focus();
  });

  onDestroy(() => {
    unlistenOutput?.();
    unlistenExit?.();
    invoke("pty_kill", { tabId }).catch(() => {});
    term?.dispose();
  });
</script>

<div class="root">
  <h1>csm v1 — Korean IME prototype</h1>
  <p>Type Korean. If 한글 composes inline like ghostty, Tauri wins.</p>
  <div class="term" bind:this={containerEl}></div>
</div>

<style>
  :global(body) { margin: 0; background: #0a0a0a; color: #cccccc; font-family: Menlo, monospace; }
  .root { height: 100vh; display: flex; flex-direction: column; padding: 12px; box-sizing: border-box; }
  h1 { color: #00ff66; font-size: 13px; margin: 0 0 4px 0; letter-spacing: 1px; }
  p { color: #888; font-size: 11px; margin: 0 0 8px 0; }
  .term { flex: 1; border: 1px solid #222; padding: 8px; background: #000; min-height: 0; }
</style>
