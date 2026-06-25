<script lang="ts">
  import { onMount } from "svelte";
  import TabSidebar from "./lib/TabSidebar.svelte";
  import SessionBrowser from "./lib/SessionBrowser.svelte";
  import Terminal from "./lib/Terminal.svelte";
  import Preview from "./lib/Preview.svelte";
  import Settings from "./lib/Settings.svelte";
  import PermissionsModal from "./lib/PermissionsModal.svelte";
  import { AppVersion } from "../wailsjs/go/main/App.js";

  let settingsOpen = false;
  let permsOpen = false;
  let updateToast: string | null = null;
  import { tabs, activeTabId, statusText, fontSize, focusSearch, leftWidth, rightWidth, progressActive, previewOpen, rightOpen, leftOpen } from "./lib/store";

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
    }
  }

  onMount(async () => {
    window.addEventListener("keydown", handleKey);
    // Check if just updated
    try {
      const current = await AppVersion();
      const pending = localStorage.getItem("csm-pending-version");
      const lastSeen = localStorage.getItem("csm-last-version");
      if (pending && current === pending && lastSeen !== current) {
        updateToast = `v${current} 업데이트 완료`;
        localStorage.removeItem("csm-pending-version");
      }
      localStorage.setItem("csm-last-version", current);
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

<div class="app" style="grid-template-columns: {$leftOpen ? $leftWidth + 'px' : '0px'} 6px 1fr 6px {$rightOpen ? $rightWidth + 'px' : '0px'};">
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
    <span class="toast-hint">(클릭해서 닫기)</span>
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

  .toast {
    position: fixed;
    bottom: 32px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 16px;
    background: var(--bg-elev);
    border: 1px solid var(--fg);
    border-radius: 4px;
    color: var(--fg);
    font-size: var(--ui-fs);
    box-shadow: 0 0 12px var(--fg-mute), 0 6px 24px rgba(0, 0, 0, 0.7);
    cursor: pointer;
    z-index: 9999;
    animation: toast-in 0.25s ease-out;
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
    from { opacity: 0; transform: translate(-50%, 8px); }
    to { opacity: 1; transform: translate(-50%, 0); }
  }
</style>
