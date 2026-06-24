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
    activeTabId.update((cur) => {
      if (cur === id) {
        const list = (window as any).__tabs__ as any[];
        return list && list.length ? list[0].id : null;
      }
      return cur;
    });
  }

  $: tabList = $tabs;
  $: (window as any).__tabs__ = tabList;
</script>

<div class="tabsidebar">
  <div class="header">열린 세션 ({tabList.length})</div>
  {#if tabList.length === 0}
    <div class="empty">세션을 열어주세요</div>
  {:else}
    {#each tabList as tab (tab.id)}
      <button
        class="tab"
        class:active={$activeTabId === tab.id}
        on:click={() => selectTab(tab.id)}
      >
        <span class="icon"><ProviderIcon provider={tab.provider || "claude"} /></span>
        <span class="title" title={tab.title}>{tab.title}</span>
        <span class="close" on:click={(e) => closeTab(e, tab.id)}>×</span>
      </button>
    {/each}
  {/if}
</div>

<style>
  .tabsidebar {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow-y: auto;
  }

  .header {
    padding: 8px 12px;
    font-size: 11px;
    color: #888;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    border-bottom: 1px solid #2a2a2e;
  }

  .empty {
    padding: 16px 12px;
    color: #555;
    font-size: 12px;
    text-align: center;
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px 12px;
    text-align: left;
    border-left: 2px solid transparent;
    color: #aaa;
  }

  .tab:hover {
    background: #232328;
    color: #e6e6e6;
  }

  .tab.active {
    background: #2a2a2e;
    color: #e6e6e6;
    border-left-color: #4a9eff;
  }

  .icon {
    flex: 0 0 auto;
  }

  .title {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 12px;
  }

  .close {
    flex: 0 0 auto;
    opacity: 0;
    color: #888;
    padding: 0 4px;
    border-radius: 3px;
  }

  .tab:hover .close {
    opacity: 1;
  }

  .close:hover {
    background: #444;
    color: #fff;
  }
</style>
