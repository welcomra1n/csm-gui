<script lang="ts">
  import { onMount } from "svelte";
  import TabSidebar from "./lib/TabSidebar.svelte";
  import SessionBrowser from "./lib/SessionBrowser.svelte";
  import Terminal from "./lib/Terminal.svelte";
  import Preview from "./lib/Preview.svelte";
  import { tabs, activeTabId, statusText, fontSize } from "./lib/store";

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
    }
  }

  onMount(() => {
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  });

  $: activeTab = $tabs.find((t) => t.id === $activeTabId);
  $: if (typeof document !== "undefined") {
    document.documentElement.style.setProperty("--ui-fs", `${$fontSize}px`);
    document.documentElement.style.setProperty("--ui-fs-sm", `${Math.max($fontSize - 2, 6)}px`);
    document.documentElement.style.setProperty("--ui-fs-xs", `${Math.max($fontSize - 3, 6)}px`);
  }
</script>

<div class="app">
  <aside class="left">
    <TabSidebar />
  </aside>

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
  </div>
</div>

<style>
  .app {
    display: grid;
    grid-template-columns: 200px 1fr 260px;
    grid-template-rows: 1fr 20px;
    grid-template-areas:
      "left center right"
      "status status status";
    height: 100vh;
    width: 100vw;
    background: var(--bg);
  }

  .left {
    grid-area: left;
    background: var(--bg-elev);
    border-right: 1px solid var(--border-strong);
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
    border-left: 1px solid var(--border-strong);
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
</style>
