import { Message, User } from '../types';

export const MO_LOGO = '/mo-logo.png';

// ============ Auth ============

let _token: string | null = localStorage.getItem('token');
let _user: User | null = JSON.parse(localStorage.getItem('user') || 'null');

async function apiFetch(path: string, options: RequestInit = {}): Promise<Response> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> || {}),
  };
  if (_token) headers['Authorization'] = `Bearer ${_token}`;

  const res = await fetch(path, { ...options, headers });
  if (res.status === 401) {
    _token = null;
    _user = null;
    localStorage.removeItem('token');
    localStorage.removeItem('user');
    window.location.reload();
  }
  const newToken = res.headers.get('X-New-Token');
  if (newToken) {
    _token = newToken;
    localStorage.setItem('token', newToken);
  }
  return res;
}

export async function login(username: string, password: string): Promise<User> {
  const res = await fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) throw new Error('登录失败');
  const data = await res.json();
  _token = data.token;
  _user = data.user;
  localStorage.setItem('token', data.token);
  localStorage.setItem('user', JSON.stringify(data.user));
  return data.user;
}

export function logout(): void {
  _token = null;
  _user = null;
  localStorage.removeItem('token');
  localStorage.removeItem('user');
}

export function isLoggedIn(): boolean {
  return !!_token;
}

export function getCurrentUser(): User {
  return _user || { id: '', name: '未登录', avatar: '', role: '' };
}

// ============ Chat ============

interface StreamCallbacks {
  onToken?: (token: string) => void;
  onResult?: (data: any) => void;
  onThinking?: (text: string) => void;
}

export async function processUserMessage(
  text: string,
  history: Message[],
  options?: { mode?: string | null; date?: string | null; sessionId?: number | null },
  callbacks?: StreamCallbacks,
): Promise<Message> {
  const { mode, date, sessionId } = options || {};
  const { onToken, onResult, onThinking } = callbacks || {};

  const lowerText = text.toLowerCase().trim();
  const isConfirmation = lowerText === '确认' || lowerText === '是' || lowerText === 'ok' ||
    lowerText.includes('确认提交') || lowerText.includes('confirm');

  // confirm 走非流式
  if (isConfirmation) {
    const res = await apiFetch('/api/chat', {
      method: 'POST',
      body: JSON.stringify({ action: 'confirm', session_id: sessionId }),
    });
    const data = await res.json();
    return {
      id: Date.now().toString(), role: 'assistant', content: data.content,
      type: data.type || 'text', timestamp: new Date(), metadata: data.metadata,
    };
  }

  // 所有 AI 模式走 SSE
  const body: Record<string, any> = { text };
  if (mode) body.mode = mode;
  if (date) body.date = date;
  if (sessionId) body.session_id = sessionId;

  // 发送最近对话历史（最多 10 条）
  // 补填模式：只发同日期的消息对，不同日期的工作内容互不干扰
  // 汇报模式：截断到最后一次提交之后，避免同会话多次提交串内容
  let filtered = history.filter(m => m.role === 'user' || m.role === 'assistant');
  if (mode === 'supplement' && date) {
    const pairs: Message[] = [];
    for (let i = 0; i < filtered.length; i++) {
      const m = filtered[i];
      if (m.role === 'user' && m.metadata?.supplementDate === date) {
        pairs.push(m);
        if (i + 1 < filtered.length && filtered[i + 1].role === 'assistant') {
          pairs.push(filtered[i + 1]);
        }
      }
    }
    filtered = pairs;
  } else if (mode === 'report') {
    const lastSubmitIdx = filtered.map((m, i) => m.role === 'assistant' && m.content.includes('已提交') ? i : -1).filter(i => i >= 0).pop();
    if (lastSubmitIdx !== undefined) filtered = filtered.slice(lastSubmitIdx + 1);
  }
  const recent = filtered.slice(-10);
  if (recent.length > 0) {
    body.history = recent.map(m => ({ role: m.role, content: m.content }));
  }

  const res = await apiFetch('/api/chat/stream', {
    method: 'POST',
    body: JSON.stringify(body),
  });

  let content = '';
  let metadata: any = undefined;
  let resultData: any = undefined;
  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';

    let currentEvent = '';
    for (const line of lines) {
      if (line.startsWith('event: ')) {
        currentEvent = line.slice(7);
      } else if (line.startsWith('data: ')) {
        try {
          const parsed = JSON.parse(line.slice(6));
          if (currentEvent === 'token' && parsed.token) {
            content += parsed.token;
            onToken?.(parsed.token);
          } else if (currentEvent === 'result') {
            resultData = parsed;
            onResult?.(parsed);
          } else if (currentEvent === 'meta') {
            metadata = parsed;
          } else if (currentEvent === 'thinking' && parsed.text) {
            onThinking?.(parsed.text);
          }
        } catch { /* ignore malformed SSE data */ }
      }
    }
  }

  if (resultData) {
    return {
      id: Date.now().toString(), role: 'assistant',
      content: '为您总结工作内容如下，请确认是否提交：',
      type: 'summary_confirm', timestamp: new Date(),
      metadata: resultData,
    };
  }

  return {
    id: Date.now().toString(), role: 'assistant', content,
    type: 'text', timestamp: new Date(), metadata,
  };
}

