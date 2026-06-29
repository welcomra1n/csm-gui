<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { Terminal } from "@xterm/xterm";
  import { FitAddon } from "@xterm/addon-fit";
  import { WebLinksAddon } from "@xterm/addon-web-links";
  import { SearchAddon } from "@xterm/addon-search";
  import { WebglAddon } from "@xterm/addon-webgl";
  import { ImageAddon } from "@xterm/addon-image";
  import "@xterm/xterm/css/xterm.css";
  import { EventsOn, EventsOff, OnFileDrop, WindowShow } from "../../wailsjs/runtime/runtime.js";
  import { WritePty, ResizePty, SaveClipboardImage, CopyImageToClipboard, GetSessionFileStat } from "../../wailsjs/go/main/App.js";
  import { fontSize, activeTabId, tabs, leftWidth, rightWidth, statusText } from "./store";
  import doneSoundUrl from "../assets/done.mp3";

  export let tabId: string;

  $: if ($activeTabId === tabId && term && fit) {
    requestAnimationFrame(() => {
      doResize();
      setTimeout(doResize, 50);
    });
    // Re-register Wails OnFileDrop so this active tab owns the callback.
    if (wailsDropCallback) {
      try { OnFileDrop(wailsDropCallback, false); } catch {}
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
  let searchAddon: SearchAddon | null = null;
  let searchOpen = false;
  let searchValue = "";
  let searchInputEl: HTMLInputElement;
  let ctxMenu: { x: number; y: number } | null = null;

  function findNext(reverse = false) {
    if (!searchAddon || !searchValue) return;
    const opts = { caseSensitive: false, wholeWord: false, regex: false, decorations: { matchOverviewRuler: "#ffd60a", activeMatchColorOverviewRuler: "#ffd60a" } };
    if (reverse) searchAddon.findPrevious(searchValue, opts as any);
    else searchAddon.findNext(searchValue, opts as any);
  }
  function closeSearch() {
    searchOpen = false;
    try { searchAddon?.clearDecorations(); } catch {}
    term?.focus();
  }
  async function ctxCopy() {
    const sel = term?.getSelection();
    if (sel) { try { await navigator.clipboard.writeText(sel); } catch {} }
    ctxMenu = null;
  }
  async function ctxPaste() {
    try {
      const text = await navigator.clipboard.readText();
      if (text) enqueueWriteExternal(text);
    } catch {}
    ctxMenu = null;
  }
  function ctxClear() { try { term?.clear(); } catch {} ; ctxMenu = null; }
  function ctxFind() { ctxMenu = null; searchOpen = true; setTimeout(() => searchInputEl?.focus(), 0); }
  async function ctxCopyLine() {
    if (!term || !ctxMenu) { ctxMenu = null; return; }
    // Synthesize triple-click at the menu open coordinate to make xterm
    // select the line under the right-click, then read the selection.
    const x = ctxMenu.x, y = ctxMenu.y;
    ctxMenu = null;
    const target = containerEl;
    const opts = { bubbles: true, cancelable: true, view: window, clientX: x, clientY: y, button: 0 } as MouseEventInit;
    for (let i = 1; i <= 3; i++) {
      target.dispatchEvent(new MouseEvent("mousedown", { ...opts, detail: i }));
      target.dispatchEvent(new MouseEvent("mouseup", { ...opts, detail: i }));
      target.dispatchEvent(new MouseEvent("click", { ...opts, detail: i }));
    }
    setTimeout(async () => {
      const sel = term.getSelection();
      if (sel && sel.trim()) {
        try { await navigator.clipboard.writeText(sel); statusText.set(`copied: ${sel.length} chars`); } catch {}
      }
    }, 30);
  }

  // Bridge to internal enqueueWrite (defined in onMount closure).
  let enqueueWriteExternal: (s: string) => void = () => {};
  let resizeObserver: ResizeObserver | null = null;
  let outputUnsubscribe: (() => void) | null = null;
  let exitUnsubscribe: (() => void) | null = null;
  let dropCleanups: (() => void)[] = [];
  let wailsDropCallback: ((x: number, y: number, paths: string[]) => void) | null = null;

  let resizeTimer: ReturnType<typeof setTimeout> | null = null;
  let lastCols = 0;
  let lastRows = 0;

  // External-write detection. csm tracks the JSONL mtime; if it advances
  // while no PTY output arrived in a small recent window, something
  // outside csm (Ghostty, another claude --resume, manual edit) is
  // touching the file and csm's in-memory state is now stale.
  let lastSessionMtime = 0;
  let lastPtyOutputAt = 0;
  let externallyModified = false;
  let externalDismissed = false;
  let extCheckTimer: ReturnType<typeof setInterval> | null = null;
  $: sessionIdForStat = ($tabs.find((t) => t.id === tabId) || {} as any).sessionId as string | undefined;

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
    term.loadAddon(new WebLinksAddon((event, uri) => {
      if (event.ctrlKey || event.metaKey) {
        try { window.open(uri, "_blank"); } catch {}
      }
    }, {
      hover: (_ev: MouseEvent, uri: string) => {
        statusText.set(`↗ ${uri}  (⌘+click to open)`);
      },
      leave: () => {
        // restore status text after hover ends
        statusText.set("");
      },
    } as any));
    searchAddon = new SearchAddon();
    term.loadAddon(searchAddon);

    // Inline image rendering — supports iTerm2 OSC 1337 and Sixel.
    try {
      const imageAddon = new ImageAddon({
        sixelSupport: true,
        iipSupport: true,
        iipSizeLimit: 64 * 1024 * 1024,
      });
      term.loadAddon(imageAddon);
    } catch (e) {
      console.warn("image addon:", e);
    }

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

    // WebGL renderer disabled: under WKWebView it interferes with the
    // xterm helper textarea's IME composition events, breaking Korean
    // input (only initial jamo survive). v0.8.0 did not load this addon
    // and IME worked perfectly there.

    // Auto-copy selected text to OS clipboard
    term.onSelectionChange(() => {
      const sel = term.getSelection();
      if (sel) {
        try { navigator.clipboard.writeText(sel); } catch {}
      }
    });
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

    // IME composition handling. v0.8.0 style: while composing, drop
    // onData; on compositionend send the composed syllable directly.
    // Every "improvement" piled on top of this between v0.9.0 and
    // v0.9.48 (suppression timers, dedup windows, jamo coalescers,
    // DEL preview, microtask batching) actually broke Korean input
    // that worked perfectly here. Keep it boring.
    let composing = false;
    let composeBuffer = "";

    function enqueueWrite(s: string) {
      WritePty(tabId, s).catch((e) => console.warn("write pty:", e));
    }
    enqueueWriteExternal = enqueueWrite;

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
        if (out) enqueueWrite(out);
      });
    }

    term.onData((data) => {
      if (composing) return;
      enqueueWrite(data);
    });

    containerEl.addEventListener("click", () => { if (!ctxMenu) term.focus(); });

    // Triple-click → auto copy the selected line. xterm already selects the
    // whole line on a 3rd click; we just read the selection right after and
    // push it to the OS clipboard so the user does not need a follow-up ⌘C.
    let clickCount = 0;
    let clickTimer: ReturnType<typeof setTimeout> | null = null;
    containerEl.addEventListener("mousedown", () => {
      clickCount++;
      if (clickTimer) clearTimeout(clickTimer);
      clickTimer = setTimeout(() => { clickCount = 0; }, 500);
    });
    containerEl.addEventListener("mouseup", () => {
      if (clickCount >= 3) {
        clickCount = 0;
        setTimeout(() => {
          const sel = term?.getSelection();
          if (sel && sel.trim()) {
            navigator.clipboard.writeText(sel).then(() => {
              statusText.set(`copied: ${sel.length} chars`);
            }).catch(() => {});
          }
        }, 0);
      }
    });

    // Right-click context menu (copy / paste / clear / find).
    containerEl.addEventListener("contextmenu", (e: MouseEvent) => {
      e.preventDefault();
      ctxMenu = { x: e.clientX, y: e.clientY };
    });
    // Shift+Insert = paste (Linux/X11 convention)
    document.addEventListener("keydown", (e: KeyboardEvent) => {
      if (e.shiftKey && e.key === "Insert" && $activeTabId === tabId) {
        e.preventDefault();
        navigator.clipboard.readText().then((text) => {
          if (text) enqueueWrite(text);
        }).catch(() => {});
      }
      // Cmd+F / Ctrl+F → toggle search (xterm scrollback search)
      const mod = e.metaKey || e.ctrlKey;
      if (mod && (e.key === "f" || e.key === "F") && $activeTabId === tabId) {
        e.preventDefault();
        e.stopPropagation();
        searchOpen = true;
        setTimeout(() => searchInputEl?.focus(), 0);
      }
      if (searchOpen && e.key === "Escape") {
        searchOpen = false;
        try { searchAddon?.clearDecorations(); } catch {}
        term.focus();
      }
    });

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
      console.debug("[csm] wails drop", tabId, paths);
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
    OnFileDrop(wailsDropCallback, false);

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
          const a = new Audio(doneSoundUrl);
          a.volume = 0.8;
          void a.play();
        } catch {}
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
      lastPtyOutputAt = now;
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

    // External-write detector. Snapshots the JSONL mtime on first
    // observation and on every PTY output (claude's own writes settle
    // ~immediately after output), then polls every 3 s. If the on-disk
    // mtime is newer than what we've seen AND no PTY output landed in
    // the last 2 s, someone outside csm wrote to the file.
    extCheckTimer = setInterval(async () => {
      if (!sessionIdForStat || externalDismissed) return;
      try {
        const stat = await GetSessionFileStat(sessionIdForStat);
        if (!stat || !stat.exists) return;
        const mt = stat.modTimeUnixNano;
        if (lastSessionMtime === 0) {
          lastSessionMtime = mt;
          return;
        }
        if (mt > lastSessionMtime) {
          const ourWrite = Date.now() - lastPtyOutputAt < 2000;
          if (ourWrite) {
            lastSessionMtime = mt;
          } else {
            externallyModified = true;
          }
        }
      } catch {}
    }, 3000);
  });

  onDestroy(() => {
    if (outputUnsubscribe) outputUnsubscribe();
    if (exitUnsubscribe) exitUnsubscribe();
    if (resizeObserver) resizeObserver.disconnect();
    if (extCheckTimer) clearInterval(extCheckTimer);
    window.removeEventListener("resize", doResize);
    // Do NOT call OnFileDropOff — it clears the global handler, which
    // would deregister other live Terminal instances' callbacks.
    for (const fn of dropCleanups) try { fn(); } catch {}
    dropCleanups = [];
    if (term) term.dispose();
  });
