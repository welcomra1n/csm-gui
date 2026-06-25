<script lang="ts">
  import { onMount } from "svelte";
  import { ListAllTags } from "../../wailsjs/go/main/App.js";

  export let title: string;
  export let initialTags: string[] = [];
  export let onConfirm: (tags: string[]) => void;
  export let onCancel: () => void;

  let selected: string[] = [...initialTags];
  let allTags: string[] = [];
  let input = "";
  let inputEl: HTMLInputElement;

  onMount(async () => {
    try {
      allTags = await ListAllTags();
    } catch {
      allTags = [];
    }
    setTimeout(() => inputEl?.focus(), 0);
  });

  function toggle(tag: string) {
    if (selected.includes(tag)) {
      selected = selected.filter((t) => t !== tag);
    } else {
      selected = [...selected, tag];
    }
  }

  function addFromInput() {
    const parts = input
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean);
    for (const p of parts) {
      if (!selected.includes(p)) selected = [...selected, p];
    }
    input = "";
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Enter") {
      e.preventDefault();
      if (input.trim()) {
        addFromInput();
      } else {
        onConfirm(selected);
      }
    } else if (e.key === "Escape") {
      e.preventDefault();
      onCancel();
    } else if (e.key === "," ) {
      e.preventDefault();
      addFromInput();
    }
  }

  $: unusedTags = allTags.filter((t) => !selected.includes(t));
</script>

<div class="overlay" on:click={onCancel}>
  <div class="modal" on:click|stopPropagation>
    <div class="title">{title}</div>

    <div class="section-label">selected</div>
    <div class="chips">
      {#each selected as t (t)}
        <button class="chip selected" on:click={() => toggle(t)} title="click to remove">
          {t} <span class="x">×</span>
        </button>
      {/each}
      {#if selected.length === 0}
        <span class="empty">(no tags)</span>
      {/if}
    </div>

    {#if unusedTags.length > 0}
      <div class="section-label">existing — click to add</div>
      <div class="chips">
        {#each unusedTags as t (t)}
          <button class="chip" on:click={() => toggle(t)}>+ {t}</button>
        {/each}
      </div>
    {/if}

    <div class="section-label">new tag</div>
    <div class="input-row">
      <input
        bind:this={inputEl}
        bind:value={input}
        on:keydown={onKey}
        placeholder="type and press Enter or ,"
        autocomplete="off"
      />
      <button class="btn" on:click={addFromInput} disabled={!input.trim()}>add</button>
    </div>

    <div class="actions">
      <button class="btn cancel" on:click={onCancel}>cancel</button>
      <button class="btn confirm" on:click={() => onConfirm(selected)}>save</button>
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }
  .modal {
    background: var(--bg-elev);
    border: 1px solid var(--fg-mute);
    border-radius: 4px;
    padding: 16px;
    min-width: 420px;
    max-width: 560px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.8);
    color: var(--fg);
  }
  .title {
    font-size: var(--ui-fs);
    color: var(--fg);
    margin-bottom: 12px;
    letter-spacing: 0.5px;
  }
  .section-label {
    font-size: var(--ui-fs-xs);
    color: var(--fg-mute);
    margin: 10px 0 4px;
    letter-spacing: 1px;
    text-transform: uppercase;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    min-height: 24px;
  }
  .chip {
    background: var(--bg);
    border: 1px solid var(--border);
    color: var(--fg-dim);
    border-radius: 3px;
    padding: 3px 8px;
    font-size: var(--ui-fs-xs);
    cursor: pointer;
  }
  .chip:hover {
    border-color: var(--fg);
    color: var(--fg);
  }
  .chip.selected {
    border-color: var(--fg-mute);
    background: rgba(0, 255, 102, 0.08);
    color: var(--fg);
  }
  .chip .x {
    color: var(--fg-mute);
    margin-left: 2px;
  }
  .empty {
    color: var(--fg-mute);
    font-size: var(--ui-fs-xs);
    font-style: italic;
  }
  .input-row {
    display: flex;
    gap: 6px;
    margin-top: 4px;
  }
  .input-row input {
    flex: 1;
    background: var(--bg);
    border: 1px solid var(--border);
    color: var(--fg);
    padding: 4px 8px;
    font-size: var(--ui-fs-xs);
    border-radius: 2px;
  }
  .input-row input:focus {
    border-color: var(--fg-mute);
    outline: none;
  }
  .btn {
    background: var(--bg);
    border: 1px solid var(--border);
    color: var(--fg-dim);
    padding: 4px 12px;
    font-size: var(--ui-fs-xs);
    border-radius: 2px;
    cursor: pointer;
  }
  .btn:hover { color: var(--fg); border-color: var(--fg); }
  .btn:disabled { opacity: 0.4; cursor: not-allowed; }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 16px;
  }
  .btn.confirm {
    color: var(--fg);
    border-color: var(--fg-mute);
  }
</style>