// ============ Sessions ============

export type SessionInfo = { id: number; title: string; created_at: number; updated_at: number };

export async function createSession(title: string): Promise<SessionInfo> {
  const res = await apiFetch('/api/sessions', { method: 'POST', body: JSON.stringify({ title }) });
  return res.json();
}

export async function listSessions(): Promise<SessionInfo[]> {
  const res = await apiFetch('/api/sessions');
  const data = await res.json();
  return Array.isArray(data) ? data : [];
}

export async function deleteSession(id: number): Promise<void> {
  await apiFetch(`/api/sessions/${id}`, { method: 'DELETE' });
}

export async function loadSessionMessages(sessionId: number): Promise<Message[]> {
  const res = await apiFetch(`/api/sessions/${sessionId}/messages`);
  const raw: any[] = await res.json();
  const messages: Message[] = [];
  for (const m of raw) {
    // 过滤 Data Asking 内部消息（system:agent:nl2sql 等）
    if (m.role && m.role.startsWith('system:')) continue;
    // 过滤 Data Asking 自动存的 user 消息（带"注：提问者"标记，与我们存的原始消息重复）
    if (m.role === 'user' && m.content && m.content.includes('（注：提问者是')) continue;
    const config = m.config ? tryParse(m.config) : {};
    if (m.role === 'user') {
      messages.push({ id: String(m.id), role: 'user', content: m.content, timestamp: new Date(m.created_at * 1000) });
    } else {
      messages.push({
        id: String(m.id), role: 'assistant', content: m.content || m.response || '',
        type: config.type || 'text', timestamp: new Date(m.created_at * 1000),
        metadata: { ...config, thinkingDone: true },
      });
    }
  }
  return messages;
}

function tryParse(s: string): any { try { return JSON.parse(s); } catch { return {}; } }

// ============ Import ============

export interface PreviewEntry { date: string; name: string; content: string }
export interface PreviewResult { token: string; entries: PreviewEntry[]; unmatched_members: string[] }
export interface ConfirmResult { imported: number; merged: number; skipped: number; skipped_members: string[]; total: number }

export async function previewImport(file: File): Promise<PreviewResult> {
  const form = new FormData();
  form.append('file', file);
  const res = await fetch('/api/import/preview', {
    method: 'POST',
    headers: _token ? { 'Authorization': `Bearer ${_token}` } : {},
    body: form,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: '解析失败' }));
    throw new Error(err.error || '解析失败');
  }
  return res.json();
}

export async function confirmImport(token: string): Promise<ConfirmResult> {
  const res = await fetch('/api/import/confirm', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...(_token ? { 'Authorization': `Bearer ${_token}` } : {}) },
    body: JSON.stringify({ token }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: '导入失败' }));
    throw new Error(err.error || '导入失败');
  }
  return res.json();
}

// ============ Feed ============

export async function getTeamReports(): Promise<any[]> {
  return [];
}