</script>

<div class="term-wrap-inner">
  {#if externallyModified}
    <div class="ext-banner">
      <span class="ext-icon">⚠</span>
      <span class="ext-msg">이 세션이 csm 밖에서 수정됨. 이 탭의 화면은 그 변경을 반영하지 못함. 동기화하려면 탭 닫고 다시 열기.</span>
      <button class="ext-dismiss" on:click={() => { externalDismissed = true; externallyModified = false; }}>닫기</button>
    </div>
  {/if}
  <div class="term-container" bind:this={containerEl}></div>
  {#if searchOpen}
    <div class="search-bar">
      <input
        bind:this={searchInputEl}
        bind:value={searchValue}
        on:keydown={(e) => {
          if (e.key === "Enter") { e.preventDefault(); findNext(e.shiftKey); }
          else if (e.key === "Escape") { e.preventDefault(); closeSearch(); }
        }}
        placeholder="find in scrollback…"
        autocomplete="off"
      />
      <button on:click={() => findNext(true)} title="previous (Shift+Enter)">▲</button>
      <button on:click={() => findNext(false)} title="next (Enter)">▼</button>
      <button on:click={closeSearch} title="close (Esc)">×</button>
    </div>
  {/if}
  {#if ctxMenu}
    <div class="ctx-overlay" on:click={() => (ctxMenu = null)} on:contextmenu|preventDefault={() => (ctxMenu = null)}>
      <div class="ctx" style="left: {ctxMenu.x}px; top: {ctxMenu.y}px;" on:click|stopPropagation>
        <button on:click={ctxCopy}>copy <span class="hint">⌘C</span></button>
        <button on:click={ctxCopyLine}>copy line <span class="hint">3-click</span></button>
        <button on:click={ctxPaste}>paste <span class="hint">⌘V</span></button>
        <button on:click={ctxFind}>find <span class="hint">⌘F</span></button>
        <button on:click={ctxClear}>clear</button>
      </div>
    </div>
  {/if}
</div>

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

  .term-wrap-inner {
    position: relative;
    width: 100%;
    height: 100%;
  }

  .ext-banner {
    position: absolute;
    top: 6px;
    left: 12px;
    right: 12px;
    z-index: 10;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 10px;
    background: rgba(255, 214, 10, 0.08);
    border: 1px solid var(--accent-pinned, #ffd60a);
    border-radius: 4px;
    color: var(--accent-pinned, #ffd60a);
    font-size: var(--ui-fs-sm);
    backdrop-filter: blur(4px);
  }
  .ext-icon { font-size: 14px; }
  .ext-msg { flex: 1; line-height: 1.4; color: var(--fg); }
  .ext-dismiss {
    background: none;
    border: 1px solid var(--accent-pinned, #ffd60a);
    color: var(--accent-pinned, #ffd60a);
    padding: 3px 10px;
    border-radius: 3px;
    font-size: var(--ui-fs-xs);
    cursor: pointer;
  }
  .ext-dismiss:hover { background: rgba(255, 214, 10, 0.15); }

  .search-bar {
    position: absolute;
    top: 6px;
    right: 12px;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 4px 6px;
    background: var(--bg-elev);
    border: 1px solid var(--fg-mute);
    border-radius: 3px;
    z-index: 50;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.5);
  }
  .search-bar input {
    background: var(--bg);
    color: var(--fg);
    border: 1px solid var(--border);
    border-radius: 2px;
    padding: 2px 6px;
    font-size: var(--ui-fs-xs);
    min-width: 200px;
  }
  .search-bar input:focus { outline: none; border-color: var(--fg-mute); }
  .search-bar button {
    background: none;
    border: 1px solid var(--border);
    color: var(--fg-mute);
    padding: 2px 6px;
    font-size: var(--ui-fs-xs);
    border-radius: 2px;
    cursor: pointer;
  }
  .search-bar button:hover { color: var(--fg); border-color: var(--fg); }

  .ctx-overlay {
    position: fixed;
    inset: 0;
    z-index: 100;
  }
  .ctx {
    position: absolute;
    background: var(--bg-elev);
    border: 1px solid var(--fg-mute);
    border-radius: 3px;
    padding: 4px 0;
    min-width: 140px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.7);
  }
  .ctx button {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    background: none;
    border: none;
    color: var(--fg);
    padding: 5px 12px;
    font-size: var(--ui-fs-sm);
    cursor: pointer;
    text-align: left;
  }
  .ctx button:hover { background: var(--bg-hover); }
  .ctx .hint { color: var(--fg-mute); font-size: var(--ui-fs-xs); margin-left: 16px; }
</style>
