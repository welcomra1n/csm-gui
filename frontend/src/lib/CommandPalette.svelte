<script lang="ts">
  import { onMount } from "svelte";

  export interface PaletteCommand {
    id: string;
    label: string;
    hint?: string;
    action: () => void;
  }

  export let commands: PaletteCommand[];
  export let onClose: () => void;

  let query = "";
  let inputEl: HTMLInputElement;
  let highlight = 0;

  $: filtered = (() => {
    const q = query.toLowerCase().trim();
    if (!q) return commands.slice(0, 30);
    return commands
      .filter((c) => c.label.toLowerCase().includes(q) || (c.hint || "").toLowerCase().includes(q))
      .slice(0, 30);
  })();

  $: if (highlight >= filtered.length) highlight = Math.max(0, filtered.length - 1);

  onMount(() => {
    setTimeout(() => inputEl?.focus(), 0);
  });

  function run(c: PaletteCommand) {
    onClose();
    try { c.action(); } catch (e) { console.warn("palette:", e); }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Escape") { e.preventDefault(); onClose(); }
    else if (e.key === "Enter") {
      e.preventDefault();
      const c = filtered[highlight];
      if (c) run(c);
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      highlight = Math.min(highlight + 1, filtered.length - 1);
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      highlight = Math.max(highlight - 1, 0);
    }
  }
</script>

<div class="overlay" on:click={onClose}>
  <div class="palette" on:click|stopPropagation>
    <input
      bind:this={inputEl}
      bind:value={query}
      on:keydown={onKey}
      placeholder="명령 찾기…"
      autocomplete="off"
    />
    <div class="list">
      {#each filtered as c, i (c.id)}
        <button
          class="row"
          class:active={i === highlight}
          on:mouseenter={() => (highlight = i)}
          on:click={() => run(c)}
        >
          <span class="label">{c.label}</span>
          {#if c.hint}<span class="hint">{c.hint}</span>{/if}
        </button>
      {/each}
      {#if filtered.length === 0}
        <div class="empty">no commands</div>
      {/if}
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.55);
    display: flex;
    justify-content: center;
    align-items: flex-start;
    padding-top: 12vh;
    z-index: 2000;
  }
  .palette {
    background: var(--bg-elev);
    border: 1px solid var(--fg-mute);
    border-radius: 4px;
    width: min(560px, 90vw);
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.8), 0 0 12px var(--fg-mute);
    overflow: hidden;
  }
  .palette input {
    width: 100%;
    box-sizing: border-box;
    background: var(--bg);
    color: var(--fg);
    border: none;
    border-bottom: 1px solid var(--border-strong);
    padding: 10px 12px;
    font-size: var(--ui-fs);
  }
  .palette input:focus { outline: none; }
  .list {
    max-height: 50vh;
    overflow-y: auto;
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    background: none;
    border: none;
    color: var(--fg-dim);
    padding: 6px 12px;
    text-align: left;
    cursor: pointer;
    font-size: var(--ui-fs-sm);
  }
  .row:hover, .row.active {
    background: rgba(0, 255, 102, 0.08);
    color: var(--fg);
  }
  .label { flex: 1; }
  .hint {
    color: var(--fg-mute);
    font-size: var(--ui-fs-xs);
    margin-left: 16px;
  }
  .empty {
    padding: 14px;
    color: var(--fg-mute);
    text-align: center;
    font-size: var(--ui-fs-sm);
  }
</style>
