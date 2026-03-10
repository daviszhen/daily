import React, { useState, useRef, useEffect, useMemo } from 'react';
import { Send, Loader2, Sparkles, FileText, Search, Calendar, FileDown, X, ChevronDown, ChevronRight, Brain, Clock } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Message, User } from '../types';
import { processUserMessage, MO_LOGO, createSession, loadSessionMessages } from '../services/apiService';

type ChatMode = 'report' | 'query' | 'summary' | 'supplement' | null;

interface ChatInterfaceProps {
  user: User;
  onReportSubmitted: () => void;
  sessionId: number | null;
  onSessionCreated: (id: number) => void;
  supplementDate?: string | null;
  onSupplementConsumed?: () => void;
}

const mdComponents: Record<string, React.FC<any>> = {
  h1: ({ children }) => <h2 className="font-bold text-lg mt-3 mb-1" style={{ color: 'var(--text-primary)' }}>{children}</h2>,
  h2: ({ children }) => <h3 className="font-semibold text-base mt-3 mb-1" style={{ color: 'var(--text-primary)' }}>{children}</h3>,
  h3: ({ children }) => <h4 className="font-semibold text-sm mt-3 mb-1" style={{ color: 'var(--text-primary)' }}>{children}</h4>,
  p: ({ children }) => <p className="my-1">{children}</p>,
  ul: ({ children }) => <ul className="ml-1 space-y-0.5">{children}</ul>,
  ol: ({ children }) => <ol className="ml-1 space-y-0.5 list-decimal list-inside">{children}</ol>,
  li: ({ children }) => (
    <div className="flex items-start gap-2 ml-1 my-0.5">
      <span className="mt-1.5 text-[6px]" style={{ color: 'var(--accent-dot)' }}>●</span>
      <span>{children}</span>
    </div>
  ),
  strong: ({ children }) => <strong className="font-semibold" style={{ color: 'var(--text-primary)' }}>{children}</strong>,
  code: ({ className, children }: any) => {
    const isBlock = className?.includes('language-');
    return isBlock
      ? <pre className="rounded p-2 text-xs overflow-x-auto my-2" style={{ background: 'var(--bg-accent)' }}><code>{children}</code></pre>
      : <code className="px-1 py-0.5 rounded text-xs" style={{ background: 'var(--bg-accent)', color: 'var(--text-dim)' }}>{children}</code>;
  },
  table: ({ children }) => (
    <div className="overflow-x-auto my-2">
      <table className="min-w-full text-sm border-collapse">{children}</table>
    </div>
  ),
  thead: ({ children }) => <thead style={{ background: 'var(--bg-sidebar)' }}>{children}</thead>,
  th: ({ children }) => <th className="px-3 py-1.5 text-left font-medium" style={{ border: '1px solid var(--border)', color: 'var(--text-body)' }}>{children}</th>,
  td: ({ children }) => <td className="px-3 py-1.5" style={{ border: '1px solid var(--border)', color: 'var(--text-dim)' }}>{children}</td>,
};

function MarkdownContent({ text }: { text: string }): React.ReactElement {
  return <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>{text}</ReactMarkdown>;
}

const MODE_LABELS: Record<string, { icon: string; text: string }> = {
  report:     { icon: '📝', text: '汇报' },
  supplement: { icon: '📝', text: '补报' },
  query:      { icon: '🔍', text: '查询' },
  summary:    { icon: '📊', text: '周报' },
};

function formatElapsed(ms: number): string {
  const sec = Math.floor(ms / 1000);
  if (sec < 60) return `${sec}秒`;
  return `${Math.floor(sec / 60)}分${sec % 60}秒`;
}

