<script lang="ts">
  import { onMount } from "svelte";
  import { sessions, tabs, activeTabId, nextTabId, statusText, selectedSessionId } from "./store";
  import type { Session } from "./types";
  import {
    ListSessions,
    StartPty,
    PinSession,
    RenameAlias,
    SetSessionTag,
    DeleteSession,
  } from "../../wailsjs/go/main/App.js";
  import ProviderIcon from "./ProviderIcon.svelte";
  import ContextMenu from "./ContextMenu.svelte";
  import PromptModal from "./PromptModal.svelte";

  let filter = "";
  let newMenuOpen = false;
  let ctxMenu: { x: number; y: number; session: Session } | null = null;
  let modal:
    | { kind: "rename" | "tag" | "delete"; session: Session; value: string }
    | null = null;

  onMount(() => {
    refresh();
    const id = setInterval(refresh, 15000);
    const onFocus = () => refresh();
    window.addEventListener("focus", onFocus);
    return () => {
      clearInterval(id);
      window.removeEventListener("focus", onFocus);
    };
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
      setTimeout(refresh, 3000);
      setTimeout(refresh, 10000);
    } catch (e: any) {
      statusText.set(`fail: ${e?.message || e}`);
    }
  }

  function openContext(e: MouseEvent, s: Session) {
    e.preventDefault();
    selectedSessionId.set(s.id);
    ctxMenu = { x: e.clientX, y: e.clientY, session: s };
  }

  function buildMenuItems(s: Session) {
    return [
      { label: "rename alias", action: () => (modal = { kind: "rename", session: s, value: s.alias || "" }), key: "F2" },
      { label: s.pinned ? "unpin" : "pin", action: () => togglePin(s), key: "P" },
      { label: "edit tags", action: () => (modal = { kind: "tag", session: s, value: (s.tags || []).join(", ") }), key: "T" },
      { label: "delete", action: () => (modal = { kind: "delete", session: s, value: "" }), danger: true, key: "Del" },
    ];
  }

  async function togglePin(s: Session) {
    try {
      await PinSession(s.id, !s.pinned);
      await refresh();
      statusText.set(`${s.pinned ? "unpinned" : "pinned"}: ${s.projectName}`);
    } catch (e: any) {
      statusText.set(`fail: ${e?.message || e}`);
    }
  }

  async function confirmModal() {
    if (!modal) return;
    const { kind, session, value } = modal;
    try {
      if (kind === "rename") {
        await RenameAlias(session.id, value.trim());
        statusText.set(`renamed: ${value.trim() || session.projectName}`);
      } else if (kind === "tag") {
        const tags = value
          .split(",")
          .map((t) => t.trim())
          .filter(Boolean);
        await SetSessionTag(session.id, tags);
        statusText.set(`tagged: ${tags.length} tag(s)`);
      } else if (kind === "delete") {
        await DeleteSession(session.id);
        statusText.set(`deleted: ${session.projectName}`);
        // Close any open tab for this session
        const tab = $tabs.find((t) => t.sessionId === session.id);
        if (tab) {
          tabs.update((arr) => arr.filter((t) => t.id !== tab.id));
        }
      }
      modal = null;
      await refresh();
    } catch (e: any) {
      statusText.set(`fail: ${e?.message || e}`);
    }
  }

  function handleSessionKey(e: KeyboardEvent, s: Session) {
    if (e.key === "F2") {
      e.preventDefault();
      modal = { kind: "rename", session: s, value: s.alias || "" };
    } else if (e.key === "p" || e.key === "P") {
      e.preventDefault();
      togglePin(s);
    } else if (e.key === "t" || e.key === "T") {
      e.preventDefault();
      modal = { kind: "tag", session: s, value: (s.tags || []).join(", ") };
    } else if (e.key === "Delete" || e.key === "Backspace") {
      e.preventDefault();
      modal = { kind: "delete", session: s, value: "" };
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
      setTimeout(refresh, 3000);
      setTimeout(refresh, 10000);
      setTimeout(refresh, 30000);
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
    <button class="new-btn" on:click={() => (newMenuOpen = !newMenuOpen)} title="new session">
      + NEW SESSION
    </button>
    {#if newMenuOpen}
      <div class="new-menu">
        <button class="menu-item claude" on:click={() => { newMenuOpen = false; newSession("claude"); }}>
          <ProviderIcon provider="claude" size={12} /> Claude
        </button>
        <button class="menu-item codex" on:click={() => { newMenuOpen = false; newSession("codex"); }}>
          <ProviderIcon provider="codex" size={12} /> Codex
        </button>
      </div>
    {/if}
  </div>

  <div class="list">
    {#if pinned.length > 0}
      <div class="group-header pinned">📌 PINNED · {pinned.length}</div>
      {#each pinned as s (s.id)}
        <button
          class="item"
          class:selected={$selectedSessionId === s.id}
          class:codex={s.provider === "codex"}
          class:pinned={s.pinned}
          on:mouseenter={() => selectedSessionId.set(s.id)}
          on:click={() => openSession(s)}
          on:contextmenu={(e) => openContext(e, s)}
          on:keydown={(e) => handleSessionKey(e, s)}
        >
          {#if s.pinned}<span class="pin">★</span>{/if}
          <span class="icon"><ProviderIcon provider={s.provider} /></span>
          <span class="name">{s.alias || s.projectName}</span>
          {#if s.tags && s.tags.length > 0}
            <span class="tags">
              {#each s.tags as t}<span class="tag">#{t}</span>{/each}
            </span>
          {/if}
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
          class:pinned={s.pinned}
          on:mouseenter={() => selectedSessionId.set(s.id)}
          on:click={() => openSession(s)}
          on:contextmenu={(e) => openContext(e, s)}
          on:keydown={(e) => handleSessionKey(e, s)}
        >
          {#if s.pinned}<span class="pin">★</span>{/if}
          <span class="icon"><ProviderIcon provider={s.provider} /></span>
          <span class="name">{s.alias || s.projectName}</span>
          {#if s.tags && s.tags.length > 0}
            <span class="tags">
              {#each s.tags as t}<span class="tag">#{t}</span>{/each}
            </span>
          {/if}
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
          class:pinned={s.pinned}
          on:mouseenter={() => selectedSessionId.set(s.id)}
          on:click={() => openSession(s)}
          on:contextmenu={(e) => openContext(e, s)}
          on:keydown={(e) => handleSessionKey(e, s)}
        >
          {#if s.pinned}<span class="pin">★</span>{/if}
          <span class="icon"><ProviderIcon provider={s.provider} /></span>
          <span class="name">{s.alias || s.projectName}</span>
          {#if s.tags && s.tags.length > 0}
            <span class="tags">
              {#each s.tags as t}<span class="tag">#{t}</span>{/each}
            </span>
          {/if}
          <span class="time">{fmtTime(s.modTime)}</span>
        </button>
      {/each}
    {/if}

    {#if filtered.length === 0}
      <div class="empty">no sessions</div>
    {/if}
  </div>
</div>

{#if ctxMenu}
  <ContextMenu
    x={ctxMenu.x}
    y={ctxMenu.y}
    items={buildMenuItems(ctxMenu.session)}
    onClose={() => (ctxMenu = null)}
  />
{/if}

{#if modal}
  <PromptModal
    title={modal.kind === "rename"
      ? `Rename: ${modal.session.projectName}`
      : modal.kind === "tag"
        ? `Tags (comma-separated): ${modal.session.projectName}`
        : `Delete "${modal.session.projectName}"?`}
    bind:value={modal.value}
    placeholder={modal.kind === "tag" ? "tag1, tag2" : ""}
    confirmLabel={modal.kind === "delete" ? "DELETE" : "save"}
    danger={modal.kind === "delete"}
    onConfirm={confirmModal}
    onCancel={() => (modal = null)}
  />
{/if}

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
    font-size: var(--ui-fs);
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
    position: relative;
    padding: 4px 6px 6px;
    border-bottom: 1px solid var(--border);
  }

  .new-btn {
    width: 100%;
    padding: 5px;
    font-size: var(--ui-fs-xs);
    letter-spacing: 1px;
    border: 1px solid var(--border);
    border-radius: 2px;
    color: var(--fg);
  }

  .new-btn:hover {
    border-color: var(--fg);
    box-shadow: 0 0 6px var(--fg-mute);
  }

  .new-menu {
    position: absolute;
    top: 100%;
    left: 6px;
    right: 6px;
    background: var(--bg-elev);
    border: 1px solid var(--fg-mute);
    border-radius: 2px;
    z-index: 10;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.6);
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 6px 10px;
    font-size: var(--ui-fs-sm);
    text-align: left;
    color: var(--fg-dim);
  }

  .menu-item.claude:hover {
    background: var(--bg-hover);
    color: var(--accent-claude);
  }

  .menu-item.codex:hover {
    background: var(--bg-hover);
    color: var(--accent-codex);
  }

  .list {
    flex: 1;
    overflow-y: auto;
    padding: 4px 0;
  }

  .group-header {
    padding: 6px 10px 3px;
    font-size: var(--ui-fs-xs);
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
    font-size: var(--ui-fs);
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

  .pin {
    color: var(--accent-pinned);
    font-size: calc(var(--ui-fs-xs) - 1px);
    margin-right: -2px;
  }

  .tags {
    display: flex;
    gap: 3px;
    flex-shrink: 0;
  }

  .tag {
    color: var(--accent-action);
    font-size: calc(var(--ui-fs-xs) - 1px);
    background: rgba(255, 77, 139, 0.12);
    padding: 1px 4px;
    border-radius: 2px;
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
    font-size: var(--ui-fs-xs);
  }

  .empty {
    text-align: center;
    color: var(--fg-mute);
    padding: 20px 10px;
    font-size: var(--ui-fs-sm);
  }
</style>
