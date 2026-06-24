<script lang="ts">
  import { onMount } from "svelte";
  import TabSidebar from "./lib/TabSidebar.svelte";
  import SessionBrowser from "./lib/SessionBrowser.svelte";
  import Terminal from "./lib/Terminal.svelte";
  import Preview from "./lib/Preview.svelte";
  import Settings from "./lib/Settings.svelte";
  import { AppVersion } from "../wailsjs/go/main/App.js";

  let settingsOpen = false;
  let updateToast: string | null = null;
  import { tabs, activeTabId, statusText, fontSize, focusSearch, leftWidth, rightWidth } from "./lib/store";

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

  onMount(() => {
    window.addEventListener("mousemove", onMouseMove);
    window.addEventListener("mouseup", onMouseUp);
    return () => {
      window.removeEventListener("mousemove", onMouseMove);
      window.removeEventListener("mouseup", onMouseUp);
    };
  });
</script>

<div class="app" style="grid-template-columns: {$leftWidth}px 4px 1fr 4px {$rightWidth}px;">
  <aside class="left">
    <TabSidebar />
  </aside>

  <div class="splitter left-split" on:mousedown={() => startDrag("left")}></div>

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

  <div class="splitter right-split" on:mousedown={() => startDrag("right")}></div>

  <aside class="right">
    <div class="right-top">
      <SessionBrowser />
    </div>
    <div class="right-bottom">
      <Preview />
    </div>
  </aside>

  <div class="statusbar">
    <span class="dot"></span>
    <span>{$statusText || `${$tabs.length} tabs`}</span>
    <span class="spacer"></span>
    <span class="zoom">{$fontSize}px</span>
    <button class="gear" on:click={() => (settingsOpen = true)} title="settings">⚙</button>
  </div>
</div>

{#if settingsOpen}
  <Settings onClose={() => (settingsOpen = false)} />
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
  }

  .splitter:hover, .splitter:active {
    background: var(--fg);
    box-shadow: 0 0 4px var(--fg-mute);
  }

  .left-split { grid-area: lsplit; }
  .right-split { grid-area: rsplit; }

  .left {
    grid-area: left;
    background: var(--bg-elev);
    overflow: hidden;
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
  }

  .right-bottom {
    flex: 1;
    overflow: hidden;
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
