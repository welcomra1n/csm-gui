<script lang="ts">
  import { onMount } from "svelte";
  import { sessions, tabs, activeTabId, nextTabId, statusText, selectedSessionId } from "./store";
  import type { Session } from "./types";
  import { ListSessions, StartPty } from "../../wailsjs/go/main/App.js";
  import ProviderIcon from "./ProviderIcon.svelte";

  let filter = "";

  onMount(async () => {
    await refresh();
  });

  async function refresh() {
    try {
      const list = await ListSessions();
      sessions.set(list || []);
    } catch (e) {
      console.error("ListSessions:", e);
    }
  }

  async function openSession(s: Session) {
    // Already open? focus existing tab
    const existing = $tabs.find((t) => t.sessionId === s.id);
    if (existing) {
      activeTabId.set(existing.id);
      statusText.set(`focus: ${s.projectName}`);
      return;
    }

    const tabId = nextTabId();
    const isCodex = s.provider === "codex";
    const cmd = isCodex ? "codex" : "claude";
    const args = isCodex
      ? ["resume", s.id, "--sandbox", "danger-full-access"]
      : ["--resume", s.id, "--dangerously-skip-permissions"];

    const dir = s.projectDir || s.cwd || "";

    try {
      await StartPty(tabId, cmd, dir, args, 80, 24);
      tabs.update((arr) => [
        ...arr,
        {
          id: tabId,
          title: s.alias || s.projectName || s.id.slice(0, 8),
          sessionId: s.id,
          provider: s.provider,
        },
      ]);
      activeTabId.set(tabId);
      statusText.set(`open: ${s.projectName}`);
    } catch (e: any) {
      statusText.set(`fail: ${e?.message || e}`);
    }
  }

  async function newSession(provider: "claude" | "codex") {
    const tabId = nextTabId();
    const cmd = provider === "codex" ? "codex" : "claude";
    const args = provider === "codex"
      ? ["--sandbox", "danger-full-access"]
      : ["--dangerously-skip-permissions"];

    // Use home dir for new sessions
    const dir = "";

    try {
      await StartPty(tabId, cmd, dir, args, 80, 24);
      tabs.update((arr) => [
        ...arr,
        {
          id: tabId,
          title: `new ${provider}`,
          provider,
        },
      ]);
      activeTabId.set(tabId);
      statusText.set(`new ${provider} session`);
    } catch (e: any) {
      statusText.set(`fail: ${e?.message || e}`);
    }
  }

  $: filtered = (() => {
    const f = filter.toLowerCase().trim();
    const all = $sessions;
    if (!f) return all;
    return all.filter((s) => {
      const hay = (
        s.projectName +
        " " +
        (s.alias || "") +
        " " +
        (s.lastUserMsg || "")
      ).toLowerCase();
      return hay.includes(f);
    });
  })();

  $: pinned = filtered.filter((s) => s.pinned);
  $: folders = (() => {
    const map = new Map<string, Session[]>();
    for (const s of filtered) {
      if (s.folder && !s.pinned) {
        const arr = map.get(s.folder) || [];
        arr.push(s);
        map.set(s.folder, arr);
      }
    }
    return Array.from(map.entries()).sort(([a], [b]) => a.localeCompare(b));
  })();
  $: regular = filtered.filter((s) => !s.pinned && !s.folder);

  function fmtTime(iso: string): string {
    if (!iso) return "";
    const d = new Date(iso);
    const now = new Date();
    const diff = (now.getTime() - d.getTime()) / 1000;
    if (diff < 60) return "방금";
    if (diff < 3600) return `${Math.floor(diff / 60)}분`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}시간`;
    return `${d.getMonth() + 1}/${d.getDate()}`;
  }
</script>

<div class="browser">
  <div class="searchbox">
    <input
      type="text"
      placeholder="search…"
      bind:value={filter}
      autocomplete="off"
    />
    <button class="refresh" on:click={refresh} title="refresh">↻</button>
  </div>

  <div class="new-row">
    <button class="new-btn claude" on:click={() => newSession("claude")} title="new claude session">
      <ProviderIcon provider="claude" size={11} /> + new claude
    </button>
    <button class="new-btn codex" on:click={() => newSession("codex")} title="new codex session">
      <ProviderIcon provider="codex" size={11} /> + new codex
    </button>
  </div>

  <div class="list">
    {#if pinned.length > 0}
      <div class="group-header pinned">📌 PINNED · {pinned.length}</div>
      {#each pinned as s (s.id)}
        <button
          class="item"
          class:selected={$selectedSessionId === s.id}
          class:codex={s.provider === "codex"}
          on:mouseenter={() => selectedSessionId.set(s.id)}
          on:click={() => openSession(s)}
        >
          <span class="icon"><ProviderIcon provider={s.provider} /></span>
          <span class="name">{s.alias || s.projectName}</span>
          <span class="time">{fmtTime(s.modTime)}</span>
        </button>
      {/each}
    {/if}

    {#each folders as [folderName, items] (folderName)}
      <div class="group-header folder">📁 {folderName.toUpperCase()} · {items.length}</div>
      {#each items as s (s.id)}
        <button
          class="item"
          class:selected={$selectedSessionId === s.id}
          class:codex={s.provider === "codex"}
          on:mouseenter={() => selectedSessionId.set(s.id)}
          on:click={() => openSession(s)}
        >
          <span class="icon"><ProviderIcon provider={s.provider} /></span>
          <span class="name">{s.alias || s.projectName}</span>
          <span class="time">{fmtTime(s.modTime)}</span>
        </button>
      {/each}
    {/each}

    {#if regular.length > 0}
      <div class="group-header normal">📂 ALL · {regular.length}</div>
      {#each regular as s (s.id)}
        <button
          class="item"
          class:selected={$selectedSessionId === s.id}
          class:codex={s.provider === "codex"}
          on:mouseenter={() => selectedSessionId.set(s.id)}
          on:click={() => openSession(s)}
        >
          <span class="icon"><ProviderIcon provider={s.provider} /></span>
          <span class="name">{s.alias || s.projectName}</span>
          <span class="time">{fmtTime(s.modTime)}</span>
        </button>
      {/each}
    {/if}

    {#if filtered.length === 0}
      <div class="empty">세션 없음</div>
    {/if}
  </div>
</div>

<style>
  .browser {
    display: flex;
    flex-direction: column;
    height: 100%;
  }

  .searchbox {
    display: flex;
    gap: 4px;
    padding: 6px 8px;
    border-bottom: 1px solid var(--border);
  }

  .searchbox input {
    flex: 1;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 2px;
    padding: 3px 6px;
    color: var(--fg);
    font-size: 12px;
  }

  .searchbox input:focus {
    border-color: var(--fg-mute);
  }

  .refresh {
    width: 26px;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 2px;
    color: var(--fg-dim);
  }

  .refresh:hover {
    background: var(--bg-hover);
    color: var(--fg);
  }

  .new-row {
    display: flex;
    gap: 4px;
    padding: 4px 6px 6px;
    border-bottom: 1px solid var(--border);
  }

  .new-btn {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    padding: 4px;
    font-size: 10px;
    border: 1px solid var(--border);
    border-radius: 2px;
    color: var(--fg-dim);
  }

  .new-btn.claude:hover {
    border-color: var(--accent-claude);
    color: var(--accent-claude);
  }

  .new-btn.codex:hover {
    border-color: var(--accent-codex);
    color: var(--accent-codex);
  }

  .list {
    flex: 1;
    overflow-y: auto;
    padding: 4px 0;
  }

  .group-header {
    padding: 6px 10px 3px;
    font-size: 10px;
    letter-spacing: 1px;
  }

  .group-header.pinned { color: var(--accent-pinned); }
  .group-header.folder { color: var(--accent-folder); }
  .group-header.normal { color: var(--fg-mute); }

  .item {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 4px 10px 4px 22px;
    text-align: left;
    color: var(--fg-dim);
    position: relative;
    font-size: 12px;
  }

  .item::before {
    content: "";
    position: absolute;
    left: 14px;
    top: 0;
    bottom: 0;
    width: 1px;
    background: var(--border);
  }

  .item:hover, .item.selected {
    background: var(--bg-hover);
    color: var(--fg);
  }

  .item:hover::before, .item.selected::before {
    background: var(--fg-mute);
  }

  .icon {
    flex: 0 0 auto;
    color: var(--accent-claude);
    display: flex;
  }

  .item.codex .icon {
    color: var(--accent-codex);
  }

  .name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .time {
    flex: 0 0 auto;
    color: var(--fg-mute);
    font-size: 10px;
  }

  .empty {
    text-align: center;
    color: var(--fg-mute);
    padding: 20px 10px;
    font-size: 11px;
  }
</style>
