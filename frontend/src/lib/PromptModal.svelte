<script lang="ts">
  import { onMount, tick } from "svelte";

  export let title: string;
  export let value: string = "";
  export let placeholder: string = "";
  export let confirmLabel: string = "OK";
  export let danger: boolean = false;
  export let onConfirm: (v: string) => void;
  export let onCancel: () => void;

  let inputEl: HTMLInputElement;

  onMount(async () => {
    await tick();
    if (inputEl) {
      inputEl.focus();
      inputEl.select();
    }
  });

  function handleKey(e: KeyboardEvent) {
    if (e.key === "Enter") {
      e.preventDefault();
      onConfirm(value);
    } else if (e.key === "Escape") {
      e.preventDefault();
      onCancel();
    }
  }
</script>

<div class="overlay" on:click={onCancel}>
  <div class="modal" on:click|stopPropagation>
    <div class="title">{title}</div>
    <input
      bind:this={inputEl}
      bind:value
      type="text"
      {placeholder}
      autocomplete="off"
      on:keydown={handleKey}
    />
    <div class="actions">
      <button class="cancel" on:click={onCancel}>cancel <span class="k">esc</span></button>
      <button class="confirm" class:danger on:click={() => onConfirm(value)}>
        {confirmLabel} <span class="k">↵</span>
      </button>
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
    z-index: 999;
  }

  .modal {
    background: var(--bg-elev);
    border: 1px solid var(--fg-mute);
    border-radius: 3px;
    padding: 14px;
    min-width: 320px;
    max-width: 480px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6);
  }

  .title {
    color: var(--fg);
    font-size: 12px;
    margin-bottom: 8px;
    letter-spacing: 1px;
  }

  input {
    width: 100%;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 2px;
    padding: 6px 8px;
    color: var(--fg);
    font-size: 12px;
    font-family: inherit;
  }

  input:focus {
    border-color: var(--fg);
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 6px;
    margin-top: 10px;
  }

  .cancel, .confirm {
    padding: 5px 10px;
    border: 1px solid var(--border);
    border-radius: 2px;
    font-size: 11px;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .cancel {
    color: var(--fg-dim);
  }

  .cancel:hover {
    border-color: var(--fg-mute);
    color: var(--fg);
  }

  .confirm {
    color: var(--fg);
    border-color: var(--fg-mute);
  }

  .confirm:hover {
    border-color: var(--fg);
    box-shadow: 0 0 6px var(--fg-mute);
  }

  .confirm.danger {
    color: var(--accent-action);
    border-color: var(--accent-action);
  }

  .confirm.danger:hover {
    background: var(--accent-action);
    color: var(--bg);
    box-shadow: 0 0 6px var(--accent-action);
  }

  .k {
    font-size: 9px;
    color: var(--fg-mute);
  }
</style>
