export interface Subagent {
  toolUseId: string;
  subagentType: string;
  description: string;
  completed: boolean;
}

export interface Session {
  id: string;
  provider: string;
  projectName: string;
  projectDir: string;
  cwd: string;
  filePath: string;
  modTime: string;
  firstUserMsg: string;
  lastUserMsg: string;
  messageCount: number;
  pinned: boolean;
  active: boolean;
  folder?: string;
  alias?: string;
  tags?: string[];
  gitBranch?: string;
  subagents?: Subagent[];
}

export interface Tab {
  id: string;
  title: string;
  sessionId?: string;
  provider?: string;
  lastActive?: number;
  stateChangedAt?: number;
  state?: "working" | "idle";
  pinned?: boolean;
}

export type SidebarMode = "tabs" | "browser";
