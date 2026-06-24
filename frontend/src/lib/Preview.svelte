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
    <div class="row title">
      <ProviderIcon provider={selected.provider} size={14} />
      <span>{selected.alias || selected.projectName}</span>
    </div>
    {#if selected.alias && selected.projectName !== selected.alias}
      <div class="row sub">{selected.projectName}</div>
    {/if}
    <div class="meta">
      <div>{fmtDate(selected.modTime)} · {selected.messageCount}msg</div>
      {#if selected.gitBranch}<div>⎇ {selected.gitBranch}</div>{/if}
    </div>
    {#if selected.lastUserMsg}
      <div class="msg">
        <div class="label">최근 메시지</div>
        <div class="content">{selected.lastUserMsg}</div>
      </div>
    {/if}
  {:else}
    <div class="empty">세션 선택 시 미리보기</div>
  {/if}
</div>

<style>
  .preview {
    height: 100%;
    overflow-y: auto;
    padding: 10px 12px;
    font-size: 12px;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .title {
    color: #e6e6e6;
    font-weight: 700;
  }

  .sub {
    color: #888;
    margin: 2px 0 6px 20px;
    font-size: 11px;
  }

  .meta {
    display: flex;
    gap: 12px;
    color: #666;
    font-size: 11px;
    margin: 6px 0 10px;
  }

  .msg .label {
    color: #888;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    margin-bottom: 4px;
  }

  .msg .content {
    color: #c0c0c0;
    white-space: pre-wrap;
    word-break: break-word;
    line-height: 1.4;
    max-height: 200px;
    overflow-y: auto;
  }

  .empty {
    text-align: center;
    color: #555;
    padding: 24px 0;
  }
</style>
