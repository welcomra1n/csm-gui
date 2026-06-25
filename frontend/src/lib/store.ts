import { writable } from "svelte/store";
import type { Session, Tab, SidebarMode } from "./types";

export const tabs = writable<Tab[]>([]);
export const activeTabId = writable<string | null>(null);

// Persist tab metadata via backend metadata.json so it survives app updates.
// Save is BLOCKED until enableTabSave() is called (after restoreTabs finishes),
// otherwise the initial empty [] from store init races against loadSavedTabs
// and wipes the saved pin state on every boot.
let saveTimer: ReturnType<typeof setTimeout> | null = null;
let lastSnapshot: any[] = [];
let saveEnabled = false;
export function enableTabSave() {
  saveEnabled = true;
}
async function flushSave() {
  if (!saveEnabled) return;
  if (saveTimer) {
    clearTimeout(saveTimer);
    saveTimer = null;
  }
  try {
    const mod = await import("../../wailsjs/go/main/App.js");
    await mod.SaveOpenTabs(lastSnapshot as any);
  } catch (e) {
    console.warn("SaveOpenTabs:", e);
  }
}
tabs.subscribe((arr) => {
  lastSnapshot = arr.map((t) => ({
    sessionId: t.sessionId || "",
    title: t.title,
    provider: t.provider || "claude",
    pinned: !!t.pinned,
  }));
  if (!saveEnabled) return;
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = setTimeout(() => {
    saveTimer = null;
    void flushSave();
  }, 60);
});
if (typeof window !== "undefined") {
  window.addEventListener("beforeunload", () => { void flushSave(); });
  window.addEventListener("pagehide", () => { void flushSave(); });
}

export async function loadSavedTabs(): Promise<{ sessionId?: string; title: string; provider?: string; pinned?: boolean }[]> {
  try {
    const mod = await import("../../wailsjs/go/main/App.js");
    const list = await mod.LoadOpenTabs();
    return (list || []).map((t: any) => ({
      sessionId: t.sessionId || undefined,
      title: t.title,
      provider: t.provider,
      pinned: t.pinned,
    }));
  } catch {
    return [];
  }
}
export const sessions = writable<Session[]>([]);
export const sidebarMode = writable<SidebarMode>("tabs");
export const statusText = writable<string>("");
export const selectedSessionId = writable<string | null>(null);

const savedZoom = parseFloat(localStorage.getItem("csm-fontsize") || "13");
export const fontSize = writable<number>(isNaN(savedZoom) ? 13 : savedZoom);
fontSize.subscribe((v) => localStorage.setItem("csm-fontsize", String(v)));

export const focusSearch = writable<number>(0);

const savedLeftW = parseInt(localStorage.getItem("csm-left-w") || "200", 10);
const savedRightW = parseInt(localStorage.getItem("csm-right-w") || "260", 10);
export const leftWidth = writable<number>(isNaN(savedLeftW) ? 200 : savedLeftW);
export const rightWidth = writable<number>(isNaN(savedRightW) ? 260 : savedRightW);
leftWidth.subscribe((v) => localStorage.setItem("csm-left-w", String(v)));
rightWidth.subscribe((v) => localStorage.setItem("csm-right-w", String(v)));

const savedPreviewOpen = localStorage.getItem("csm-preview-open");
export const previewOpen = writable<boolean>(savedPreviewOpen === null ? true : savedPreviewOpen === "1");
previewOpen.subscribe((v) => localStorage.setItem("csm-preview-open", v ? "1" : "0"));

const savedRightOpen = localStorage.getItem("csm-right-open");
export const rightOpen = writable<boolean>(savedRightOpen === null ? true : savedRightOpen === "1");
rightOpen.subscribe((v) => localStorage.setItem("csm-right-open", v ? "1" : "0"));

const savedLeftOpen = localStorage.getItem("csm-left-open");
export const leftOpen = writable<boolean>(savedLeftOpen === null ? true : savedLeftOpen === "1");
leftOpen.subscribe((v) => localStorage.setItem("csm-left-open", v ? "1" : "0"));

// Alt-held indicator (for showing tab hotkey badges 1..9)
export const altHeld = writable<boolean>(false);

export const progressActive = writable<number>(0); // counter; >0 = busy
export function startProgress() { progressActive.update((n) => n + 1); }
export function endProgress() { progressActive.update((n) => Math.max(0, n - 1)); }

let tabCounter = 0;
export function nextTabId(): string {
  tabCounter++;
  return `tab-${Date.now()}-${tabCounter}`;
}
