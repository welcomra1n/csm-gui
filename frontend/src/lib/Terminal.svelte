<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Terminal } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";
  import { WebLinksAddon } from "@xterm/addon-web-links";
  import "@xterm/xterm/css/xterm.css";
  import { EventsOn, EventsOff, OnFileDrop, OnFileDropOff } from "../../wailsjs/runtime/runtime.js";
  import { WritePty, ResizePty, SaveClipboardImage } from "../../wailsjs/go/main/App.js";
  import { fontSize, activeTabId, tabs, leftWidth, rightWidth } from "./store";

  export let tabId: string;

  $: if ($activeTabId === tabId && term && fit) {
    requestAnimationFrame(() => {
      doResize();
      setTimeout(doResize, 50);
    });
  }

  // Re-fit on splitter drag (when this tab is active)
  $: if (term && fit && $activeTabId === tabId) {
    void $leftWidth; void $rightWidth;
    requestAnimationFrame(doResize);
  }

  let containerEl: HTMLDivElement;
  let term: Terminal;
  let fit: FitAddon;
  let resizeObserver: ResizeObserver | null = null;
  let outputUnsubscribe: (() => void) | null = null;
  let exitUnsubscribe: (() => void) | null = null;

  let resizeTimer: ReturnType<typeof setTimeout> | null = null;
  let lastCols = 0;
  let lastRows = 0;

  function doResize() {
    if (resizeTimer) clearTimeout(resizeTimer);
    resizeTimer = setTimeout(performResize, 40);
  }

  function performResize() {
    if (!fit || !term || !containerEl) return;
    try {
      const rect = containerEl.getBoundingClientRect();
      if (rect.width < 80 || rect.height < 40) return;
      fit.fit();
      const cols = Math.max(term.cols, 20);
      const rows = Math.max(term.rows, 5);
      if (cols === lastCols && rows === lastRows) return; // no actual change
      lastCols = cols;
      lastRows = rows;
      ResizePty(tabId, cols, rows).catch((e) =>
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

    // Ensure Tab + arrow keys always go to PTY, never to browser focus traversal.
    // Also explicitly handle Ctrl+V / Cmd+V paste — xterm.js by default treats
    // Ctrl+V as literal ^V on non-mac, so paste needs to be intercepted.
    term.attachCustomKeyEventHandler((ev: KeyboardEvent) => {
      if (ev.type !== "keydown") return true;
      if (ev.key === "Tab") {
        ev.preventDefault();
        return true;
      }
      const mod = ev.ctrlKey || ev.metaKey;
      if (mod && !ev.shiftKey && !ev.altKey && (ev.key === "v" || ev.key === "V")) {
        ev.preventDefault();
        ev.stopPropagation();
        navigator.clipboard.readText().then((text) => {
          if (text) WritePty(tabId, text).catch((err) => console.warn("write pty:", err));
        }).catch((err) => console.warn("clipboard read:", err));
        return false;
      }
      if (mod && !ev.shiftKey && !ev.altKey && (ev.key === "c" || ev.key === "C")) {
        const sel = term.getSelection();
        if (sel) {
          ev.preventDefault();
          ev.stopPropagation();
          navigator.clipboard.writeText(sel).catch((err) => console.warn("clipboard write:", err));
          return false;
        }
      }
      return true;
    });

    term.open(containerEl);
    // Wait for layout + fonts then resize repeatedly
    const fontsReady = (document as any).fonts?.ready || Promise.resolve();
    fontsReady.then(() => {
      requestAnimationFrame(() => {
        doResize();
        setTimeout(doResize, 50);
        setTimeout(doResize, 200);
        setTimeout(doResize, 500);
        setTimeout(doResize, 1000);
      });
    });
    term.focus();

    // IME composition handling.
    // Strategy: while composing, swallow onData. On compositionend, write the
    // composed text and suppress onData for a short window so xterm's
    // own keydown→data path doesn't re-emit the same chars.
    let composing = false;
    let composeBuffer = "";
    let suppressUntil = 0;

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
        suppressUntil = Date.now() + 80;
        const out = e.data || composeBuffer;
        composeBuffer = "";
        if (out) {
          WritePty(tabId, out).catch((err) => console.warn("write pty:", err));
        }
        // Clear helper textarea so xterm doesn't echo composed text
        try {
          helper.value = "";
        } catch {}
      });
    }

    term.onData((data) => {
      if (composing) return;
      if (Date.now() < suppressUntil) return;
      WritePty(tabId, data).catch((e) => console.warn("write pty:", e));
    });

    containerEl.addEventListener("click", () => term.focus());

    // File drop: when user drops files anywhere in csm-gui window,
    // write the absolute paths into the currently active tab's PTY.
    OnFileDrop((_x, _y, paths) => {
      if ($activeTabId !== tabId) return;
      if (!paths || !paths.length) return;
      const quoted = paths
        .map((p) => (p.includes(" ") ? `"${p}"` : p))
        .join(" ");
      WritePty(tabId, quoted + " ").catch(() => {});
    }, true);

    // Clipboard paste: if image present, save to temp then paste path
    containerEl.addEventListener("paste", async (e: ClipboardEvent) => {
      if ($activeTabId !== tabId) return;
      const items = e.clipboardData?.items;
      if (!items) return;
      for (const item of items) {
        if (item.type.startsWith("image/")) {
          e.preventDefault();
          const blob = item.getAsFile();
          if (!blob) continue;
          const reader = new FileReader();
          reader.onload = async () => {
            try {
              const path: string = await SaveClipboardImage(reader.result as string);
              const quoted = path.includes(" ") ? `"${path}"` : path;
              WritePty(tabId, quoted + " ").catch(() => {});
            } catch (err) {
              console.warn("paste image:", err);
            }
          };
          reader.readAsDataURL(blob);
          return;
        }
      }
    });

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
    window.addEventListener("resize", doResize);
  });

  onDestroy(() => {
    if (outputUnsubscribe) outputUnsubscribe();
    if (exitUnsubscribe) exitUnsubscribe();
    if (resizeObserver) resizeObserver.disconnect();
    window.removeEventListener("resize", doResize);
    try { OnFileDropOff(); } catch {}
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
