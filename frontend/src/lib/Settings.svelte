<script lang="ts">
  import { onMount } from "svelte";
  import { AppVersion, CheckUpdate, ApplyUpdate, RestartApp } from "../../wailsjs/go/main/App.js";
  import PermissionsPanel from "./PermissionsPanel.svelte";

  export let onClose: () => void;

  let current = "";
  let latest = "";
  let hasUpdate = false;
  let releaseUrl = "";
  let body = "";
  let checking = false;
  let applying = false;
  let updated = false;
  let log = "";

  async function load() {
    current = await AppVersion();
    await check();
  }

  async function check() {
    checking = true;
    log = "";
    try {
      const info: any = await CheckUpdate();
      latest = info.latest;
      hasUpdate = info.hasUpdate;
      releaseUrl = info.url;
      body = info.body || "";
    } catch (e: any) {
      log = `check failed: ${e?.message || e}`;
    } finally {
      checking = false;
    }
  }

  async function apply() {
    applying = true;
    log = "running package manager…";
    try {
      const out: string = await ApplyUpdate();
      log = (out || "done") + "\n\n자동 재시작 중…";
      updated = true;
      localStorage.setItem("csm-pending-version", latest);
      // Windows path schedules its own updater + quit; calling RestartApp
      // again would race a second relaunch. Skip if backend already handled it.
      if (out && out.startsWith("updater scheduled")) {
        return;
      }
      setTimeout(() => RestartApp().catch(() => {}), 800);
    } catch (e: any) {
      log = `apply failed: ${e?.message || e}\n\nFallback: manually run\n  brew upgrade --cask csm-gui  (macOS)\n  scoop update csm-gui  (Windows)`;
    } finally {
      applying = false;
    }
  }

  async function restart() {
    try {
      await RestartApp();
    } catch (e: any) {
      log += `\nrestart failed: ${e?.message || e}`;
    }
  }

  onMount(load);

  function handleKey(e: KeyboardEvent) {
    if (e.key === "Escape") onClose();
  }
</script>

<svelte:window on:keydown={handleKey} />

<div class="overlay" on:click={onClose}>
  <div class="modal" on:click|stopPropagation>
    <div class="header">
      <span class="title">SETTINGS</span>
      <button class="close" on:click={onClose}>✕</button>
    </div>

    <div class="section">
      <div class="row">
        <span class="key">version</span>
        <span class="val">{current || "…"}</span>
      </div>
      <div class="row">
        <span class="key">latest</span>
        <span class="val">{latest || (checking ? "checking…" : "—")}</span>
      </div>
      <div class="actions">
        <button class="btn" on:click={check} disabled={checking}>
          {checking ? "checking…" : "check for updates"}
        </button>
        {#if hasUpdate && !updated}
          <button class="btn primary" on:click={apply} disabled={applying}>
            {applying ? "updating…" : `update to ${latest}`}
          </button>
        {/if}
        {#if updated}
          <button class="btn primary" on:click={restart}>
            restart now ↻
          </button>
        {/if}
      </div>
      {#if hasUpdate}
        <div class="release">
          <div class="release-label">release notes ({latest})</div>
          <div class="release-body">{body}</div>
          <a class="release-link" href={releaseUrl} target="_blank" rel="noopener">view on github →</a>
        </div>
      {/if}
      {#if log}
        <pre class="log">{log}</pre>
      {/if}
    </div>

    <div class="section">
      <div class="section-title">PERMISSIONS</div>
      <PermissionsPanel dense />
    </div>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 999;
  }

  .modal {
    background: var(--bg-elev);
    border: 1px solid var(--fg-mute);
    border-radius: 3px;
    min-width: 420px;
    max-width: 560px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.7);
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
  }

  .title {
    color: var(--fg);
    letter-spacing: 1px;
    font-size: var(--ui-fs);
  }

  .close {
    color: var(--fg-mute);
    padding: 2px 8px;
    border: 1px solid var(--border);
    border-radius: 2px;
  }

  .close:hover {
    color: var(--accent-action);
    border-color: var(--accent-action);
  }

  .section {
    padding: 14px;
    font-size: var(--ui-fs-sm);
  }

  .row {
    display: flex;
    justify-content: space-between;
    padding: 4px 0;
  }

  .key { color: var(--fg-mute); }
  .val { color: var(--fg); }

  .actions {
    display: flex;
    gap: 6px;
    margin-top: 10px;
  }

  .btn {
    flex: 1;
    padding: 6px 10px;
    border: 1px solid var(--border);
    border-radius: 2px;
    color: var(--fg-dim);
    font-size: var(--ui-fs-sm);
  }

  .btn:hover:not(:disabled) {
    border-color: var(--fg-mute);
    color: var(--fg);
  }

  .btn.primary {
    color: var(--fg);
    border-color: var(--fg-mute);
  }

  .btn.primary:hover:not(:disabled) {
    border-color: var(--fg);
    box-shadow: 0 0 6px var(--fg-mute);
  }

  .btn:disabled {
    opacity: 0.5;
  }

  .release {
    margin-top: 12px;
    padding: 8px 10px;
    border: 1px solid var(--border);
    border-radius: 2px;
    background: var(--bg);
  }

  .release-label {
    color: var(--accent-action);
    font-size: var(--ui-fs-xs);
    letter-spacing: 1px;
    margin-bottom: 4px;
  }

  .release-body {
    color: var(--fg-dim);
    white-space: pre-wrap;
    max-height: 160px;
    overflow-y: auto;
    font-size: var(--ui-fs-xs);
  }

  .release-link {
    display: block;
    color: var(--accent-folder);
    font-size: var(--ui-fs-xs);
    margin-top: 6px;
  }

  .log {
    margin-top: 10px;
    padding: 8px;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 2px;
    color: var(--fg-dim);
    font-size: var(--ui-fs-xs);
    max-height: 180px;
    overflow: auto;
    white-space: pre-wrap;
  }
</style>
