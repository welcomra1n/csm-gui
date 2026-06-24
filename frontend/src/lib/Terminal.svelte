<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Terminal } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";
  import { WebLinksAddon } from "@xterm/addon-web-links";
  import "@xterm/xterm/css/xterm.css";
  import { EventsOn, EventsOff } from "../../wailsjs/runtime/runtime.js";
  import { WritePty, ResizePty } from "../../wailsjs/go/main/App.js";
  import { fontSize, activeTabId, tabs } from "./store";

  export let tabId: string;

  $: if ($activeTabId === tabId && term && fit) {
    requestAnimationFrame(() => {
      doResize();
      setTimeout(doResize, 50);
    });
  }

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
    let currentFontSize = 13;
    fontSize.subscribe((v) => {
      currentFontSize = v;
      if (term) {
        term.options.fontSize = v;
        doResize();
      }
    });

    term = new Terminal({
      fontFamily:
        '"D2Coding", Menlo, Monaco, "Courier New", monospace, "Apple Color Emoji", "Segoe UI Emoji"',
      fontSize: currentFontSize,
      lineHeight: 1.2,
      theme: {
        background: "#000000",
        foreground: "#00ff66",
        cursor: "#00ff66",
        cursorAccent: "#000000",
        selectionBackground: "#00663a",
        black: "#000000",
        red: "#ff4d8b",
        green: "#00ff66",
        yellow: "#ffd60a",
        blue: "#4a9eff",
        magenta: "#d976ff",
        cyan: "#10e0d0",
        white: "#cccccc",
        brightBlack: "#444444",
        brightRed: "#ff7eb6",
        brightGreen: "#88ff88",
        brightYellow: "#ffe66d",
        brightBlue: "#7ec0ff",
        brightMagenta: "#e9a6ff",
        brightCyan: "#7dffff",
        brightWhite: "#ffffff",
      },
      cursorBlink: true,
      scrollback: 10000,
      allowProposedApi: true,
    });

    fit = new FitAddon();
    term.loadAddon(fit);
    term.loadAddon(new WebLinksAddon());

    term.open(containerEl);
    // Wait for layout then resize a few times to ensure PTY matches
    requestAnimationFrame(() => {
      doResize();
      setTimeout(doResize, 50);
      setTimeout(doResize, 200);
      setTimeout(doResize, 500);
    });
    term.focus();

    // IME composition handling: while composing, swallow onData,
    // then emit the composed string once on compositionend.
    let composing = false;
    let composeBuffer = "";

    const helper = containerEl.querySelector(".xterm-helper-textarea") as HTMLTextAreaElement | null;
    if (helper) {
      helper.addEventListener("compositionstart", () => {
        composing = true;
        composeBuffer = "";
      });
      helper.addEventListener("compositionupdate", (e: CompositionEvent) => {
        composeBuffer = e.data || "";
      });
      helper.addEventListener("compositionend", (e: CompositionEvent) => {
        composing = false;
        const out = e.data || composeBuffer;
        composeBuffer = "";
        if (out) {
          WritePty(tabId, out).catch((err) => console.warn("write pty:", err));
        }
      });
    }

    term.onData((data) => {
      if (composing) return;
      WritePty(tabId, data).catch((e) => console.warn("write pty:", e));
    });

    containerEl.addEventListener("click", () => term.focus());

    let trustAnswered = false;
    const outputEvent = `pty:output:${tabId}`;
    EventsOn(outputEvent, (data: string) => {
      term.write(data);
      const now = Date.now();
      tabs.update((arr) =>
        arr.map((t) => {
          if (t.id !== tabId) return t;
          // If previously idle (>2.5s of silence), this is a new working burst
          const wasIdle = !t.lastActive || now - t.lastActive > 2500;
          return {
            ...t,
            lastActive: now,
            state: "working",
            stateChangedAt: wasIdle ? now : (t.stateChangedAt || now),
          };
        }),
      );
      if (!trustAnswered && /trust.*folder|yes,?\s*proceed|do you trust/i.test(data)) {
        trustAnswered = true;
        setTimeout(() => {
          WritePty(tabId, "1\r").catch(() => {});
        }, 150);
      }
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
    padding: 6px;
    background: #000;
    overflow: hidden;
  }

  :global(.xterm) {
    height: 100%;
  }

  :global(.xterm-viewport) {
    overflow-y: auto !important;
  }
</style>
