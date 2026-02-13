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
  options?: { mode?: string | null; date?: string | null },
  callbacks?: StreamCallbacks,
): Promise<Message> {
  const { mode, date } = options || {};
  const { onToken, onResult, onThinking } = callbacks || {};

  const lowerText = text.toLowerCase().trim();
  const isConfirmation = lowerText === '确认' || lowerText === '是' || lowerText === 'ok' ||
    lowerText.includes('确认提交') || lowerText.includes('confirm');

  // confirm 走非流式
  if (isConfirmation) {
    const res = await apiFetch('/api/chat', {
      method: 'POST',
      body: JSON.stringify({ action: 'confirm' }),
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

// ============ Feed ============

export async function getTeamReports(): Promise<any[]> {
  return [];
}
