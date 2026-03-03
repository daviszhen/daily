export type User = {
  id: string;
  name: string;
  avatar: string;
  role: string;
};

export type MessageType = 'text' | 'system' | 'summary_confirm';

export type MessageMetadata = {
  summary?: string;
  risks?: string[];
  isSupplement?: boolean;
  supplementDate?: string;
  downloadUrl?: string;
  downloadTitle?: string;
  mode?: string;
  confirmed?: boolean;
  dismissed?: boolean;
  edited?: boolean;
  thinkingSteps?: string[];
  thinkingDone?: boolean;
  thinkingElapsed?: number;
  thinkingCollapsed?: boolean;
};

export type Message = {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  type?: MessageType;
  timestamp: Date;
  metadata?: MessageMetadata;
};

export type DailyReport = {
  id: string;
  userId: string;
  userName: string;
  userAvatar: string;
  content: string;
  date: string;
  tags: string[];
  risks?: string[];
  status: 'draft' | 'submitted';
  timestamp: Date;
};

export type ViewMode = 'chat' | 'feed' | 'stats';
