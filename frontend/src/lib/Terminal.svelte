<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Terminal } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";
  import { WebLinksAddon } from "@xterm/addon-web-links";
  import "@xterm/xterm/css/xterm.css";
  import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime.js";
  import { WritePty, ResizePty } from "../../wailsjs/go/main/App.js";

  export let tabId: string;

  let containerEl: HTMLDivElement;
  let term: Terminal;
  let fit: FitAddon;
  let resizeObserver: ResizeObserver | null = null;
  let outputUnsubscribe: (() => void) | null = null;
  let exitUnsubscribe: (() => void) | null = null;

  function doResize() {
    if (!fit || !term) return;
    try {
      fit.fit();
      ResizePty(tabId, term.cols, term.rows).catch((e) =>
        console.warn("resize pty:", e),
      );
    } catch (e) {
      console.warn("fit:", e);
    }
  }

  onMount(() => {
    term = new Terminal({
      fontFamily:
        'Menlo, Monaco, "Courier New", monospace, "Apple Color Emoji", "Segoe UI Emoji"',
      fontSize: 13,
      lineHeight: 1.2,
      theme: {
        background: "#1b1b1f",
        foreground: "#e6e6e6",
        cursor: "#4a9eff",
        selectionBackground: "#3a4858",
      },
      cursorBlink: true,
      scrollback: 10000,
      allowProposedApi: true,
    });

    fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon());

    term.open(containerEl);
    doResize();
    term.focus();

    term.onData((data) => {
      WritePty(tabId, data).catch((e) => console.warn("write pty:", e));
    });

    containerEl.addEventListener("click", () => term.focus());

    const outputEvent = `pty:output:${tabId}`;
    EventsOn(outputEvent, (data: string) => {
      term.write(data);
    });
    outputUnsubscribe = () => EventsOff(outputEvent);

    const exitEvent = `pty:exit:${tabId}`;
    EventsOn(exitEvent, () => {
      term.write("\r\n\x1b[33m[세션 종료]\x1b[0m\r\n");
    });
    exitUnsubscribe = () => EventsOff(exitEvent);

    resizeObserver = new ResizeObserver(() => doResize());
    resizeObserver.observe(containerEl);
  });

  onDestroy(() => {
    if (outputUnsubscribe) outputUnsubscribe();
    if (exitUnsubscribe) exitUnsubscribe();
    if (resizeObserver) resizeObserver.disconnect();
    if (term) term.dispose();
  });
</script>

<div class="term-container" bind:this={containerEl}></div>

<style>
  .term-container {
    width: 100%;
    height: 100%;
    padding: 8px;
    background: #1b1b1f;
    overflow: hidden;
  }

  :global(.xterm) {
    height: 100%;
  }

  :global(.xterm-viewport) {
    overflow-y: auto !important;
  }
</style>
