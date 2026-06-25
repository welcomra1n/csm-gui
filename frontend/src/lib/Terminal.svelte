<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Terminal } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";
  import { WebLinksAddon } from "@xterm/addon-web-links";
  import "@xterm/xterm/css/xterm.css";
  import { EventsOn, EventsOff, OnFileDrop, WindowShow } from "../../wailsjs/runtime/runtime.js";
  import { WritePty, ResizePty, SaveClipboardImage, CopyImageToClipboard } from "../../wailsjs/go/main/App.js";
  import { fontSize, activeTabId, tabs, leftWidth, rightWidth } from "./store";

  export let tabId: string;

  $: if ($activeTabId === tabId && term && fit) {
    requestAnimationFrame(() => {
      doResize();
      setTimeout(doResize, 50);
    });
    // Re-register Wails OnFileDrop so this active tab owns the callback.
    if (wailsDropCallback) {
      try { OnFileDrop(wailsDropCallback, true); } catch {}
    }
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
  let wailsDropCallback: ((x: number, y: number, paths: string[]) => void) | null = null;

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
    // Korean/Japanese composition fires compositionend AND a delayed
    // textarea input echo that xterm forwards via onData. We dedupe by
    // remembering the last composed string (for 350ms) and silently
    // dropping any onData burst that exactly matches it. We do NOT use a
    // time-based blanket suppress, because the keystroke that committed
    // the composition (typically space) arrives within that same window
    // and must reach the PTY — otherwise the user has to press space
    // twice to get a single space after Korean input.
    let composing = false;
    let composeBuffer = "";
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
        try { helper.value = ""; } catch {}
        lastComposed = out;
        if (out) {
          WritePty(tabId, out).catch((err) => console.warn("write pty:", err));
        }
        setTimeout(() => { if (lastComposed === out) lastComposed = ""; }, 350);
      });
    }

    term.onData((data) => {
      if (composing) return;
      // Late-arriving echo of the composed string — drop ONLY if it
      // matches exactly. Other bytes (the commit space, the next char)
      // pass through unchanged.
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
    const IMG_EXT_RE = /\.(png|jpg|jpeg|gif|webp|bmp|tiff|tif|heic|heif)$/i;
    function isImagePath(p: string) {
      return IMG_EXT_RE.test(p);
    }
    function writePaths(arr: string[]) {
      if (arr.length === 0) return;
      const quoted = arr
        .map((p) => (p.includes(" ") ? `"${p}"` : p))
        .join(" ");
      WritePty(tabId, quoted + " ").catch(() => {});
    }
    // Image drop path: copy file into OS clipboard then send Ctrl+V so
    // claude / codex use their built-in paste detection and render the
    // attachment as [Image #N] instead of a bare file path.
    async function deliverDropped(arr: string[]) {
      if (arr.length === 0) return;
      const images = arr.filter(isImagePath);
      const others = arr.filter((p) => !isImagePath(p));
      if (others.length > 0) writePaths(others);
      for (const imgPath of images) {
        try {
          await CopyImageToClipboard(imgPath);
          await WritePty(tabId, "\x16");
          // small spacing between multiple image pastes
          if (images.length > 1) await new Promise((r) => setTimeout(r, 150));
        } catch (err) {
          console.warn("clipboard paste failed, falling back to path:", err);
          writePaths([imgPath]);
        }
      }
    }

    // Wails OnFileDrop holds a single callback — last registration wins.
    // Register here and ALSO re-register whenever this tab becomes active
    // (see reactive block below). Active-tab guard inside the callback
    // is belt-and-suspenders.
    wailsDropCallback = async (_x: number, _y: number, paths: string[]) => {
      if ($activeTabId !== tabId) return;
      if (!paths || !paths.length) return;
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
      deliverDropped(processed);
    };
    OnFileDrop(wailsDropCallback, true);

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
      deliverDropped(collected);
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

    // Skip trust auto-answer entirely for resumed sessions — the trust
    // dialog only appears on the first claude run in a folder, so a tab
    // with an existing sessionId never needs the auto answer, and replayed
    // history can contain the prompt text verbatim which would falsely
    // trigger it.
    const initialTab = $tabs.find((t) => t.id === tabId);
    const skipTrust = !!(initialTab && initialTab.sessionId);
    let trustAnswered = skipTrust;
    const trustWindowStart = Date.now();
    let trustBuffer = "";
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
      // Auto-answer claude / codex trust prompt.
      // Resume replay of session history can include arbitrary text
      // (including past assistant messages that quoted the prompt verbatim),
      // so the option-line alone is not enough. We require BOTH the
      // prompt header AND the option line to appear in a small rolling
      // buffer of the most recent 4KB of output. Bounded to the first 5s
      // of the session.
      if (!trustAnswered && Date.now() - trustWindowStart < 5000) {
        trustBuffer = (trustBuffer + data).slice(-4096);
        const hasHeader = /Quick safety check|Do you trust the files in this folder\?/i.test(trustBuffer);
        const hasOption = /1\.\s*Yes,?\s*(I trust this folder|proceed)/i.test(trustBuffer);
        if (hasHeader && hasOption) {
          trustAnswered = true;
          trustBuffer = "";
          setTimeout(() => {
            WritePty(tabId, "1\r").catch(() => {});
          }, 150);
        }
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
    // Do NOT call OnFileDropOff — it clears the global handler, which
    // would deregister other live Terminal instances' callbacks.
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
    --wails-drop-target: drop;
  }

  :global(.xterm) {
    height: 100%;
  }

  :global(.xterm-viewport) {
    overflow-y: auto !important;
  }
</style>
