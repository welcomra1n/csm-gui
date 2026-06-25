<script lang="ts">
  import { onMount } from "svelte";
  import { ListPermissions, SetPermission, OpenURL } from "../../wailsjs/go/main/App.js";

  interface Permission {
    key: string;
    label: string;
    description: string;
    category: string;
    required: boolean;
    systemUrl: string;
    enabled: boolean;
  }

  export let dense: boolean = false; // settings inline view

  let perms: Permission[] = [];

  async function load() {
    perms = (await ListPermissions()) || [];
  }

  async function toggle(p: Permission) {
    if (p.required) return;
    const next = !p.enabled;
    await SetPermission(p.key, next);
    perms = perms.map((x) => (x.key === p.key ? { ...x, enabled: next } : x));
  }

  function open(url: string) {
    OpenURL(url).catch(() => {});
  }

  onMount(load);
</script>

<div class="perms" class:dense>
  {#each perms as p (p.key)}
    <div class="perm" class:os={p.category === "os"} class:req={p.required}>
      <label class="row">
        <input
          type="checkbox"
          checked={p.enabled}
          disabled={p.required}
          on:change={() => toggle(p)}
        />
        <div class="info">
          <div class="label">
            {p.label}
            {#if p.required}<span class="badge req-badge">필수</span>{/if}
            {#if p.category === "os"}<span class="badge os-badge">OS</span>{/if}
          </div>
          <div class="desc">{p.description}</div>
        </div>
        {#if p.systemUrl}
          <button class="ext" on:click|stopPropagation|preventDefault={() => open(p.systemUrl)} title="open system settings">
            ↗
          </button>
        {/if}
      </label>
    </div>
  {/each}
</div>

<style>
  .perms {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .perm {
    border: 1px solid var(--border);
    border-radius: 3px;
    background: var(--bg);
  }

  .row {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 8px 10px;
    cursor: pointer;
  }

  .dense .row {
    padding: 6px 8px;
  }

  input[type="checkbox"] {
    margin-top: 2px;
    accent-color: var(--fg);
    cursor: pointer;
  }

  input[type="checkbox"]:disabled {
    cursor: not-allowed;
    opacity: 0.6;
  }

  .info {
    flex: 1;
  }

  .label {
    color: var(--fg);
    font-size: var(--ui-fs-sm);
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .desc {
    color: var(--fg-mute);
    font-size: var(--ui-fs-xs);
    margin-top: 2px;
    line-height: 1.5;
    white-space: pre-wrap;
  }

  .badge {
    font-size: calc(var(--ui-fs-xs) - 1px);
    padding: 1px 4px;
    border-radius: 2px;
    letter-spacing: 0.5px;
  }

  .req-badge {
    background: rgba(255, 77, 139, 0.12);
    color: var(--accent-action);
  }

  .os-badge {
    background: rgba(74, 158, 255, 0.12);
    color: var(--accent-folder);
  }

  .ext {
    color: var(--fg-mute);
    padding: 0 6px;
  }

  .ext:hover {
    color: var(--fg);
  }
</style>
