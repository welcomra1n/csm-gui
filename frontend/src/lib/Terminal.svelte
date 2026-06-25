<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Terminal } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";
  import { WebLinksAddon } from "@xterm/addon-web-links";
  import "@xterm/xterm/css/xterm.css";
  import { EventsOn, EventsOff, OnFileDrop, OnFileDropOff, WindowShow } from "../../wailsjs/runtime/runtime.js";
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
  let dropCleanups: (() => void)[] = [];

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
        '"D2Coding", "Apple Color Emoji", "Segoe UI Emoji", "Noto Color Emoji", Menlo, Monaco, "Courier New", monospace',
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

    // Ensure Tab key always goes to PTY. On Windows only, intercept Ctrl+V
    // because xterm.js sends it as literal ^V; on macOS we let xterm/WKWebView
    // handle Cmd+V natively to avoid the WKWebView clipboard permission prompt
    // that fires on every navigator.clipboard.readText() call.
    const isWindows = navigator.platform.toLowerCase().includes("win");
    term.attachCustomKeyEventHandler((ev: KeyboardEvent) => {
      if (ev.type !== "keydown") return true;
      if (ev.key === "Tab") {
        ev.preventDefault();
        return true;
      }
      if (isWindows) {
        const mod = ev.ctrlKey;
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
    // Korean/Japanese composition fires both compositionend AND a delayed
    // textarea input that xterm forwards through onData. Without a wide
    // enough suppress window the composed chars are written twice. We also
    // remember the last composed string so any matching onData burst within
    // the window is silently dropped even if longer than the timer.
    let composing = false;
    let composeBuffer = "";
    let suppressUntil = 0;
    let lastComposed = "";

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
        // Clear helper FIRST so the post-composition input event xterm
        // listens for gets an empty value and emits nothing.
        try { helper.value = ""; } catch {}
        lastComposed = out;
        suppressUntil = Date.now() + 250;
        if (out) {
          WritePty(tabId, out).catch((err) => console.warn("write pty:", err));
        }
        setTimeout(() => { if (lastComposed === out) lastComposed = ""; }, 350);
      });
    }

    term.onData((data) => {
      if (composing) return;
      if (Date.now() < suppressUntil) return;
      // Late-arriving echo of the composed string after the timer expired.
      if (lastComposed && data === lastComposed) {
        lastComposed = "";
        return;
      }
      WritePty(tabId, data).catch((e) => console.warn("write pty:", e));
    });

    containerEl.addEventListener("click", () => term.focus());

    // File drop.
    // HTML5 drop handler fires synchronously when the user releases the
    // mouse in the WebView. Wails OnFileDrop may also fire for the same
    // drop from the native layer. To avoid writing the same file twice,
    // the HTML5 handler records every basename it touches into
    // htmlHandled, and the Wails handler waits a tick then drops any
    // path whose basename was already recorded.
    const htmlHandled = new Set<string>();
    function rememberHandled(name: string) {
      htmlHandled.add(name);
      setTimeout(() => htmlHandled.delete(name), 1500);
    }
    function basename(p: string) {
      const i = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
      return i >= 0 ? p.slice(i + 1) : p;
    }
    function writePaths(arr: string[]) {
      if (arr.length === 0) return;
      const quoted = arr
        .map((p) => (p.includes(" ") ? `"${p}"` : p))
        .join(" ");
      WritePty(tabId, quoted + " ").catch(() => {});
    }

    OnFileDrop(async (_x, _y, paths) => {
      if ($activeTabId !== tabId) return;
      if (!paths || !paths.length) return;
      // Give the HTML5 handler a tick to claim its files first.
      await new Promise((r) => setTimeout(r, 80));
      const mod = await import("../../wailsjs/go/main/App.js");
      const processed: string[] = [];
      for (const p of paths) {
        if (htmlHandled.has(basename(p))) continue;
        try {
          const out = await mod.ProcessDroppedPath(p);
          processed.push(out || p);
        } catch {
          processed.push(p);
        }
      }
      writePaths(processed);
    }, false);

    // Attach to document so the listener catches drops anywhere in the
    // window — xterm's helper textarea sometimes intercepts events before
    // they reach containerEl.
    const dragOverHandler = (e: DragEvent) => {
      e.preventDefault();
      if (e.dataTransfer) e.dataTransfer.dropEffect = "copy";
    };
    const dropHandler = async (e: DragEvent) => {
      if ($activeTabId !== tabId) return;
      const files = e.dataTransfer?.files;
      if (!files || files.length === 0) return;
      e.preventDefault();
      const collected: string[] = [];
      for (const f of Array.from(files)) {
        rememberHandled(f.name);
        const realPath = (f as any).path as string | undefined;
        if (realPath) {
          try {
            const mod = await import("../../wailsjs/go/main/App.js");
            const out = await mod.ProcessDroppedPath(realPath);
            collected.push(out || realPath);
          } catch {
            collected.push(realPath);
          }
          continue;
        }
        if (f.type.startsWith("image/")) {
          try {
            const dataUrl: string = await new Promise((resolve, reject) => {
              const r = new FileReader();
              r.onload = () => resolve(r.result as string);
              r.onerror = () => reject(r.error);
              r.readAsDataURL(f);
            });
            const path: string = await SaveClipboardImage(dataUrl);
            collected.push(path);
          } catch (err) {
            console.warn("drop image:", err);
          }
        }
      }
      writePaths(collected);
    };
    // document-level only; containerEl listener would bubble to document
    // and fire the handler twice for the same drop.
    document.addEventListener("dragover", dragOverHandler);
    document.addEventListener("drop", dropHandler);
    dropCleanups.push(
      () => document.removeEventListener("dragover", dragOverHandler),
      () => document.removeEventListener("drop", dropHandler),
    );

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
    const trustWindowStart = Date.now();
    let idleTimer: ReturnType<typeof setTimeout> | null = null;
    let everWasWorking = false;
    function scheduleIdleNotif() {
      if (idleTimer) clearTimeout(idleTimer);
      idleTimer = setTimeout(() => {
        if (!everWasWorking) return;
        tabs.update((arr) =>
          arr.map((t) => {
            if (t.id !== tabId) return t;
            if (t.state === "idle") return t;
            return { ...t, state: "idle", stateChangedAt: Date.now() };
          }),
        );
        const tab = $tabs.find((t) => t.id === tabId);
        if (!tab) return;
        const isActive = $activeTabId === tabId && document.hasFocus();
        if (isActive) return; // user already watching
        try {
          if ("Notification" in window && Notification.permission === "granted") {
            const n = new Notification("작업 완료", {
              body: tab.title || "세션 응답 끝",
              silent: false,
              tag: `csm-done-${tabId}`,
            });
            n.onclick = () => {
              try { WindowShow(); } catch {}
              activeTabId.set(tabId);
              window.focus();
              n.close();
            };
          }
        } catch (e) {
          console.warn("notify:", e);
        }
      }, 2500);
    }
    const outputEvent = `pty:output:${tabId}`;
    EventsOn(outputEvent, (data: string) => {
      term.write(data);
      const now = Date.now();
      everWasWorking = true;
      tabs.update((arr) =>
        arr.map((t) => {
          if (t.id !== tabId) return t;
          const wasIdle = !t.lastActive || now - t.lastActive > 2500;
          return {
            ...t,
            lastActive: now,
            state: "working",
            stateChangedAt: wasIdle ? now : (t.stateChangedAt || now),
          };
        }),
      );
      scheduleIdleNotif();
      // Auto-answer claude / codex trust prompt. Match the option line
      // that both old ("1. Yes, proceed") and new ("1. Yes, I trust this
      // folder") prompts share. Bounded to the first 8s of the session
      // so unrelated later output containing "1. Yes" does not retrigger.
      if (
        !trustAnswered &&
        Date.now() - trustWindowStart < 8000 &&
        /1\.\s*Yes,?\s*(I trust this folder|proceed)/i.test(data)
      ) {
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
    for (const fn of dropCleanups) try { fn(); } catch {}
    dropCleanups = [];
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
