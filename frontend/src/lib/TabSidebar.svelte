<script lang="ts">
  import { tabs, activeTabId, altHeld } from "./store";
  import { KillPty, RenameAlias, GenerateRecap, DeleteSession } from "../../wailsjs/go/main/App.js";
  import ProviderIcon from "./ProviderIcon.svelte";
  import ContextMenu from "./ContextMenu.svelte";
  import PromptModal from "./PromptModal.svelte";
  import type { Tab } from "./types";

  let ctxMenu: { x: number; y: number; tab: Tab } | null = null;
  let renaming: Tab | null = null;
  let renameValue = "";

  // Tick periodically so idle detection re-evaluates
  let nowTick = Date.now();
  setInterval(() => (nowTick = Date.now()), 1000);

  const SPINNER_FRAMES = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"];
  let spinnerIdx = 0;
  let spinnerFrame = SPINNER_FRAMES[0];
  setInterval(() => {
    spinnerIdx = (spinnerIdx + 1) % SPINNER_FRAMES.length;
    spinnerFrame = SPINNER_FRAMES[spinnerIdx];
  }, 80);

  const IDLE_AFTER_MS = 2500;

  function tabState(tab: Tab): "working" | "idle" | "unknown" {
    if (!tab.lastActive) return "unknown";
    return nowTick - tab.lastActive < IDLE_AFTER_MS ? "working" : "idle";
  }

  function fmtDuration(ms: number): string {
    const s = Math.max(0, Math.floor(ms / 1000));
    if (s < 60) return `${s}s`;
    const m = Math.floor(s / 60);
    if (m < 60) return `${m}m ${s % 60}s`;
    const h = Math.floor(m / 60);
    return `${h}h ${m % 60}m`;
  }

  function tooltip(tab: Tab): string {
    const st = tabState(tab);
    if (st === "unknown") return tab.title;
    if (st === "idle") {
      const idleSince = (tab.lastActive || nowTick) + IDLE_AFTER_MS;
      return `${tab.title}\n쉬는 중 · ${fmtDuration(nowTick - idleSince)}`;
    }
    const burstStart = tab.stateChangedAt || tab.lastActive || nowTick;
    return `${tab.title}\n작업 중 · ${fmtDuration(nowTick - burstStart)}`;
  }

  function openContext(e: MouseEvent, tab: Tab) {
    e.preventDefault();
    e.stopPropagation();
    ctxMenu = null;
    const x = e.clientX;
    const y = e.clientY;
    setTimeout(() => (ctxMenu = { x, y, tab }), 0);
  }

  function togglePinTab(tab: Tab) {
    tabs.update((arr) => arr.map((t) => (t.id === tab.id ? { ...t, pinned: !t.pinned } : t)));
  }

  function buildMenu(tab: Tab) {
    const items: any[] = [
      {
        label: tab.pinned ? "unpin" : "pin",
        action: () => togglePinTab(tab),
      },
      {
        label: "rename",
        action: () => {
          renameValue = tab.title;
          renaming = tab;
        },
        key: "F2",
      },
      { label: "close", action: () => closeTab(new MouseEvent("click"), tab.id), key: "Ctrl+W" },
    ];
    if (tab.sessionId) {
      items.push({
        label: "delete session",
        action: () => deleteUnderlying(tab),
        danger: true,
      });
    }
    return items;
  }

  async function deleteUnderlying(tab: Tab) {
    try {
      await KillPty(tab.id);
    } catch {}
    if (tab.sessionId) {
      try {
        await DeleteSession(tab.sessionId);
      } catch (e) {
        console.error("delete session:", e);
      }
    }
    tabs.update((arr) => arr.filter((t) => t.id !== tab.id));
  }

  async function confirmRename() {
    if (!renaming) return;
    const newTitle = renameValue.trim();
    if (newTitle) {
      // Update tab title locally
      const t = renaming;
      tabs.update((arr) => arr.map((x) => (x.id === t.id ? { ...x, title: newTitle } : x)));
      // Also persist alias to session if attached
      if (renaming.sessionId) {
        try {
          await RenameAlias(renaming.sessionId, newTitle);
        } catch (e) {
          console.error("rename alias:", e);
        }
      }
    }
    renaming = null;
  }

  function selectTab(id: string) {
    activeTabId.set(id);
  }

  async function closeTab(e: MouseEvent, id: string) {
    e.stopPropagation();
    const tab = $tabs.find((t) => t.id === id);
    try {
      await KillPty(id);
    } catch (err) {
      console.error("kill pty:", err);
    }
    tabs.update((arr) => arr.filter((t) => t.id !== id));
    if (tab?.sessionId) {
      GenerateRecap(tab.sessionId, true).catch((e) => console.warn("recap:", e));
    }
  }

  let draggedId: string | null = null;

  function onDragStart(e: DragEvent, id: string) {
    draggedId = id;
    if (e.dataTransfer) {
      e.dataTransfer.effectAllowed = "move";
    }
  }

  function onDragOver(e: DragEvent) {
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
  }

  function onDrop(e: DragEvent, targetId: string) {
    e.preventDefault();
    if (!draggedId || draggedId === targetId) return;
    tabs.update((arr) => {
      const src = arr.findIndex((t) => t.id === draggedId);
      const dst = arr.findIndex((t) => t.id === targetId);
      if (src < 0 || dst < 0) return arr;
      const next = arr.slice();
      const [moved] = next.splice(src, 1);
      next.splice(dst, 0, moved);
      return next;
    });
    draggedId = null;
  }

  async function closeAll() {
    const list = $tabs.slice();
    const toKill = list.filter((t) => !t.pinned);
    if (!toKill.length) return;
    for (const t of toKill) {
      try {
        await KillPty(t.id);
      } catch (err) {
        console.error("kill pty:", err);
      }
      if (t.sessionId) {
        GenerateRecap(t.sessionId, true).catch((e) => console.warn("recap:", e));
      }
    }
    const killSet = new Set(toKill.map((t) => t.id));
    tabs.update((arr) => arr.filter((t) => !killSet.has(t.id)));
  }
