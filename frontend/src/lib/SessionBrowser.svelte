<script lang="ts">
  import { onMount } from "svelte";
  import { sessions, tabs, activeTabId, nextTabId, statusText } from "./store";
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
      statusText.set(`열림: ${s.projectName}`);
    } catch (e: any) {
      statusText.set(`실패: ${e?.message || e}`);
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
      placeholder="세션 검색…"
      bind:value={filter}
      autocomplete="off"
    />
    <button class="refresh" on:click={refresh} title="새로고침">↻</button>
  </div>

  <div class="list">
    {#if pinned.length > 0}
      <div class="group-header">📌 고정 ({pinned.length})</div>
      {#each pinned as s (s.id)}
        <button class="item" on:click={() => openSession(s)}>
          <span class="icon"><ProviderIcon provider={s.provider} /></span>
          <span class="name">{s.alias || s.projectName}</span>
          <span class="time">{fmtTime(s.modTime)}</span>
        </button>
      {/each}
    {/if}

    {#each folders as [folderName, items] (folderName)}
      <div class="group-header">📁 {folderName} ({items.length})</div>
      {#each items as s (s.id)}
        <button class="item" on:click={() => openSession(s)}>
          <span class="icon"><ProviderIcon provider={s.provider} /></span>
          <span class="name">{s.alias || s.projectName}</span>
          <span class="time">{fmtTime(s.modTime)}</span>
        </button>
      {/each}
    {/each}

    {#if regular.length > 0}
      <div class="group-header">📂 일반 ({regular.length})</div>
      {#each regular as s (s.id)}
        <button class="item" on:click={() => openSession(s)}>
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
    padding: 8px;
    border-bottom: 1px solid #2a2a2e;
  }

  .searchbox input {
    flex: 1;
    background: #2a2a2e;
    border: 1px solid #3a3a3e;
    border-radius: 4px;
    padding: 4px 8px;
    color: #e6e6e6;
    font-size: 12px;
  }

  .refresh {
    width: 28px;
    background: #2a2a2e;
    border-radius: 4px;
    color: #aaa;
  }

  .refresh:hover {
    background: #3a3a3e;
    color: #e6e6e6;
  }

  .list {
    flex: 1;
    overflow-y: auto;
    padding: 4px 0;
  }

  .group-header {
    padding: 8px 12px 4px;
    font-size: 11px;
    color: #888;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 6px 12px 6px 28px;
    text-align: left;
    color: #c0c0c0;
    position: relative;
  }

  .item::before {
    content: "";
    position: absolute;
    left: 18px;
    top: 0;
    bottom: 0;
    width: 1px;
    background: #2a2a2e;
  }

  .item:hover {
    background: #2a2a2e;
    color: #e6e6e6;
  }

  .icon {
    flex: 0 0 auto;
  }

  .name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 12px;
  }

  .time {
    flex: 0 0 auto;
    color: #666;
    font-size: 11px;
  }

  .empty {
    text-align: center;
    color: #555;
    padding: 24px 12px;
    font-size: 12px;
  }
</style>
