<script lang="ts">
  import { onMount } from "svelte";

  export let x: number;
  export let y: number;
  export let items: { label: string; action: () => void; danger?: boolean; key?: string }[];
  export let onClose: () => void;

  function handleClick(item: { action: () => void }) {
    item.action();
    onClose();
  }

  onMount(() => {
    const close = (e: MouseEvent) => onClose();
    const esc = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    setTimeout(() => {
      window.addEventListener("click", close, { once: true });
      window.addEventListener("contextmenu", close, { once: true });
    }, 0);
    window.addEventListener("keydown", esc);
    return () => window.removeEventListener("keydown", esc);
  });
</script>

<div class="menu" style="top: {y}px; left: {x}px;" on:click|stopPropagation>
  {#each items as item}
    <button class="item" class:danger={item.danger} on:click={() => handleClick(item)}>
      <span>{item.label}</span>
      {#if item.key}<span class="key">{item.key}</span>{/if}
    </button>
  {/each}
</div>

<style>
  .menu {
    position: fixed;
    min-width: 160px;
    background: var(--bg-elev);
    border: 1px solid var(--fg-mute);
    border-radius: 2px;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.6);
    z-index: 1000;
    padding: 2px 0;
  }

  .item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    width: 100%;
    padding: 5px 10px;
    text-align: left;
    color: var(--fg-dim);
    font-size: var(--ui-fs-sm);
  }

  .item:hover {
    background: var(--bg-hover);
    color: var(--fg);
  }

  .item.danger {
    color: var(--accent-action);
  }

  .item.danger:hover {
    background: var(--accent-action);
    color: var(--bg);
  }

  .key {
    color: var(--fg-mute);
    font-size: var(--ui-fs-xs);
  }
</style>
