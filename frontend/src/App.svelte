<script lang="ts">
  import { onMount } from "svelte";
  import TabSidebar from "./lib/TabSidebar.svelte";
  import SessionBrowser from "./lib/SessionBrowser.svelte";
  import Terminal from "./lib/Terminal.svelte";
  import Preview from "./lib/Preview.svelte";
  import Settings from "./lib/Settings.svelte";
  import PermissionsModal from "./lib/PermissionsModal.svelte";
  import CommandPalette from "./lib/CommandPalette.svelte";
  import type { PaletteCommand } from "./lib/CommandPalette.svelte";
  import { AppVersion, WhatsNew, AcknowledgeVersion } from "../wailsjs/go/main/App.js";

  let settingsOpen = false;
  let permsOpen = false;
  let paletteOpen = false;
  let updateToast: string | null = null;
  let whatsNew: { version: string; body: string; url: string } | null = null;

  async function dismissWhatsNew() {
    whatsNew = null;
    try { await AcknowledgeVersion(); } catch {}
  }

  function buildPaletteCommands(): PaletteCommand[] {
    const list: PaletteCommand[] = [
      { id: "settings", label: "설정 열기", hint: "Cmd+,", action: () => (settingsOpen = true) },
      { id: "perms", label: "권한 모달 열기", action: () => (permsOpen = true) },
      { id: "toggle-left", label: "좌측 사이드바 토글", action: () => leftOpen.update((v) => !v) },
      { id: "toggle-right", label: "우측 사이드바 토글", action: () => rightOpen.update((v) => !v) },
      { id: "toggle-preview", label: "프리뷰 토글", action: () => previewOpen.update((v) => !v) },
      { id: "zoom-in", label: "글자 키우기", hint: "Cmd++", action: () => fontSize.update((v) => Math.min(v + 1, 32)) },
      { id: "zoom-out", label: "글자 줄이기", hint: "Cmd+-", action: () => fontSize.update((v) => Math.max(v - 1, 8)) },
      { id: "zoom-reset", label: "글자 크기 리셋", hint: "Cmd+0", action: () => fontSize.set(13) },
      { id: "close-tab", label: "현재 탭 닫기", hint: "Cmd+W", action: () => {
        const id = $activeTabId; if (id) tabs.update((arr) => arr.filter((t) => t.id !== id));
      }},
      { id: "focus-search", label: "세션 검색창 포커스", hint: "Cmd+F", action: () => focusSearch.update((n) => n + 1) },
    ];
    // Add jump-to-tab commands
    const sorted = [...$tabs].sort((a, b) => (b.pinned ? 1 : 0) - (a.pinned ? 1 : 0));
    sorted.forEach((t, i) => {
      list.push({
        id: `tab-${t.id}`,
        label: `탭으로 이동: ${t.title}`,
        hint: i < 9 ? `Alt+${i + 1}` : undefined,
        action: () => activeTabId.set(t.id),
      });
    });
    return list;
  }
  import { tabs, activeTabId, statusText, fontSize, focusSearch, leftWidth, rightWidth, progressActive, previewOpen, rightOpen, leftOpen, altHeld } from "./lib/store";

  function handleKey(e: KeyboardEvent) {
    const mod = e.metaKey || e.ctrlKey;
    if (mod && (e.key === "w" || e.key === "W")) {
      const id = $activeTabId;
      if (id) {
        e.preventDefault();
        tabs.update((arr) => arr.filter((t) => t.id !== id));
      }
    } else if (mod && (e.key === "+" || e.key === "=")) {
      e.preventDefault();
      fontSize.update((v) => Math.min(v + 1, 32));
    } else if (mod && e.key === "-") {
      e.preventDefault();
      fontSize.update((v) => Math.max(v - 1, 8));
    } else if (mod && e.key === "0") {
      e.preventDefault();
      fontSize.set(13);
    } else if (mod && (e.key === "f" || e.key === "F")) {
      e.preventDefault();
      focusSearch.update((n) => n + 1);
    } else if (mod && e.key === ",") {
      e.preventDefault();
      settingsOpen = true;
    } else if (mod && e.shiftKey && (e.key === "p" || e.key === "P")) {
      e.preventDefault();
      paletteOpen = true;
    } else if (e.altKey && !e.metaKey && !e.ctrlKey && /^[1-9]$/.test(e.key)) {
      // Alt+1..9 → jump to that tab in the visible sidebar order
      // (pinned first, then mount order — matches TabSidebar sort).
      const idx = parseInt(e.key, 10) - 1;
      const sorted = [...$tabs].sort((a, b) => (b.pinned ? 1 : 0) - (a.pinned ? 1 : 0));
      if (sorted[idx]) {
        e.preventDefault();
        activeTabId.set(sorted[idx].id);
      }
    }
  }

  function trackAlt(e: KeyboardEvent) {
    if (e.type === "keydown" && e.altKey) altHeld.set(true);
    else if (e.type === "keyup" && e.key === "Alt") altHeld.set(false);
  }
  function blurClearAlt() { altHeld.set(false); }

  onMount(async () => {
    window.addEventListener("keydown", handleKey);
    window.addEventListener("keydown", trackAlt);
    window.addEventListener("keyup", trackAlt);
    window.addEventListener("blur", blurClearAlt);
    // Check if just updated → fetch release notes for this build
    try {
      const info = await WhatsNew();
      if (info && info.body) {
        whatsNew = info;
      } else if (info && info.version) {
        // Body unavailable (offline / API failure) — fall back to toast.
        updateToast = `v${info.version} 업데이트 완료`;
        setTimeout(() => { updateToast = null; }, 5000);
        try { await AcknowledgeVersion(); } catch {}
      }
      // First-launch permissions intro
      if (!localStorage.getItem("csm-perms-seen")) {
        permsOpen = true;
      }
    } catch {}
    return () => window.removeEventListener("keydown", handleKey);
  });

  $: activeTab = $tabs.find((t) => t.id === $activeTabId);
  $: if (typeof document !== "undefined") {
    document.documentElement.style.setProperty("--ui-fs", `${$fontSize}px`);
    document.documentElement.style.setProperty("--ui-fs-sm", `${Math.max($fontSize - 2, 6)}px`);
    document.documentElement.style.setProperty("--ui-fs-xs", `${Math.max($fontSize - 3, 6)}px`);
  }

  let dragSide: "left" | "right" | null = null;

  function startDrag(side: "left" | "right") {
    dragSide = side;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
  }

  function onMouseMove(e: MouseEvent) {
    if (!dragSide) return;
    if (dragSide === "left") {
      const w = Math.max(120, Math.min(500, e.clientX));
      leftWidth.set(w);
    } else {
      const w = Math.max(160, Math.min(600, window.innerWidth - e.clientX));
      rightWidth.set(w);
    }
  }

  function onMouseUp() {
    if (dragSide) {
      dragSide = null;
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    }
  }

  function maybeStartDrag(side: "left" | "right", e: MouseEvent) {
    const t = e.target as HTMLElement | null;
    if (t && t.closest(".split-arrow")) return;
    startDrag(side);
  }

  onMount(() => {
    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);
    return () => {
      window.removeEventListener("mousemove", onMouseMove);
      window.removeEventListener("mouseup", onMouseUp);
    };
  });