</script>

<div class="sidebar">
  <div class="header">
    <span>SESSIONS · {$tabs.length}</span>
    <div class="header-actions">
      {#if $tabs.length > 0}
        {@const unpinnedCount = $tabs.filter((t) => !t.pinned).length}
        <button class="close-all" on:click={closeAll} title={`close ${unpinnedCount} unpinned tabs (pinned kept)`} disabled={unpinnedCount === 0}>✕ {unpinnedCount}</button>
      {/if}
    </div>
  </div>
  {#if $tabs.length === 0}
    <div class="empty">no open sessions</div>
  {:else}
    {#each [...$tabs].sort((a, b) => (b.pinned ? 1 : 0) - (a.pinned ? 1 : 0)) as tab, i (tab.id)}
      {@const st = tabState(tab)}
      <button
        class="tab"
        class:active={$activeTabId === tab.id}
        class:codex={tab.provider === "codex"}
        class:dragging={draggedId === tab.id}
        class:working={st === "working"}
        class:idle={st === "idle"}
        class:pinned-tab={tab.pinned}
        draggable="true"
        on:dragstart={(e) => onDragStart(e, tab.id)}
        on:dragover={onDragOver}
        on:drop={(e) => onDrop(e, tab.id)}
        on:click={() => selectTab(tab.id)}
        on:contextmenu={(e) => openContext(e, tab)}
        on:dblclick={() => { renameValue = tab.title; renaming = tab; }}
        title={tooltip(tab)}
      >
        <span class="num">{tab.pinned ? "★" : String(i + 1).padStart(2, "0")}</span>
        {#if $altHeld && i < 9}
          <span class="hotkey">{i + 1}</span>
        {/if}
        <span class="state-dot" class:working={st === "working"} class:idle={st === "idle"}>{st === "working" ? spinnerFrame : ""}</span>
        <span class="icon"><ProviderIcon provider={tab.provider || "claude"} size={12} /></span>
        <span class="title">{tab.title}</span>
        <span class="close" on:click={(e) => closeTab(e, tab.id)}>×</span>
      </button>
    {/each}
  {/if}
</div>

{#if ctxMenu}
  <ContextMenu
    x={ctxMenu.x}
    y={ctxMenu.y}
    items={buildMenu(ctxMenu.tab)}
    onClose={() => (ctxMenu = null)}
  />
{/if}

{#if renaming}
  <PromptModal
    title={`Rename tab`}
    bind:value={renameValue}
    placeholder="new name"
    confirmLabel="save"
    danger={false}
    onConfirm={confirmRename}
    onCancel={() => (renaming = null)}
  />
{/if}

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow-y: auto;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px;
    font-size: var(--ui-fs-xs);
    color: var(--fg-mute);
    letter-spacing: 1px;
    border-bottom: 1px solid var(--border);
  }

  .close-all {
    color: var(--fg-mute);
    padding: 1px 5px;
    border: 1px solid var(--border);
    border-radius: 2px;
    font-size: calc(var(--ui-fs-xs) - 1px);
  }

  .close-all:hover {
    color: var(--accent-action);
    border-color: var(--accent-action);
  }

  .close-all:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .header-actions {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .hide-side {
    color: var(--fg-mute);
    padding: 1px 5px;
    border: 1px solid var(--border);
    border-radius: 2px;
    font-size: calc(var(--ui-fs-xs) - 1px);
    background: none;
    cursor: pointer;
  }

  .hide-side:hover {
    color: var(--fg);
    border-color: var(--fg);
  }

  .empty {
    padding: 14px 10px;
    color: var(--fg-mute);
    font-size: var(--ui-fs-sm);
    text-align: center;
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 5px 10px;
    text-align: left;
    border-left: 2px solid transparent;
    color: var(--fg-mute);
    opacity: 0.55;
    font-size: var(--ui-fs);
    transition: opacity 0.12s ease, color 0.12s ease, background 0.12s ease;
  }

  /* Visual count helper: extra gap after every 5th tab so it's easier
     to count at a glance. */
  .tab:nth-of-type(5n+5) {
    margin-bottom: 6px;
  }

  .tab:hover {
    background: var(--bg-hover);
    color: var(--fg);
    opacity: 0.85;
  }

  .tab.dragging {
    opacity: 0.4;
  }

  .tab.active {
    background: var(--bg-hover);
    color: var(--fg);
    opacity: 1;
    border-left-color: var(--accent-claude, var(--fg));
    border-left-width: 3px;
    padding-left: 9px;
    box-shadow: inset 0 0 12px rgba(0, 255, 102, 0.12), inset 3px 0 0 var(--accent-claude, var(--fg));
    font-weight: 600;
  }

  .tab.active .title {
    color: var(--fg);
    text-shadow: 0 0 6px rgba(0, 255, 102, 0.25);
  }

  .tab.active .num {
    color: var(--fg);
  }

  .tab.active.codex {
    border-left-color: var(--accent-codex);
    box-shadow: inset 0 0 12px rgba(255, 255, 255, 0.06), inset 3px 0 0 var(--accent-codex);
  }

  .num {
    color: var(--fg-mute);
    font-size: var(--ui-fs-xs);
    width: 16px;
    flex-shrink: 0;
    text-align: center;
  }

  .tab.pinned-tab {
    border-left-color: var(--accent-pinned);
    background: rgba(255, 200, 0, 0.05);
  }

  .tab.pinned-tab .num {
    color: var(--accent-pinned);
    font-size: var(--ui-fs);
    text-shadow: 0 0 4px var(--accent-pinned);
  }

  .hotkey {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 14px;
    height: 14px;
    padding: 0 3px;
    border: 1px solid var(--accent-pinned);
    border-radius: 2px;
    color: var(--accent-pinned);
    font-size: 9px;
    line-height: 1;
    background: rgba(255, 214, 10, 0.08);
    text-shadow: 0 0 3px var(--accent-pinned);
  }

  .state-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    flex-shrink: 0;
    background: var(--fg-mute);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    line-height: 1;
    color: var(--fg-mute);
  }

  .state-dot.working {
    background: transparent;
    color: var(--accent-pinned);
    text-shadow: 0 0 4px var(--accent-pinned);
  }

  .state-dot.idle {
    background: var(--fg-mute);
  }

  .icon {
    flex: 0 0 auto;
    color: var(--accent-claude);
    display: flex;
  }

  .tab.codex .icon {
    color: var(--accent-codex);
  }

  .title {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .close {
    flex: 0 0 auto;
    opacity: 0;
    color: var(--fg-mute);
    padding: 0 4px;
    border-radius: 2px;
  }

  .tab:hover .close {
    opacity: 1;
  }

  .close:hover {
    background: var(--accent-action);
    color: var(--bg);
  }
</style>
