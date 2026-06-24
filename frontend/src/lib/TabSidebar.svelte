<script lang="ts">
  import { tabs, activeTabId } from "./store";
  import { KillPty } from "../../wailsjs/go/main/App.js";
  import ProviderIcon from "./ProviderIcon.svelte";

  function selectTab(id: string) {
    activeTabId.set(id);
  }

  async function closeTab(e: MouseEvent, id: string) {
    e.stopPropagation();
    try {
      await KillPty(id);
    } catch (err) {
      console.error("kill pty:", err);
    }
    tabs.update((arr) => arr.filter((t) => t.id !== id));
  }

  async function closeAll() {
    const list = $tabs.slice();
    for (const t of list) {
      try {
        await KillPty(t.id);
      } catch (err) {
        console.error("kill pty:", err);
      }
    }
    tabs.set([]);
  }
</script>

<div class="sidebar">
  <div class="header">
    <span>SESSIONS · {$tabs.length}</span>
    {#if $tabs.length > 0}
      <button class="close-all" on:click={closeAll} title="close all tabs">✕ all</button>
    {/if}
  </div>
  {#if $tabs.length === 0}
    <div class="empty">no open sessions</div>
  {:else}
    {#each $tabs as tab, i (tab.id)}
      <button
        class="tab"
        class:active={$activeTabId === tab.id}
        class:codex={tab.provider === "codex"}
        on:click={() => selectTab(tab.id)}
      >
        <span class="num">{String(i + 1).padStart(2, "0")}</span>
        <span class="icon"><ProviderIcon provider={tab.provider || "claude"} size={12} /></span>
        <span class="title" title={tab.title}>{tab.title}</span>
        <span class="close" on:click={(e) => closeTab(e, tab.id)}>×</span>
      </button>
    {/each}
  {/if}
</div>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow-y: auto;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px;
    font-size: var(--ui-fs-xs);
    color: var(--fg-mute);
    letter-spacing: 1px;
    border-bottom: 1px solid var(--border);
  }

  .close-all {
    color: var(--fg-mute);
    padding: 1px 5px;
    border: 1px solid var(--border);
    border-radius: 2px;
    font-size: calc(var(--ui-fs-xs) - 1px);
  }

  .close-all:hover {
    color: var(--accent-action);
    border-color: var(--accent-action);
  }

  .empty {
    padding: 14px 10px;
    color: var(--fg-mute);
    font-size: var(--ui-fs-sm);
    text-align: center;
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 5px 10px;
    text-align: left;
    border-left: 2px solid transparent;
    color: var(--fg-dim);
    font-size: var(--ui-fs);
  }

  .tab:hover {
    background: var(--bg-hover);
    color: var(--fg);
  }

  .tab.active {
    background: var(--bg-hover);
    color: var(--fg);
    border-left-color: var(--fg);
    box-shadow: inset 0 0 8px rgba(0, 255, 102, 0.05);
  }

  .num {
    color: var(--fg-mute);
    font-size: var(--ui-fs-xs);
    width: 16px;
  }

  .icon {
    flex: 0 0 auto;
    color: var(--accent-claude);
    display: flex;
  }

  .tab.codex .icon {
    color: var(--accent-codex);
  }

  .title {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .close {
    flex: 0 0 auto;
    opacity: 0;
    color: var(--fg-mute);
    padding: 0 4px;
    border-radius: 2px;
  }

  .tab:hover .close {
    opacity: 1;
  }

  .close:hover {
    background: var(--accent-action);
    color: var(--bg);
  }
</style>
