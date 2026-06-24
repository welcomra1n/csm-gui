import { writable } from "svelte/store";
import type { Session, Tab, SidebarMode } from "./types";

export const tabs = writable<Tab[]>([]);
export const activeTabId = writable<string | null>(null);

// Persist tab metadata via backend metadata.json so it survives app updates
let saveTimer: ReturnType<typeof setTimeout> | null = null;
tabs.subscribe((arr) => {
  if (saveTimer) clearTimeout(saveTimer);
  saveTimer = setTimeout(async () => {
    try {
      const snap = arr.map((t) => ({
        sessionId: t.sessionId || "",
        title: t.title,
        provider: t.provider || "claude",
        pinned: !!t.pinned,
      }));
      const mod = await import("../../wailsjs/go/main/App.js");
      await mod.SaveOpenTabs(snap as any);
    } catch (e) {
      console.warn("SaveOpenTabs:", e);
    }
  }, 250);
});

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

export const progressActive = writable<number>(0); // counter; >0 = busy
export function startProgress() { progressActive.update((n) => n + 1); }
export function endProgress() { progressActive.update((n) => Math.max(0, n - 1)); }

let tabCounter = 0;
export function nextTabId(): string {
  tabCounter++;
  return `tab-${Date.now()}-${tabCounter}`;
}