</script>

<div class="app" style="grid-template-columns: {$leftOpen ? $leftWidth + 'px' : '0px'} 3px 1fr 3px {$rightOpen ? $rightWidth + 'px' : '0px'};">
  <aside class="left" class:collapsed={!$leftOpen}>
    {#if $leftOpen}<TabSidebar />{/if}
  </aside>

  <div
    class="splitter left-split"
    class:collapsed={!$leftOpen}
    on:mousedown={(e) => maybeStartDrag("left", e)}
  >
    <button
      class="split-arrow"
      on:mousedown|stopPropagation
      on:click={() => leftOpen.update((v) => !v)}
      title={$leftOpen ? "hide tab sidebar" : "show tab sidebar"}
    >{$leftOpen ? "◀" : "▶"}</button>
  </div>

  <main class="center">
    {#each $tabs as tab (tab.id)}
      <div class="term-wrap" class:visible={tab.id === $activeTabId}>
        <Terminal tabId={tab.id} />
      </div>
    {/each}
    {#if !activeTab}
      <div class="placeholder">
        <pre class="ascii">  ____ ____  __  __
 / ___/ ___||  \/  |
| |   \___ \| |\/| |
| |___ ___) | |  | |
 \____|____/|_|  |_|</pre>
        <div class="hint">우측 패널에서 세션 선택</div>
        <div class="hint dim">Ctrl+W 닫기 · Ctrl+/- 줌 · Ctrl+0 리셋</div>
      </div>
    {/if}
  </main>

  <div
    class="splitter right-split"
    class:collapsed={!$rightOpen}
    on:mousedown={(e) => maybeStartDrag("right", e)}
  >
    <button
      class="split-arrow"
      on:mousedown|stopPropagation
      on:click={() => rightOpen.update((v) => !v)}
      title={$rightOpen ? "hide session sidebar" : "show session sidebar"}
    >{$rightOpen ? "▶" : "◀"}</button>
  </div>

  <aside class="right" class:collapsed={!$rightOpen}>
    {#if $rightOpen}
      <div class="right-top" class:full={!$previewOpen}>
        <SessionBrowser />
      </div>
      {#if $previewOpen}
        <div class="right-bottom">
          <div class="preview-header">
            <span>PREVIEW</span>
            <button class="preview-close" on:click={() => previewOpen.set(false)} title="hide preview (collapse)">▼ hide</button>
          </div>
          <div class="preview-body">
            <Preview />
          </div>
        </div>
      {:else}
        <button class="preview-show-bar" on:click={() => previewOpen.set(true)} title="show preview">
          ▲ show preview
        </button>
      {/if}
    {/if}
  </aside>

  {#if $progressActive > 0}
    <div class="progress-bar"></div>
  {/if}

  <div class="statusbar">
    <span class="dot"></span>
    <span>{$statusText || `${$tabs.length} tabs`}</span>
    <span class="spacer"></span>
    <span class="zoom">{$fontSize}px</span>
    <button class="gear" on:click={() => leftOpen.update((v) => !v)} title={$leftOpen ? "hide tabs" : "show tabs"}>{$leftOpen ? "◀▦" : "▦▶"}</button>
    <button class="gear" on:click={() => rightOpen.update((v) => !v)} title={$rightOpen ? "hide sessions" : "show sessions"}>{$rightOpen ? "▦▶" : "◀▦"}</button>
    <button class="gear" on:click={() => previewOpen.update((v) => !v)} title={$previewOpen ? "hide preview" : "show preview"}>{$previewOpen ? "▤" : "▣"}</button>
    <button class="gear" on:click={() => (settingsOpen = true)} title="settings">⚙</button>
  </div>
</div>

{#if settingsOpen}
  <Settings onClose={() => (settingsOpen = false)} />
{/if}

{#if paletteOpen}
  <CommandPalette commands={buildPaletteCommands()} onClose={() => (paletteOpen = false)} />
{/if}

{#if permsOpen}
  <PermissionsModal onClose={() => {
    permsOpen = false;
    localStorage.setItem("csm-perms-seen", "1");
  }} />
{/if}

{#if updateToast}
  <div class="toast" on:click={() => (updateToast = null)}>
    <span class="toast-icon">✓</span>
    <span>{updateToast}</span>
  </div>
{/if}

{#if whatsNew}
  <div class="whatsnew-backdrop" on:click|self={dismissWhatsNew}>
    <div class="whatsnew" role="dialog" aria-modal="true">
      <header>
        <div class="title">
          <span class="badge">UPDATED</span>
          <span>v{whatsNew.version} · 변경 사항</span>
        </div>
        <button class="close" on:click={dismissWhatsNew} title="닫기">×</button>
      </header>
      <div class="body">
        <pre>{whatsNew.body}</pre>
      </div>
      <footer>
        <a class="link" href={whatsNew.url} target="_blank" rel="noopener">GitHub 릴리스 ↗</a>
        <button class="ok" on:click={dismissWhatsNew}>확인</button>
      </footer>
    </div>
  </div>
{/if}

<style>
  .app {
    display: grid;
    grid-template-rows: 1fr 20px;
    grid-template-areas:
      "left lsplit center rsplit right"
      "status status status status status";
    height: 100vh;
    width: 100vw;
    background: var(--bg);
  }

  .splitter {
    background: var(--border-strong);
    cursor: col-resize;
    position: relative;
  }

  .splitter:hover, .splitter:active {
    background: var(--fg);
    box-shadow: 0 0 4px var(--fg-mute);
  }

  .splitter.collapsed {
    cursor: pointer;
  }

  .split-arrow {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    width: 14px;
    height: 28px;
    padding: 0;
    background: var(--bg-elev);
    border: 1px solid var(--border-strong);
    color: var(--fg-mute);
    cursor: pointer;
    font-size: 9px;
    line-height: 1;
    border-radius: 2px;
    z-index: 5;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .split-arrow:hover {
    background: var(--bg);
    color: var(--fg);
    border-color: var(--fg);
  }

  .left-split { grid-area: lsplit; }
  .right-split { grid-area: rsplit; }

  .left {
    grid-area: left;
    background: var(--bg-elev);
    overflow: hidden;
    position: relative;
  }

  .left.collapsed, .right.collapsed {
    background: var(--bg-elev);
  }

  .expand-strip {
    width: 100%;
    height: 100%;
    background: var(--bg-elev);
    color: var(--fg-mute);
    border: none;
    cursor: pointer;
    font-size: var(--ui-fs);
    writing-mode: vertical-rl;
    letter-spacing: 4px;
    padding: 8px 0;
  }

  .expand-strip:hover {
    color: var(--fg);
    background: var(--bg-hover);
  }

  .collapse-btn {
    background: none;
    border: 1px solid var(--border-strong);
    color: var(--fg-mute);
    cursor: pointer;
    padding: 2px 6px;
    font-size: var(--ui-fs-xs);
    border-radius: 3px;
  }

  .collapse-btn:hover {
    color: var(--fg);
    border-color: var(--fg);
  }

  .collapse-left {
    position: absolute;
    top: 4px;
    right: 4px;
  }

  .right-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 8px;
    background: var(--bg-elev);
    border-bottom: 1px solid var(--border-strong);
    font-size: var(--ui-fs-xs);
    color: var(--fg-mute);
    letter-spacing: 0.5px;
    flex-shrink: 0;
  }

  .center {
    grid-area: center;
    position: relative;
    background: var(--bg);
    overflow: hidden;
  }

  .right {
    grid-area: right;
    display: flex;
    flex-direction: column;
    background: var(--bg-elev);
    overflow: hidden;
  }

  .right-top {
    flex: 2;
    overflow: hidden;
    border-bottom: 1px solid var(--border-strong);
    display: flex;
    flex-direction: column;
    min-height: 0;
  }

  .right-top.full {
    flex: 1;
    border-bottom: none;
  }

  .right-bottom {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-height: 0;
  }

  .preview-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 8px;
    font-size: var(--ui-fs-xs);
    color: var(--fg-mute);
    background: var(--bg-elev);
    border-bottom: 1px solid var(--border-strong);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .preview-close {
    color: var(--fg-mute);
    font-size: var(--ui-fs-xs);
    padding: 2px 8px;
    background: none;
    border: 1px solid var(--border-strong);
    border-radius: 3px;
    cursor: pointer;
    line-height: 1;
  }

  .preview-close:hover {
    color: var(--fg);
    border-color: var(--fg);
  }

  .preview-show-bar {
    background: var(--bg-elev);
    color: var(--fg-mute);
    border: none;
    border-top: 1px solid var(--border-strong);
    padding: 6px 8px;
    font-size: var(--ui-fs-xs);
    cursor: pointer;
    text-align: center;
    letter-spacing: 0.5px;
  }

  .preview-show-bar:hover {
    color: var(--fg);
    background: var(--bg-hover);
  }

  .preview-body {
    flex: 1;
    overflow: hidden;
    min-height: 0;
  }

  .term-wrap {
    position: absolute;
    inset: 0;
    display: none;
  }

  .term-wrap.visible {
    display: block;
  }

  .placeholder {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
  }

  .ascii {
    color: var(--fg);
    text-shadow: 0 0 8px var(--fg-mute);
    font-size: var(--ui-fs);
    line-height: 1.2;
  }

  .hint {
    color: var(--fg-dim);
    font-size: var(--ui-fs);
  }

  .hint.dim {
    color: var(--fg-mute);
    font-size: var(--ui-fs-sm);
  }

  .statusbar {
    grid-area: status;
    display: flex;
    align-items: center;
    gap: 8px;
    background: var(--bg-elev);
    border-top: 1px solid var(--border-strong);
    padding: 0 10px;
    line-height: 20px;
    font-size: var(--ui-fs-sm);
    color: var(--fg-dim);
  }

  .statusbar .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--fg);
    box-shadow: 0 0 4px var(--fg);
  }

  .spacer {
    flex: 1;
  }

  .zoom {
    color: var(--fg-mute);
  }

  .gear {
    color: var(--fg-mute);
    font-size: var(--ui-fs);
    padding: 0 6px;
  }

  .gear:hover {
    color: var(--fg);
  }

  .progress-bar {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: 3px;
    background: var(--bg);
    z-index: 100;
    overflow: hidden;
  }

  .progress-bar::after {
    content: "";
    position: absolute;
    inset: 0;
    background: linear-gradient(
      90deg,
      transparent 0%,
      var(--fg) 40%,
      var(--fg) 60%,
      transparent 100%
    );
    animation: indeterminate 1.1s ease-in-out infinite;
    box-shadow: 0 0 8px var(--fg);
  }

  @keyframes indeterminate {
    0%   { transform: translateX(-100%); }
    100% { transform: translateX(100%); }
  }

  .whatsnew-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    backdrop-filter: blur(2px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .whatsnew {
    background: #0a0a0a;
    border: 1px solid var(--accent-claude, #00ff66);
    border-radius: 6px;
    box-shadow: 0 0 24px rgba(0, 255, 102, 0.18);
    width: min(640px, 92vw);
    max-height: 80vh;
    display: flex;
    flex-direction: column;
    color: var(--fg, #cccccc);
    font-family: "D2Coding", Menlo, monospace;
  }
  .whatsnew header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border, #222);
  }
  .whatsnew .title {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    color: var(--fg);
  }
  .whatsnew .badge {
    background: var(--accent-claude, #00ff66);
    color: #000;
    font-size: 10px;
    font-weight: 700;
    padding: 2px 6px;
    border-radius: 2px;
    letter-spacing: 1px;
  }
  .whatsnew .close {
    background: none;
    border: 0;
    color: var(--fg-mute);
    font-size: 20px;
    line-height: 1;
    cursor: pointer;
    padding: 0 4px;
  }
  .whatsnew .close:hover { color: var(--accent-action, #ff4d8b); }
  .whatsnew .body {
    overflow-y: auto;
    padding: 12px 14px;
    flex: 1;
  }
  .whatsnew .body pre {
    margin: 0;
    white-space: pre-wrap;
    word-wrap: break-word;
    font-family: inherit;
    font-size: 12.5px;
    line-height: 1.55;
    color: var(--fg);
  }
  .whatsnew footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    border-top: 1px solid var(--border, #222);
  }
  .whatsnew .link {
    color: var(--fg-mute);
    font-size: 11px;
    text-decoration: none;
  }
  .whatsnew .link:hover { color: var(--accent-claude); }
  .whatsnew .ok {
    background: var(--accent-claude, #00ff66);
    color: #000;
    border: 0;
    padding: 6px 16px;
    border-radius: 3px;
    font-weight: 600;
    cursor: pointer;
  }
  .whatsnew .ok:hover { box-shadow: 0 0 8px rgba(0, 255, 102, 0.5); }

  .toast {
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 14px 24px;
    background: var(--bg-elev);
    border: 1px solid var(--fg);
    border-radius: 4px;
    color: var(--fg);
    font-size: var(--ui-fs);
    box-shadow: 0 0 16px var(--fg-mute), 0 8px 32px rgba(0, 0, 0, 0.8);
    cursor: pointer;
    z-index: 9999;
    animation: toast-in 0.25s ease-out, toast-out 0.5s ease-in 4.5s forwards;
  }

  .toast:hover {
    border-color: var(--accent-pinned);
    box-shadow: 0 0 12px var(--accent-pinned), 0 6px 24px rgba(0, 0, 0, 0.7);
  }

  .toast-icon {
    color: var(--fg);
    font-size: calc(var(--ui-fs) + 2px);
  }

  .toast-hint {
    color: var(--fg-mute);
    font-size: var(--ui-fs-xs);
  }

  @keyframes toast-in {
    from { opacity: 0; transform: translate(-50%, calc(-50% + 8px)); }
    to { opacity: 1; transform: translate(-50%, -50%); }
  }
  @keyframes toast-out {
    from { opacity: 1; transform: translate(-50%, -50%); }
    to { opacity: 0; transform: translate(-50%, calc(-50% - 8px)); }
  }
</style>
