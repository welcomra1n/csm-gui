<script lang="ts">
  import { onMount } from "svelte";
  import { sessions, tabs, activeTabId, nextTabId, statusText, selectedSessionId, focusSearch } from "./store";
  import type { Session } from "./types";
  import {
    ListSessions,
    StartPty,
    PinSession,
    RenameAlias,
    SetSessionTag,
    DeleteSession,
    SetSessionFolder,
    CreateFolder,
    RenameFolder,
    DeleteFolder,
    ListFolders,
  } from "../../wailsjs/go/main/App.js";
  import ProviderIcon from "./ProviderIcon.svelte";
  import ContextMenu from "./ContextMenu.svelte";
  import PromptModal from "./PromptModal.svelte";

  let filter = "";
  let newMenuOpen = false;
  let searchEl: HTMLInputElement;

  $: if ($focusSearch && searchEl) {
    searchEl.focus();
    searchEl.select();
  }
  let ctxMenu: { x: number; y: number; session: Session } | null = null;
  let modal:
    | { kind: "rename" | "tag"; session: Session; value: string }
    | null = null;
  let newTitleModal: { provider: "claude" | "codex" | "shell"; value: string } | null = null;
  let folderModal: { kind: "create" | "rename"; oldName?: string; value: string } | null = null;
  let folderCtx: { x: number; y: number; folder: string } | null = null;
  let draggedSession: Session | null = null;

  function onSessionDragStart(e: DragEvent, s: Session) {
    draggedSession = s;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", s.id);
    }
  }

  function onFolderDragOver(e: DragEvent) {
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
  }

  async function onFolderDrop(e: DragEvent, folder: string) {
    e.preventDefault();
    if (!draggedSession) return;
    const s = draggedSession;
    draggedSession = null;
    try {
      await SetSessionFolder(s.id, folder);
      statusText.set(`${s.alias || s.projectName} → ${folder}`);
      await refresh();
    } catch (err: any) {
      statusText.set(`fail: ${err?.message || err}`);
    }
  }

  function openFolderCtx(e: MouseEvent, folder: string) {
    e.preventDefault();
    e.stopPropagation();
    folderCtx = null;
    const x = e.clientX;
    const y = e.clientY;
    setTimeout(() => (folderCtx = { x, y, folder }), 0);
  }

  function folderMenuItems(folder: string) {
    return [
      {
        label: "rename folder",
        action: () => (folderModal = { kind: "rename", oldName: folder, value: folder }),
      },
      {
        label: "delete folder",
        action: () => deleteFolderCmd(folder),
        danger: true,
      },
    ];
  }

  async function deleteFolderCmd(name: string) {
    if (!confirm(`Delete folder "${name}"? Sessions inside become unfiled.`)) return;
    try {
      await DeleteFolder(name);
      await refresh();
      statusText.set(`folder deleted: ${name}`);
    } catch (e: any) {
      statusText.set(`fail: ${e?.message || e}`);
    }
  }

  async function confirmFolderModal() {
    if (!folderModal) return;
    const m = folderModal;
    folderModal = null;
    try {
      if (m.kind === "create") {
        await CreateFolder(m.value);
        statusText.set(`folder created: ${m.value}`);
      } else if (m.kind === "rename" && m.oldName) {
        await RenameFolder(m.oldName, m.value);
        statusText.set(`folder: ${m.oldName} → ${m.value}`);
      }
      await refresh();
    } catch (e: any) {
      statusText.set(`fail: ${e?.message || e}`);
    }
  }

  let prevRunningAgents = new Set<string>();
  let audioCtx: AudioContext | null = null;

  function playDing() {
    try {
      if (!audioCtx) audioCtx = new (window.AudioContext || (window as any).webkitAudioContext)();
      const t = audioCtx.currentTime;
      const osc = audioCtx.createOscillator();
      const gain = audioCtx.createGain();
      osc.connect(gain);
      gain.connect(audioCtx.destination);
      osc.frequency.setValueAtTime(880, t);
      osc.frequency.exponentialRampToValueAtTime(1320, t + 0.08);
      gain.gain.setValueAtTime(0.0001, t);
      gain.gain.exponentialRampToValueAtTime(0.15, t + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.0001, t + 0.3);
      osc.start(t);
      osc.stop(t + 0.35);
    } catch (e) {
      // ignore
    }
  }

  onMount(() => {
    refresh();
    if ("Notification" in window && Notification.permission === "default") {
      Notification.requestPermission().catch(() => {});
    }
    const id = setInterval(refresh, 15000);
    const onFocus = () => refresh();
    window.addEventListener("focus", onFocus);
    return () => {
      clearInterval(id);
      window.removeEventListener("focus", onFocus);
    };
  });

  function notify(title: string, body: string) {
    try {
      if ("Notification" in window && Notification.permission === "granted") {
        new Notification(title, { body, silent: true });
      }
    } catch (e) {
      // ignore
    }
  }

  let allFolderNames: string[] = [];

  async function refreshFolders() {
    try {
      allFolderNames = (await ListFolders()) || [];
    } catch (e) {
      console.warn("ListFolders:", e);
    }
  }

  async function refresh() {
    try {
      refreshFolders();
      const list = await ListSessions();
      // Detect newly completed subagents
      const newRunning = new Set<string>();
      for (const s of list || []) {
        for (const a of s.subagents || []) {
          if (!a.completed) newRunning.add(a.toolUseId);
        }
      }
      // Anything previously running that is no longer running = completed
      let completedCount = 0;
      for (const id of prevRunningAgents) {
        if (!newRunning.has(id)) completedCount++;
      }
      if (completedCount > 0 && prevRunningAgents.size > 0) {
        playDing();
        // Only show system notification when window not focused
        if (!document.hasFocus()) {
          notify("agent done", `${completedCount} subagent${completedCount > 1 ? "s" : ""} completed`);
        }
      }
      prevRunningAgents = newRunning;
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
      await StartPty(tabId, cmd, dir, args, 120, 40);
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
    e.stopPropagation();
    selectedSessionId.set(s.id);
    // Force unmount first so onMount re-runs and listeners reset
    ctxMenu = null;
    const x = e.clientX;
    const y = e.clientY;
    setTimeout(() => {
      ctxMenu = { x, y, session: s };
    }, 0);
  }

  function findGroup(s: Session) {
    const allGroups = [...pinnedGroups, ...regularGroups];
    return allGroups.find((g) => g.head.id === s.id || g.rest.some((r) => r.id === s.id));
  }

  function buildMenuItems(s: Session) {
    const group = findGroup(s);
    const groupSize = group ? group.rest.length + 1 : 1;
    const items: any[] = [
      { label: "rename alias", action: () => (modal = { kind: "rename", session: s, value: s.alias || "" }), key: "F2" },
      { label: s.pinned ? "unpin" : "pin", action: () => togglePin(s), key: "P" },
      { label: "edit tags", action: () => (modal = { kind: "tag", session: s, value: (s.tags || []).join(", ") }), key: "T" },
      { label: "delete", action: () => deleteSession(s), danger: true, key: "Del" },
    ];
    if (groupSize > 1) {
      items.push({
        label: `delete all ${groupSize} in group`,
        action: () => deleteGroup(group!),
        danger: true,
      });
    }
    return items;
  }

  async function deleteGroup(g: { head: Session; rest: Session[] }) {
    const all = [g.head, ...g.rest];
    if (!confirm(`Delete ${all.length} sessions named "${g.head.alias || g.head.projectName}"?`)) return;
    statusText.set(`deleting ${all.length}…`);
    let ok = 0;
    for (const s of all) {
      try {
        await DeleteSession(s.id);
        ok++;
        const tab = $tabs.find((t) => t.sessionId === s.id);
        if (tab) tabs.update((arr) => arr.filter((t) => t.id !== tab.id));
      } catch (e) {
        console.error("delete:", e);
      }
    }
    statusText.set(`deleted ${ok}/${all.length}`);
    await refresh();
  }

  async function deleteSession(s: Session) {
    try {
      await DeleteSession(s.id);
      statusText.set(`deleted: ${s.alias || s.projectName}`);
      const tab = $tabs.find((t) => t.sessionId === s.id);
      if (tab) tabs.update((arr) => arr.filter((t) => t.id !== tab.id));
      await refresh();
    } catch (e: any) {
      statusText.set(`fail: ${e?.message || e}`);
    }
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
    } else if (e.key === "Delete") {
      e.preventDefault();
      deleteSession(s);
    }
  }

  function askNewSession(provider: "claude" | "codex" | "shell") {
    newTitleModal = { provider, value: "" };
  }

  function confirmNewSession() {
    if (!newTitleModal) return;
    const m = newTitleModal;
    newTitleModal = null;
    newSession(m.provider, m.value);
  }

  function cancelNewSession() {
    newTitleModal = null;
  }

  async function newSession(provider: "claude" | "codex" | "shell", title: string = "") {
    const tabId = nextTabId();
    let cmd: string;
    let args: string[] = [];
    if (provider === "codex") {
      cmd = "codex";
      args = ["--sandbox", "danger-full-access"];
    } else if (provider === "shell") {
      cmd = navigator.platform.toLowerCase().includes("win") ? "pwsh.exe" : "zsh";
      args = ["-l"];
    } else {
      cmd = "claude";
      args = ["--dangerously-skip-permissions"];
    }

    // Use home dir for new sessions
    const dir = "";

    try {
      await StartPty(tabId, cmd, dir, args, 120, 40);
      const finalTitle = title.trim() || `new ${provider}`;
      tabs.update((arr) => [
        ...arr,
        {
          id: tabId,
          title: finalTitle,
          provider,
        },
      ]);
      activeTabId.set(tabId);
      statusText.set(`new ${provider} session`);
      // If user provided title, schedule alias persistence once JSONL appears
      if (title.trim()) {
        const wanted = title.trim();
        let attempts = 0;
        const tryApply = async () => {
          attempts++;
          const list = $sessions;
          // Find newest session matching the active tab (no sessionId yet — match by recency + provider)
          const candidate = list
            .filter((s) => s.provider === provider && !openIds.has(s.id))
            .sort((a, b) => (b.modTime || "").localeCompare(a.modTime || ""))[0];
          if (candidate) {
            try {
              await RenameAlias(candidate.id, wanted);
              tabs.update((arr) => arr.map((t) => (t.id === tabId ? { ...t, sessionId: candidate.id } : t)));
              await refresh();
              return;
            } catch (e) {
              console.warn("apply alias:", e);
            }
          }
          if (attempts < 6) setTimeout(tryApply, 5000);
        };
        setTimeout(tryApply, 4000);
      }
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
    // seed all known folders so empty ones still show
    for (const name of allFolderNames) {
      map.set(name, []);
    }
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
  $: openIds = new Set($tabs.map((t) => t.sessionId).filter(Boolean));

  // Group sessions by display name (alias or projectName)
  let expandedGroups = new Set<string>();

  function groupSessions(arr: Session[]): { key: string; head: Session; rest: Session[] }[] {
    const map = new Map<string, Session[]>();
    for (const s of arr) {
      const key = (s.alias || s.projectName || "").trim().toLowerCase();
      const list = map.get(key) || [];
      list.push(s);
      map.set(key, list);
    }
    const out: { key: string; head: Session; rest: Session[] }[] = [];
    for (const [key, list] of map) {
      list.sort((a, b) => (b.modTime || "").localeCompare(a.modTime || ""));
      out.push({ key, head: list[0], rest: list.slice(1) });
    }
    out.sort((a, b) => (b.head.modTime || "").localeCompare(a.head.modTime || ""));
    return out;
  }

  function toggleGroup(key: string) {
    if (expandedGroups.has(key)) expandedGroups.delete(key);
    else expandedGroups.add(key);
    expandedGroups = new Set(expandedGroups);
  }

  $: pinnedGroups = groupSessions(pinned);
  $: regularGroups = groupSessions(regular);

  const TAG_PALETTE = [
    "#ff4d8b", "#ffd60a", "#10e0d0", "#4a9eff",
    "#d976ff", "#88ff88", "#ff7eb6", "#7dffff",
    "#e9a6ff", "#ffb84d", "#7ec0ff", "#ffe66d",
  ];

  function tagColor(tag: string): string {
    let h = 0;
    for (let i = 0; i < tag.length; i++) h = (h * 31 + tag.charCodeAt(i)) >>> 0;
    return TAG_PALETTE[h % TAG_PALETTE.length];
  }

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
      bind:this={searchEl}
      type="text"
      placeholder="search… (⌘F)"
      bind:value={filter}
      autocomplete="off"
    />
    <button class="refresh" on:click={refresh} title="refresh">↻</button>
  </div>

  <div class="new-row">
    <button class="new-btn" on:click={() => (newMenuOpen = !newMenuOpen)} title="new session">
      + NEW SESSION
    </button>
    <button
      class="folder-btn"
      on:click={() => (folderModal = { kind: "create", value: "" })}
      title="new folder"
    >
      📁+
    </button>
    {#if newMenuOpen}
      <div class="new-menu">
        <button class="menu-item claude" on:click={() => { newMenuOpen = false; askNewSession("claude"); }}>
          <ProviderIcon provider="claude" size={12} /> Claude
        </button>
        <button class="menu-item codex" on:click={() => { newMenuOpen = false; askNewSession("codex"); }}>
          <ProviderIcon provider="codex" size={12} /> Codex
        </button>
        <button class="menu-item shell" on:click={() => { newMenuOpen = false; askNewSession("shell"); }}>
          <span style="font-family: monospace;">›_</span> Shell
        </button>
      </div>
    {/if}
  </div>

  <div class="list">
    {#if pinnedGroups.length > 0}
      <div class="group-header pinned">📌 PINNED · {pinned.length}</div>
      {#each pinnedGroups as g (g.key)}
        {@const renderList = expandedGroups.has(g.key) ? [g.head, ...g.rest] : [g.head]}
        {#each renderList as s, idx (s.id)}
          <div class="session-block">
            <button
              class="item"
              class:selected={$selectedSessionId === s.id}
              class:codex={s.provider === "codex"}
              class:pinned={s.pinned}
              class:active={openIds.has(s.id)}
              class:nested={idx > 0}
              on:mouseenter={() => selectedSessionId.set(s.id)}
              on:click={() => openSession(s)}
              on:contextmenu={(e) => openContext(e, s)}
              on:keydown={(e) => handleSessionKey(e, s)}
            >
              {#if openIds.has(s.id)}<span class="live-dot"></span>{/if}
              {#if s.pinned && idx === 0}<span class="pin">★</span>{/if}
              <span class="icon"><ProviderIcon provider={s.provider} /></span>
              <span class="name">{s.alias || s.projectName}</span>
              {#if idx === 0 && g.rest.length > 0}
                <span class="count" on:click|stopPropagation={() => toggleGroup(g.key)}>
                  {expandedGroups.has(g.key) ? "▾" : "▸"} {g.rest.length + 1}
                </span>
              {/if}
              {#if s.tags && s.tags.length > 0}
                <span class="tags">
                  {#each s.tags as t}<span class="tag" style="color: {tagColor(t)}; background: {tagColor(t)}22;">#{t}</span>{/each}
                </span>
              {/if}
              <span class="time">{fmtTime(s.modTime)}</span>
            </button>
            {#if s.subagents && s.subagents.length > 0}
              {#each s.subagents.slice(-5) as a (a.toolUseId)}
                <div class="agent" class:running={!a.completed}>
                  <span class="agent-status">{a.completed ? "✓" : "◐"}</span>
                  <span class="agent-name">{a.subagentType || "agent"}</span>
                  <span class="agent-desc">{a.description || ""}</span>
                </div>
              {/each}
            {/if}
          </div>
        {/each}
      {/each}
    {/if}

    {#each folders as [folderName, items] (folderName)}
      <div
        class="group-header folder droppable"
        on:dragover={onFolderDragOver}
        on:drop={(e) => onFolderDrop(e, folderName)}
        on:contextmenu={(e) => openFolderCtx(e, folderName)}
        title="drop session here · right-click for options"
      >
        📁 {folderName.toUpperCase()} · {items.length}
      </div>
      {#each items as s (s.id)}
        <div class="session-block">
          <button
            class="item"
            class:selected={$selectedSessionId === s.id}
            class:codex={s.provider === "codex"}
            class:pinned={s.pinned}
            class:active={openIds.has(s.id)}
            draggable="true"
            on:dragstart={(e) => onSessionDragStart(e, s)}
            on:mouseenter={() => selectedSessionId.set(s.id)}
            on:click={() => openSession(s)}
            on:contextmenu={(e) => openContext(e, s)}
            on:keydown={(e) => handleSessionKey(e, s)}
          >
            {#if openIds.has(s.id)}<span class="live-dot"></span>{/if}
            {#if s.pinned}<span class="pin">★</span>{/if}
            <span class="icon"><ProviderIcon provider={s.provider} /></span>
            <span class="name">{s.alias || s.projectName}</span>
            {#if s.tags && s.tags.length > 0}
              <span class="tags">
                {#each s.tags as t}<span class="tag" style="color: {tagColor(t)}; background: {tagColor(t)}22;">#{t}</span>{/each}
              </span>
            {/if}
            <span class="time">{fmtTime(s.modTime)}</span>
          </button>
          {#if s.subagents && s.subagents.length > 0}
            {#each s.subagents.slice(-5) as a (a.toolUseId)}
              <div class="agent" class:running={!a.completed}>
                <span class="agent-status">{a.completed ? "✓" : "◐"}</span>
                <span class="agent-name">{a.subagentType || "agent"}</span>
                <span class="agent-desc">{a.description || ""}</span>
              </div>
            {/each}
          {/if}
        </div>
      {/each}
    {/each}

    {#if regularGroups.length > 0}
      <div class="group-header normal">📂 ALL · {regular.length}</div>
      {#each regularGroups as g (g.key)}
        {@const renderList = expandedGroups.has(g.key) ? [g.head, ...g.rest] : [g.head]}
        {#each renderList as s, idx (s.id)}
        <div class="session-block">
          <button
            class="item"
            class:selected={$selectedSessionId === s.id}
            class:codex={s.provider === "codex"}
            class:pinned={s.pinned}
            class:active={openIds.has(s.id)}
            class:nested={idx > 0}
            draggable="true"
            on:dragstart={(e) => onSessionDragStart(e, s)}
            on:mouseenter={() => selectedSessionId.set(s.id)}
            on:click={() => openSession(s)}
            on:contextmenu={(e) => openContext(e, s)}
            on:keydown={(e) => handleSessionKey(e, s)}
          >
            {#if openIds.has(s.id)}<span class="live-dot"></span>{/if}
            {#if s.pinned && idx === 0}<span class="pin">★</span>{/if}
            <span class="icon"><ProviderIcon provider={s.provider} /></span>
            <span class="name">{s.alias || s.projectName}</span>
            {#if idx === 0 && g.rest.length > 0}
              <span class="count" on:click|stopPropagation={() => toggleGroup(g.key)}>
                {expandedGroups.has(g.key) ? "▾" : "▸"} {g.rest.length + 1}
              </span>
            {/if}
            {#if s.tags && s.tags.length > 0}
              <span class="tags">
                {#each s.tags as t}<span class="tag" style="color: {tagColor(t)}; background: {tagColor(t)}22;">#{t}</span>{/each}
              </span>
            {/if}
            <span class="time">{fmtTime(s.modTime)}</span>
          </button>
          {#if s.subagents && s.subagents.length > 0}
            {#each s.subagents.slice(-5) as a (a.toolUseId)}
              <div class="agent" class:running={!a.completed}>
                <span class="agent-status">{a.completed ? "✓" : "◐"}</span>
                <span class="agent-name">{a.subagentType || "agent"}</span>
                <span class="agent-desc">{a.description || ""}</span>
              </div>
            {/each}
          {/if}
        </div>
        {/each}
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
      : `Tags (comma-separated): ${modal.session.projectName}`}
    bind:value={modal.value}
    placeholder={modal.kind === "tag" ? "tag1, tag2" : ""}
    confirmLabel="save"
    danger={false}
    onConfirm={confirmModal}
    onCancel={() => (modal = null)}
  />
{/if}

{#if newTitleModal}
  <PromptModal
    title={`New ${newTitleModal.provider} session — title (optional)`}
    bind:value={newTitleModal.value}
    placeholder="leave empty for default name"
    confirmLabel="start"
    danger={false}
    onConfirm={confirmNewSession}
    onCancel={cancelNewSession}
  />
{/if}

{#if folderModal}
  <PromptModal
    title={folderModal.kind === "create" ? "New folder" : `Rename folder: ${folderModal.oldName}`}
    bind:value={folderModal.value}
    placeholder="folder name"
    confirmLabel={folderModal.kind === "create" ? "create" : "rename"}
    danger={false}
    onConfirm={confirmFolderModal}
    onCancel={() => (folderModal = null)}
  />
{/if}

{#if folderCtx}
  <ContextMenu
    x={folderCtx.x}
    y={folderCtx.y}
    items={folderMenuItems(folderCtx.folder)}
    onClose={() => (folderCtx = null)}
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
    display: flex;
    gap: 4px;
    padding: 4px 6px 6px;
    border-bottom: 1px solid var(--border);
  }

  .new-btn {
    flex: 1;
    padding: 5px;
    font-size: var(--ui-fs-xs);
    letter-spacing: 1px;
    border: 1px solid var(--border);
    border-radius: 2px;
    color: var(--fg);
  }

  .folder-btn {
    padding: 5px 8px;
    border: 1px solid var(--border);
    border-radius: 2px;
    color: var(--accent-folder);
    font-size: var(--ui-fs-xs);
  }

  .folder-btn:hover {
    border-color: var(--accent-folder);
  }

  .group-header.droppable {
    cursor: pointer;
  }

  .group-header.droppable:hover {
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

  .menu-item.shell:hover {
    background: var(--bg-hover);
    color: var(--fg);
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

  .item.active {
    color: var(--fg);
  }

  .item.active .name {
    text-shadow: 0 0 6px var(--fg-mute);
  }

  .live-dot {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background: var(--fg);
    margin-left: -8px;
    margin-right: 2px;
    box-shadow: 0 0 6px var(--fg);
    animation: pulse 1.4s ease-in-out infinite;
    flex-shrink: 0;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.3; transform: scale(0.7); }
  }

  .session-block {
    display: flex;
    flex-direction: column;
  }

  .agent {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 2px 10px 2px 38px;
    font-size: var(--ui-fs-xs);
    color: var(--fg-mute);
    position: relative;
  }

  .agent::before {
    content: "";
    position: absolute;
    left: 26px;
    top: 0;
    bottom: 0;
    width: 1px;
    background: var(--border);
  }

  .agent-status {
    color: var(--fg-dim);
    flex-shrink: 0;
  }

  .agent.running .agent-status {
    color: var(--accent-pinned);
    display: inline-block;
    animation: spin 1.2s linear infinite;
  }

  .agent-name {
    color: var(--accent-folder);
    flex-shrink: 0;
  }

  .agent-desc {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }

  @keyframes spin {
    0%   { transform: rotate(0deg); }
    25%  { transform: rotate(90deg); }
    50%  { transform: rotate(180deg); }
    75%  { transform: rotate(270deg); }
    100% { transform: rotate(360deg); }
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

  .item.nested {
    padding-left: 32px;
    color: var(--fg-mute);
  }

  .item.nested .name {
    font-size: var(--ui-fs-sm);
  }

  .count {
    color: var(--accent-pinned);
    font-size: var(--ui-fs-xs);
    padding: 1px 5px;
    background: rgba(255, 214, 10, 0.08);
    border-radius: 2px;
    flex-shrink: 0;
  }

  .count:hover {
    background: rgba(255, 214, 10, 0.2);
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