export function ChatInterface({ user, onReportSubmitted, sessionId, onSessionCreated, supplementDate, onSupplementConsumed }: ChatInterfaceProps): React.ReactElement {
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [thinkingSteps, setThinkingSteps] = useState<string[]>([]);
  const [thinkingCollapsed, setThinkingCollapsed] = useState(false);
  const [thinkingStartTime, setThinkingStartTime] = useState<number | null>(null);
  const [thinkingElapsed, setThinkingElapsed] = useState(0);
  const [activeMode, setActiveMode] = useState<ChatMode>(null);
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const dateInputRef = useRef<HTMLInputElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const activeSessionRef = useRef<number | null>(sessionId);
  const msgCacheRef = useRef<Map<number | null, Message[]>>(new Map());
  const justCreatedRef = useRef(false);

  const [messages, setMessages] = useState<Message[]>([{
    id: 'welcome', role: 'assistant',
    content: `你好，${user.name}。我是你的 AI 日报助手。请选择下方的功能按钮。`,
    timestamp: new Date(),
  }]);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, thinkingSteps]);

  useEffect(() => {
    const el = inputRef.current;
    if (!el) return;
    el.style.height = 'auto';
    const maxH = 128;
    if (el.scrollHeight > maxH) {
      el.style.height = maxH + 'px';
      el.style.overflowY = 'auto';
    } else {
      el.style.height = el.scrollHeight + 'px';
      el.style.overflowY = 'hidden';
    }
  }, [input]);

  // Thinking elapsed timer
  useEffect(() => {
    if (!thinkingStartTime || !isLoading) return;
    const interval = setInterval(() => setThinkingElapsed(Date.now() - thinkingStartTime), 1000);
    return () => clearInterval(interval);
  }, [thinkingStartTime, isLoading]);

  useEffect(() => {
    if (activeMode !== 'supplement') setSelectedDate(null);
  }, [activeMode]);

  // Calendar supplement: auto-activate supplement mode with pre-selected date
  useEffect(() => {
    if (supplementDate === '__today__') {
      setActiveMode('report');
      setSelectedDate(null);
      onSupplementConsumed?.();
    } else if (supplementDate) {
      setActiveMode('supplement');
      setSelectedDate(supplementDate);
      onSupplementConsumed?.();
    }
  }, [supplementDate]);

  // 切换会话时缓存当前消息，恢复目标会话消息
  useEffect(() => {
    const prevSession = activeSessionRef.current;
    activeSessionRef.current = sessionId;

    // 从 null（新建）切到有 ID 的 session：迁移缓存，不重置消息
    if (prevSession === null && sessionId !== null && justCreatedRef.current) {
      justCreatedRef.current = false;
      setMessages(prev => {
        msgCacheRef.current.set(sessionId, prev);
        return prev;
      });
      return;
    }

    // 缓存离开的 session
    if (prevSession !== null) {
      setMessages(prev => { msgCacheRef.current.set(prevSession, prev); return prev; });
    }

    if (!sessionId) {
      if (!supplementDate) setActiveMode(null);
      setIsLoading(false);
      setMessages([{
        id: 'welcome', role: 'assistant',
        content: `你好，${user.name}。我是你的 AI 日报助手。请选择下方的功能按钮。`,
        timestamp: new Date(),
      }]);
      return;
    }

    // 优先用缓存
    const cached = msgCacheRef.current.get(sessionId);
    if (cached && cached.length > 0) {
      setMessages(cached);
      return; // 有缓存就不从 API 加载，避免覆盖进行中的流
    }

    // 缓存没有，从 API 加载
    loadSessionMessages(sessionId).then(msgs => {
      if (activeSessionRef.current !== sessionId) return;
      if (msgs.length > 0) {
        setMessages(msgs);
        msgCacheRef.current.set(sessionId, msgs);
      }
    }).catch(() => {});
  }, [sessionId]);

  function isConfirmation(text: string): boolean {
    const t = text.toLowerCase().trim();
    return t === '确认' || t === '是' || t === 'ok' || t.includes('确认提交') || t.includes('confirm');
  }

  async function handleSend(text: string = input): Promise<void> {
    if (!text.trim() && activeMode !== 'summary') return;
    const sendText = text.trim() || (activeMode === 'summary' ? '生成最近一周的周报' : '');
    if (!sendText) return;
    const isConfirm = ['确认', '是', 'ok', '确认提交', 'confirm'].includes(sendText.toLowerCase());

    // 用户主动发新消息时，自动关闭旧的未操作确认卡片
    if (!isConfirm) {
      setMessages(prev => prev.map(m =>
        m.type === 'summary_confirm' && m.metadata && !m.metadata.confirmed && !m.metadata.dismissed && !m.metadata.edited
          ? { ...m, metadata: { ...m.metadata, dismissed: true } } : m
      ));
    }

    // Ensure session exists (lazy create on first message)
    let sid = sessionId;
    if (!sid) {
      try {
        const sess = await createSession(new Date().toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).replace(/\//g, '-'));
        sid = sess.id;
        justCreatedRef.current = true;
        onSessionCreated(sid);
      } catch (e) {
        console.error('create session failed', e);
      }
    }

    const userMsg: Message = {
      id: Date.now().toString(), role: 'user', content: sendText, timestamp: new Date(),
      metadata: { ...(selectedDate ? { supplementDate: selectedDate } : {}), ...(activeMode ? { mode: activeMode } : {}) },
    };
    setMessages(prev => [...prev.filter(m => m.id !== 'welcome'), userMsg]);
    setInput('');
    setIsLoading(true);
    const sendSessionId = sid; // 记录发送时的 session，回调里检查是否还是当前 session
    const isActive = () => activeSessionRef.current === sendSessionId;

    try {
      if (isConfirmation(sendText)) {
        const response = await processUserMessage(sendText, messages, { mode: activeMode, date: selectedDate, sessionId: sid });
        if (!isActive()) return;
        setMessages(prev => [...prev, response]);
        onReportSubmitted();
        setActiveMode(null);
        setSelectedDate(null);
        return;
      }

      // Streaming: add placeholder message
      const streamId = (Date.now() + 1).toString();
      setMessages(prev => [...prev, { id: streamId, role: 'assistant', content: '', type: 'text', timestamp: new Date(), metadata: { mode: activeMode || undefined } }]);
      setThinkingSteps([]);
      setThinkingStartTime(null);
      setThinkingElapsed(0);

      const thinkStart = Date.now();
      const response = await processUserMessage(sendText, messages, { mode: activeMode, date: selectedDate, sessionId: sid }, {
        onToken(token) {
          if (!isActive()) return;
          // 收到正文 token 后折叠思考过程
          setMessages(prev => prev.map(m => m.id === streamId
            ? { ...m, content: m.content + token, metadata: { ...m.metadata, thinkingCollapsed: true } } : m));
        },
        onResult(data) {
          if (!isActive()) return;
          setMessages(prev => prev.map(m => m.id === streamId
            ? { ...m, content: '为您总结工作内容如下，请确认是否提交：', type: 'summary_confirm' as any, metadata: data }
            : m));
        },
        onThinking(step) {
          if (!isActive()) return;
          setThinkingStartTime(prev => prev ?? Date.now());
          setThinkingSteps(prev => [...prev, step]);
          // 思考中默认展开（thinkingCollapsed: false）
          setMessages(prev => prev.map(m => {
            if (m.id !== streamId) return m;
            const steps = [...(m.metadata?.thinkingSteps || []), step];
            return { ...m, metadata: { ...m.metadata, thinkingSteps: steps, thinkingCollapsed: false, thinkingElapsed: Date.now() - thinkStart } };
          }));
        },
        onModeSwitch(mode) {
          if (!isActive()) return;
          setActiveMode(mode as ChatMode);
          setMessages(prev => prev.map(m => m.id === streamId ? { ...m, metadata: { ...m.metadata, mode } } : m));
        },
      });

      // Update metadata (downloadUrl etc) if not already set by onResult
      if (response.metadata && response.type !== 'summary_confirm') {
        setMessages(prev => prev.map(m => m.id === streamId ? { ...m, metadata: response.metadata } : m));
      }
    } catch (error: any) {
      console.error(error);
    } finally {
      // 流完成后同步缓存（不管是否当前 session，确保切回来能看到）
      if (sendSessionId) {
        setMessages(prev => { msgCacheRef.current.set(sendSessionId, prev); return prev; });
      }
      if (!isActive()) return;
      // Save final elapsed into message metadata
      setMessages(prev => prev.map(m => {
        if (m.metadata?.thinkingSteps?.length > 0 && !m.metadata?.thinkingDone) {
          return { ...m, metadata: { ...m.metadata, thinkingDone: true } };
        }
        return m;
      }));
      setThinkingStartTime(prev => { if (prev) setThinkingElapsed(Date.now() - prev); return null; });
      setIsLoading(false);
    }
  }

  function handleEdit(summary: string, date?: string): void {
    setInput(summary);
    if (date) {
      setActiveMode('supplement');
      setSelectedDate(date);
    } else {
      setActiveMode('report');
    }
    setTimeout(() => inputRef.current?.focus(), 0);
  }

  function toggleMode(mode: ChatMode): void {
    setActiveMode(activeMode === mode ? null : mode);
  }

  function getPlaceholder(): string {
    switch (activeMode) {
      case 'report': return '输入今日完成的工作内容...';
      case 'query': return '输入想查询的同事姓名、项目或关键词...';
      case 'summary': return '输入周报的时间范围或重点关注内容...';
      case 'supplement': return selectedDate ? '输入该日的工作内容...' : '请先选择补填日期...';
      default: return '选择上方功能，或随便聊聊...';
    }
  }

  function getYesterdayDate(): string {
    const d = new Date();
    d.setDate(d.getDate() - 1);
    return d.toISOString().split('T')[0];
  }

  const isEmpty = messages.length <= 1 && messages[0]?.id === 'welcome';

  // Shared input area (used in both layouts)
  const inputArea = (
    <>
      <div className="flex flex-wrap gap-2 mb-3 px-1">
        <ModeButton mode="report" label="汇报今日工作" icon={FileText} active={activeMode === 'report'} disabled={isLoading} onToggle={() => toggleMode('report')} />
        <ModeButton mode="supplement" label="补填往期日报" icon={Calendar} active={activeMode === 'supplement'} disabled={isLoading} onToggle={() => toggleMode('supplement')} />
        <ModeButton mode="summary" label="生成周报总结" icon={Sparkles} active={activeMode === 'summary'} disabled={isLoading} onToggle={() => toggleMode('summary')} />
        <ModeButton mode="query" label="查询团队动态" icon={Search} active={activeMode === 'query'} disabled={isLoading} onToggle={() => toggleMode('query')} />
      </div>

      <div className="relative flex flex-col rounded-2xl shadow-sm transition-colors" style={{ background: 'var(--bg-input)', border: '1px solid var(--border)' }}>
        {activeMode === 'supplement' && !selectedDate && (
          <div className="px-3 pt-3 flex">
            <button type="button" onClick={() => dateInputRef.current?.showPicker()} className="flex items-center gap-1.5 text-xs font-medium px-2.5 py-1.5 rounded-md transition-colors cursor-pointer" style={{ background: 'var(--bg-accent)', color: 'var(--text-dim)', border: '1px solid var(--border)' }}>
              <Calendar size={13} />
              选择补填日期
            </button>
            <input ref={dateInputRef} type="date" max={getYesterdayDate()}
              className="absolute w-0 h-0 opacity-0 pointer-events-none"
              onChange={(e) => e.target.value && setSelectedDate(e.target.value)}
            />
          </div>
        )}
        {activeMode === 'supplement' && selectedDate && (
          <div className="px-3 pt-3 flex">
            <div className="flex items-center text-xs font-medium px-2 py-1 rounded-md animate-fade-in" style={{ background: 'var(--bg-accent)', color: 'var(--text-dim)', border: '1px solid var(--border)' }}>
              <Calendar size={12} className="mr-1.5" />
              <span>补填: {selectedDate}</span>
              <button onClick={() => setSelectedDate(null)} className="ml-2 rounded p-0.5 transition-colors" style={{ color: 'var(--text-secondary)' }}>
                <X size={12} />
              </button>
            </div>
          </div>
        )}

        <div className="flex items-end w-full">
          <textarea
            ref={inputRef} value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && e.shiftKey && !(activeMode === 'supplement' && !selectedDate)) { e.preventDefault(); handleSend(); }
            }}
            placeholder={getPlaceholder()}
            style={{ outline: 'none', boxShadow: 'none', color: 'var(--text-primary)' }}
            className="flex-1 bg-transparent border-none focus:ring-0 focus:outline-none resize-none py-3 px-3 min-h-[48px] max-h-32"
            rows={1}
          />

          <button
            onClick={() => handleSend()}
            disabled={(activeMode !== 'summary' && !input.trim()) || isLoading || (activeMode === 'supplement' && !selectedDate)}
            className="p-2 m-1.5 rounded-xl transition-all flex-shrink-0"
            style={{
              background: (input.trim() || activeMode === 'summary') && !isLoading && !(activeMode === 'supplement' && !selectedDate) ? 'var(--btn-primary)' : 'var(--bg-active)',
              color: (input.trim() || activeMode === 'summary') && !isLoading && !(activeMode === 'supplement' && !selectedDate) ? 'var(--btn-primary-text)' : 'var(--text-muted)',
              cursor: (input.trim() || activeMode === 'summary') && !isLoading && !(activeMode === 'supplement' && !selectedDate) ? 'pointer' : 'not-allowed',
            }}
          >
            <Send size={18} />
          </button>
        </div>
      </div>
      <p className="text-center text-xs mt-2" style={{ color: 'var(--text-muted)' }}>AI 生成内容可能存在误差，重要信息请核对。</p>
    </>
  );

  // Empty state: centered layout
  if (isEmpty) {
    return (
      <div className="flex flex-col items-center justify-center h-full max-w-2xl mx-auto w-full px-4">
        <div className="mb-2">
          <img src={MO_LOGO} alt="MOI" className="w-12 h-12 object-contain" />
        </div>
        <h2 className="text-xl font-semibold mb-1" style={{ color: 'var(--text-primary)' }}>你好，{user.name}</h2>
        <p className="text-sm mb-8" style={{ color: 'var(--text-muted)' }}>选择功能开始，或随便聊聊</p>
        <div className="w-full">{inputArea}</div>
      </div>
    );
  }

  // Chat state: normal layout
  return (
    <div className="flex flex-col h-full max-w-4xl mx-auto w-full">
      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-4 py-6 space-y-6">
        {messages.map((msg) => {
          const isUser = msg.role === 'user';

          return (
            <div key={msg.id} className={`flex ${isUser ? 'justify-end' : 'justify-start'} animate-fade-in`}>
              <div className={`flex max-w-[85%] md:max-w-[70%] ${isUser ? 'flex-row-reverse space-x-reverse' : 'flex-row'} items-start space-x-3`}>
                {!isUser && (
                  <div className="w-8 h-8 rounded-full flex items-center justify-center flex-shrink-0 overflow-hidden" style={{ background: 'var(--bg-input)', border: '1px solid var(--border)' }}>
                    <img src={MO_LOGO} alt="MO" className="w-5 h-5 object-contain" />
                  </div>
                )}

                <div className={`space-y-1 ${isUser ? 'text-right' : 'text-left'}`}>
                  {/* Mode label pill - above bubble */}
                  {isUser && (() => {
                    const ml = MODE_LABELS[msg.metadata?.mode as string];
                    return ml ? (
                      <div className="inline-flex items-center text-xs px-2 py-0.5 rounded-full border"
                        style={{ borderColor: 'var(--border)', color: 'var(--text-dim)', background: 'var(--bg-sidebar)' }}>
                        <span className="mr-1 text-[10px]">{ml.icon}</span>{ml.text}
                      </div>
                    ) : null;
                  })()}
                  {msg.type !== 'summary_confirm' && (
                  <div className={`px-5 py-3 rounded-2xl text-base leading-relaxed shadow-sm ${
                    isUser ? 'rounded-tr-sm' : 'rounded-tl-sm'
                  }`} style={isUser ? { background: 'var(--bg-bubble-user)', color: 'var(--text-primary)' } : { background: 'var(--bg-card)', border: '1px solid var(--border)', color: 'var(--text-body)' }}>
                    {msg.metadata?.supplementDate && (
                      <div className="inline-flex items-center text-xs px-2 py-0.5 rounded mb-2" style={{ background: 'var(--bg-accent)', color: 'var(--text-dim)', border: '1px solid var(--border)' }}>
                        <Calendar size={10} className="mr-1" />
                        补填: {msg.metadata.supplementDate}
                      </div>
                    )}
                    {/* Thinking steps (query mode) - above answer */}
                    {(() => {
                      const steps = msg.metadata?.thinkingSteps as string[] | undefined;
                      if (!steps?.length) return null;
                      const done = msg.metadata?.thinkingDone as boolean | undefined;
                      const elapsed = (isLoading && !done) ? thinkingElapsed : msg.metadata?.thinkingElapsed as number | undefined;
                      const collapsed = msg.metadata?.thinkingCollapsed !== false; // 默认折叠
                      const toggleCollapse = () => setMessages(prev => prev.map(m =>
                        m.id === msg.id ? { ...m, metadata: { ...m.metadata, thinkingCollapsed: collapsed ? false : true } } : m
                      ));
                      return (
                        <div className="mb-2">
                          <button
                            onClick={toggleCollapse}
                            className="flex items-center gap-1.5 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}
                          >
                            <Brain size={13} />
                            {collapsed ? <ChevronRight size={13} /> : <ChevronDown size={13} />}
                            思考过程 ({steps.length} 步)
                            {elapsed && elapsed > 0 && (
                              <span className="font-normal flex items-center gap-0.5" style={{ color: 'var(--text-muted)' }}>
                                <Clock size={10} /> {formatElapsed(elapsed)}
                              </span>
                            )}
                            {isLoading && !done && !collapsed && <Loader2 size={11} className="animate-spin ml-1" />}
                          </button>
                          {!collapsed && (
                            <div className="mt-1.5 space-y-0.5 text-xs pl-2.5" style={{ color: 'var(--text-muted)', borderLeft: '2px solid var(--accent-dot)' }}>
                              {steps.map((step, i) => (
                                <p key={i} className="leading-relaxed">{step}</p>
                              ))}
                            </div>
                          )}
                        </div>
                      );
                    })()}
                    {isUser
                      ? <p className="whitespace-pre-wrap">{msg.content}</p>
                      : msg.content === '' && isLoading
                        ? <Loader2 size={16} className="animate-spin" style={{ color: 'var(--text-muted)' }} />
                        : <MarkdownContent text={msg.content} />
                    }
                  </div>
                  )}

                  {/* Summary confirmation card */}
                  {msg.type === 'summary_confirm' && msg.metadata && (
                    <div className="rounded-xl p-4 shadow-sm mt-2 max-w-sm ml-0 mr-auto text-left" style={{ background: 'var(--bg-input)', border: '1px solid var(--border)' }}>
                      <h4 className="text-xs font-semibold uppercase tracking-wide mb-2" style={{ color: 'var(--text-secondary)' }}>
                        {msg.metadata.isSupplement ? '补交预览' : '日报预览'}
                        {msg.metadata.supplementDate && <span className="ml-2 font-normal" style={{ color: 'var(--text-muted)' }}>({msg.metadata.supplementDate})</span>}
                      </h4>
                      <div className="text-sm p-3 rounded-lg mb-3 whitespace-pre-line" style={{ color: 'var(--text-body)', background: 'var(--bg-sidebar)', border: '1px solid var(--border)' }}>
                        {msg.metadata.summary}
                      </div>
                      {msg.metadata.risks?.length > 0 && (
                        <div className="mb-3">
                          <span className="text-xs font-medium text-red-500 bg-red-50 px-2 py-1 rounded">检测到风险</span>
                        </div>
                      )}
                      <div className="flex space-x-2 w-64">
                        {msg.metadata.confirmed ? (
                          <div className="flex-1 text-center text-sm py-2" style={{ color: 'var(--text-muted)' }}>已提交</div>
                        ) : msg.metadata.dismissed ? (
                          <div className="flex-1 text-center text-sm py-2" style={{ color: 'var(--text-muted)' }}>已取消</div>
                        ) : msg.metadata.edited ? (
                          <div className="flex-1 text-center text-sm py-2" style={{ color: 'var(--text-muted)' }}>已编辑</div>
                        ) : (
                          <>
                            <button onClick={() => {
                              setMessages(prev => prev.map(m => m.id === msg.id ? { ...m, metadata: { ...m.metadata, confirmed: true } } : m));
                              handleSend('确认提交');
                            }} className="w-20 text-white text-sm py-2 rounded-lg transition-colors" style={{ background: 'var(--btn-primary)' }}>
                              提交
                            </button>
                            <button onClick={() => {
                              setMessages(prev => prev.map(m => m.id === msg.id ? { ...m, metadata: { ...m.metadata, edited: true } } : m));
                              handleEdit(msg.metadata.summary, msg.metadata.supplementDate);
                            }} className="w-20 text-sm py-2 rounded-lg transition-colors" style={{ background: 'var(--bg-input)', border: '1px solid var(--border)', color: 'var(--text-body)' }}>
                              编辑
                            </button>
                            <button onClick={() => {
                              setMessages(prev => prev.map(m => m.id === msg.id ? { ...m, metadata: { ...m.metadata, dismissed: true } } : m));
                            }} className="w-20 text-sm py-2 rounded-lg transition-colors" style={{ background: 'var(--bg-input)', border: '1px solid var(--border)', color: 'var(--text-secondary)' }}>
                              取消
                            </button>
                          </>
                        )}
                      </div>
                    </div>
                  )}

                  {/* Download card */}
                  {msg.metadata?.downloadUrl && (
                    <div className="rounded-xl p-3 shadow-sm mt-2 max-w-sm ml-0 mr-auto flex items-center cursor-pointer transition-colors group" style={{ background: 'var(--bg-input)', border: '1px solid var(--border)' }}>
                      <div className="w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0 transition-colors" style={{ background: 'var(--bg-accent)', color: 'var(--text-secondary)' }}>
                        <FileDown size={20} />
                      </div>
                      <div className="ml-3 flex-1 min-w-0">
                        <p className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>{msg.metadata.downloadTitle || 'Document.pdf'}</p>
                        <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>点击下载生成的文档</p>
                      </div>
                    </div>
                  )}

                  <span className="text-xs block pt-1" style={{ color: 'var(--accent-dot)' }}>
                    {msg.timestamp.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                  </span>
                </div>
              </div>
            </div>
          );
        })}

        {/* Loading spinner (only when no streaming message exists) */}
        {isLoading && messages[messages.length - 1]?.role !== 'assistant' && (
          <div className="flex items-center space-x-3">
            <div className="w-8 h-8 rounded-full flex items-center justify-center overflow-hidden" style={{ background: 'var(--bg-input)', border: '1px solid var(--border)' }}>
              <img src={MO_LOGO} alt="Loading" className="w-5 h-5 object-contain" />
            </div>
            <div className="px-4 py-2 rounded-2xl rounded-tl-sm" style={{ background: 'var(--bg-accent)' }}>
              <Loader2 size={16} className="animate-spin" style={{ color: 'var(--text-muted)' }} />
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input area */}
      <div className="p-4 border-t" style={{ background: 'var(--bg-page)', borderColor: 'var(--border)' }}>
        <div className="max-w-4xl mx-auto">
          {inputArea}
        </div>
      </div>
    </div>
  );
}

// ============ Sub-components ============

function ModeButton({ label, icon: Icon, active, disabled, onToggle }: {
  mode: ChatMode; label: string; icon: React.ElementType;
  active: boolean; disabled: boolean; onToggle: () => void;
}): React.ReactElement {
  return (
    <button
      onClick={onToggle} disabled={disabled}
      className="flex items-center space-x-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-all shadow-sm whitespace-nowrap"
      style={active
        ? { background: 'var(--btn-primary)', border: '1px solid var(--btn-primary)', color: 'var(--btn-primary-text)' }
        : { background: 'var(--bg-sidebar)', border: '1px solid var(--border)', color: 'var(--text-dim)' }
      }
    >
      <Icon size={12} />
      <span>{label}</span>
    </button>
  );
}
