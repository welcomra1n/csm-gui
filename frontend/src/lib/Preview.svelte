<script lang="ts">
  import { selectedSessionId, sessions } from "./store";
  import ProviderIcon from "./ProviderIcon.svelte";
  import { marked } from "marked";
  import DOMPurify from "dompurify";
  import { GenerateRecap } from "../../wailsjs/go/main/App.js";

  $: selected = $sessions.find((s) => s.id === $selectedSessionId);

  let generating = false;

  async function regenerate() {
    if (!selected) return;
    generating = true;
    try {
      const recap = await GenerateRecap(selected.id, true);
      // mutate local sessions array to update UI
      sessions.update((arr) =>
        arr.map((s) => (s.id === selected!.id ? { ...s, recap } : s))
      );
    } catch (e: any) {
      console.error("recap:", e);
    } finally {
      generating = false;
    }
  }

  function renderMarkdown(src: string): string {
    if (!src) return "";
    const html = marked.parse(src, { breaks: true, gfm: true }) as string;
    return DOMPurify.sanitize(html);
  }

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
    <div class="msg recap-section">
      <div class="label-row">
        <span class="label recap-label">RECAP</span>
        <button class="regen" on:click={regenerate} disabled={generating} title="regenerate recap">
          {generating ? "…" : "↻"}
        </button>
      </div>
      {#if selected.recap}
        <div class="content md">{@html renderMarkdown(selected.recap)}</div>
      {:else if generating}
        <div class="empty">generating recap…</div>
      {:else}
        <div class="empty">click ↻ to generate</div>
      {/if}
    </div>

    {#if selected.firstUserMsg}
      <div class="msg">
        <div class="label">TOPIC</div>
        <div class="content md">{@html renderMarkdown(selected.firstUserMsg)}</div>
      </div>
    {/if}
    {#if selected.lastUserMsg && selected.lastUserMsg !== selected.firstUserMsg}
      <div class="msg">
        <div class="label">RECENT</div>
        <div class="content md">{@html renderMarkdown(selected.lastUserMsg)}</div>
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
    font-size: var(--ui-fs-sm);
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
    font-size: var(--ui-fs-xs);
  }

  .meta {
    display: flex;
    gap: 5px;
    color: var(--fg-mute);
    font-size: var(--ui-fs-xs);
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
    font-size: var(--ui-fs-xs);
    letter-spacing: 1px;
    margin-bottom: 4px;
  }

  .label-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 4px;
  }

  .recap-label {
    color: var(--fg);
    margin-bottom: 0;
  }

  .regen {
    color: var(--fg-mute);
    padding: 1px 6px;
    border: 1px solid var(--border);
    border-radius: 2px;
    font-size: var(--ui-fs-xs);
  }

  .regen:hover:not(:disabled) {
    color: var(--fg);
    border-color: var(--fg-mute);
  }

  .recap-section {
    padding: 6px 8px;
    background: var(--bg-hover);
    border: 1px solid var(--border);
    border-radius: 3px;
    margin-bottom: 10px;
  }

  .msg .content {
    color: var(--fg-dim);
    white-space: pre-wrap;
    word-break: break-word;
    line-height: 1.4;
    max-height: 200px;
    overflow-y: auto;
  }

  .msg .content.md :global(p) { margin: 0 0 6px; }
  .msg .content.md :global(code) {
    background: var(--bg);
    color: var(--accent-pinned);
    padding: 1px 4px;
    border-radius: 2px;
    font-size: 0.9em;
  }
  .msg .content.md :global(pre) {
    background: var(--bg);
    padding: 6px 8px;
    border-radius: 2px;
    border: 1px solid var(--border);
    overflow-x: auto;
  }
  .msg .content.md :global(pre code) {
    background: none;
    padding: 0;
    color: var(--fg);
  }
  .msg .content.md :global(a) { color: var(--accent-folder); }
  .msg .content.md :global(strong) { color: var(--fg); }
  .msg .content.md :global(ul), .msg .content.md :global(ol) { padding-left: 18px; margin: 4px 0; }
  .msg .content.md :global(h1), .msg .content.md :global(h2), .msg .content.md :global(h3) {
    color: var(--fg);
    margin: 8px 0 4px;
    font-size: var(--ui-fs);
  }

  .empty {
    color: var(--fg-mute);
    padding: 16px 0;
    font-size: var(--ui-fs-sm);
  }
</style>
