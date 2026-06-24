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
}

export interface Tab {
  id: string;
  title: string;
  sessionId?: string;
  provider?: string;
}

export type SidebarMode = "tabs" | "browser";
