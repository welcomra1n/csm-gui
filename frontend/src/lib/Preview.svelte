<script lang="ts">
  import { selectedSessionId, sessions } from "./store";
  import ProviderIcon from "./ProviderIcon.svelte";

  $: selected = $sessions.find((s) => s.id === $selectedSessionId);

  function fmtDate(iso: string): string {
    if (!iso) return "";
    const d = new Date(iso);
    return `${d.getMonth() + 1}/${d.getDate()} ${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
  }
</script>

<div class="preview">
  {#if selected}
    <div class="row title" class:codex={selected.provider === "codex"}>
      <ProviderIcon provider={selected.provider} size={12} />
      <span>{selected.alias || selected.projectName}</span>
    </div>
    {#if selected.alias && selected.projectName !== selected.alias}
      <div class="sub">{selected.projectName}</div>
    {/if}
    <div class="meta">
      <span>{fmtDate(selected.modTime)}</span>
      <span class="sep">·</span>
      <span>{selected.messageCount}msg</span>
      {#if selected.gitBranch}
        <span class="sep">·</span>
        <span class="branch">⎇ {selected.gitBranch}</span>
      {/if}
    </div>
    {#if selected.lastUserMsg}
      <div class="msg">
        <div class="label">LAST MESSAGE</div>
        <div class="content">{selected.lastUserMsg}</div>
      </div>
    {/if}
  {:else}
    <div class="empty">› hover or select a session</div>
  {/if}
</div>

<style>
  .preview {
    height: 100%;
    overflow-y: auto;
    padding: 8px 10px;
    font-size: 11px;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    color: var(--accent-claude);
    font-weight: 700;
  }

  .row.codex {
    color: var(--accent-codex);
  }

  .sub {
    color: var(--fg-mute);
    margin: 2px 0 6px 18px;
    font-size: 10px;
  }

  .meta {
    display: flex;
    gap: 5px;
    color: var(--fg-mute);
    font-size: 10px;
    margin: 6px 0 10px;
  }

  .sep {
    color: var(--border);
  }

  .branch {
    color: var(--accent-folder);
  }

  .msg .label {
    color: var(--accent-action);
    font-size: 10px;
    letter-spacing: 1px;
    margin-bottom: 4px;
  }

  .msg .content {
    color: var(--fg-dim);
    white-space: pre-wrap;
    word-break: break-word;
    line-height: 1.4;
    max-height: 200px;
    overflow-y: auto;
  }

  .empty {
    color: var(--fg-mute);
    padding: 16px 0;
    font-size: 11px;
  }
</style>
