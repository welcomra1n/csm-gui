<script lang="ts">
  import { onMount } from "svelte";
  import TabSidebar from "./lib/TabSidebar.svelte";
  import SessionBrowser from "./lib/SessionBrowser.svelte";
  import Terminal from "./lib/Terminal.svelte";
  import Preview from "./lib/Preview.svelte";
  import { tabs, activeTabId, statusText } from "./lib/store";

  function handleKey(e: KeyboardEvent) {
    if (e.ctrlKey && (e.key === "w" || e.key === "W")) {
      const id = $activeTabId;
      if (id) {
        e.preventDefault();
        tabs.update((arr) => arr.filter((t) => t.id !== id));
      }
    }
  }

  onMount(() => {
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  });

  $: activeTab = $tabs.find((t) => t.id === $activeTabId);
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
        <div>오른쪽에서 세션을 선택하세요</div>
        <div class="hint">Ctrl+W: 탭 닫기</div>
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

  <div class="statusbar">{$statusText || `${$tabs.length}개 탭 열림`}</div>
</div>

<style>
  .app {
    display: grid;
    grid-template-columns: 220px 1fr 280px;
    grid-template-rows: 1fr 22px;
    grid-template-areas:
      "left center right"
      "status status status";
    height: 100vh;
    width: 100vw;
  }

  .left {
    grid-area: left;
    background: #1f1f23;
    border-right: 1px solid #2a2a2e;
    overflow: hidden;
  }

  .center {
    grid-area: center;
    position: relative;
    background: #1b1b1f;
    overflow: hidden;
  }

  .right {
    grid-area: right;
    display: flex;
    flex-direction: column;
    background: #1f1f23;
    border-left: 1px solid #2a2a2e;
    overflow: hidden;
  }

  .right-top {
    flex: 2;
    overflow: hidden;
    border-bottom: 1px solid #2a2a2e;
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
    color: #555;
    gap: 12px;
  }

  .hint {
    font-size: 11px;
    color: #444;
  }

  .statusbar {
    grid-area: status;
    background: #18181b;
    border-top: 1px solid #2a2a2e;
    padding: 0 12px;
    line-height: 22px;
    font-size: 11px;
    color: #888;
  }
</style>
