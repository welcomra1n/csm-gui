import { writable } from "svelte/store";
import type { Session, Tab, SidebarMode } from "./types";

export const tabs = writable<Tab[]>([]);
export const activeTabId = writable<string | null>(null);
export const sessions = writable<Session[]>([]);
export const sidebarMode = writable<SidebarMode>("tabs");
export const statusText = writable<string>("");
export const selectedSessionId = writable<string | null>(null);

const savedZoom = parseFloat(localStorage.getItem("csm-fontsize") || "13");
export const fontSize = writable<number>(isNaN(savedZoom) ? 13 : savedZoom);
fontSize.subscribe((v) => localStorage.setItem("csm-fontsize", String(v)));

let tabCounter = 0;
export function nextTabId(): string {
  tabCounter++;
  return `tab-${Date.now()}-${tabCounter}`;
}
