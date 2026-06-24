<script lang="ts">
  import { onMount } from "svelte";
  import TabSidebar from "./lib/TabSidebar.svelte";
  import SessionBrowser from "./lib/SessionBrowser.svelte";
  import Terminal from "./lib/Terminal.svelte";
  import { tabs, activeTabId, sidebarMode, statusText } from "./lib/store";

  function toggleMode() {
    sidebarMode.update((m) => (m === "tabs" ? "browser" : "tabs"));
  }

  function handleKey(e: KeyboardEvent) {
    if (e.key === "F1") {
      e.preventDefault();
      toggleMode();
    } else if (e.key === "F2") {
      e.preventDefault();
      sidebarMode.set("tabs");
    } else if (e.ctrlKey && (e.key === "w" || e.key === "W")) {
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
  <aside class="sidebar">
    <div class="mode-toggle">
      <button
        class:active={$sidebarMode === "tabs"}
        on:click={() => sidebarMode.set("tabs")}
        title="F2"
      >
        탭 ({$tabs.length})
      </button>
      <button
        class:active={$sidebarMode === "browser"}
        on:click={() => sidebarMode.set("browser")}
        title="F1"
      >
        세션 찾기
      </button>
    </div>
    <div class="sidebar-content">
      {#if $sidebarMode === "tabs"}
        <TabSidebar />
      {:else}
        <SessionBrowser />
      {/if}
    </div>
  </aside>

  <main class="main">
    {#each $tabs as tab (tab.id)}
      <div class="term-wrap" class:visible={tab.id === $activeTabId}>
        <Terminal tabId={tab.id} />
      </div>
    {/each}
    {#if !activeTab}
      <div class="placeholder">
        <div>세션 찾기에서 세션을 선택하세요</div>
        <div class="hint">F1: 모드 전환 · F2: 탭 보기 · Ctrl+W: 탭 닫기</div>
      </div>
    {/if}
  </main>

  <div class="statusbar">{$statusText || `${$tabs.length}개 탭 열림`}</div>
</div>

<style>
  .app {
    display: grid;
    grid-template-columns: 260px 1fr;
    grid-template-rows: 1fr 22px;
    grid-template-areas:
      "sidebar main"
      "status status";
    height: 100vh;
    width: 100vw;
  }

  .sidebar {
    grid-area: sidebar;
    display: flex;
    flex-direction: column;
    background: #1f1f23;
    border-right: 1px solid #2a2a2e;
    overflow: hidden;
  }

  .mode-toggle {
    display: flex;
    background: #18181b;
    border-bottom: 1px solid #2a2a2e;
  }

  .mode-toggle button {
    flex: 1;
    padding: 8px;
    color: #888;
    font-size: 12px;
    border-bottom: 2px solid transparent;
  }

  .mode-toggle button:hover {
    color: #e6e6e6;
  }

  .mode-toggle button.active {
    color: #e6e6e6;
    border-bottom-color: #4a9eff;
  }

  .sidebar-content {
    flex: 1;
    overflow: hidden;
  }

  .main {
    grid-area: main;
    position: relative;
    background: #1b1b1f;
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
