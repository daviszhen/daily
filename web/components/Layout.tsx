import React, { useState, useEffect } from 'react';
import { LayoutDashboard, MessageSquare, PieChart, CalendarDays, Menu, X, UploadCloud, FileUp, CheckCircle2, LogOut, Trash2, Eye, PlusCircle, MessageCircle, Send, XCircle } from 'lucide-react';
import { ViewMode, User } from '../types';
import { MO_LOGO, logout, SessionInfo, previewImport, confirmImport, PreviewEntry, PreviewResult, ConfirmResult, MemberDecision, getTeams, Team, FeedbackItem, submitFeedback, listFeedback, closeFeedback, deleteFeedback } from '../services/apiService';

const THEMES = [
  { id: 'warm', label: '暖沙', color: '#C8B898' },
  { id: 'light', label: '简白', color: '#D0D0D0' },
  { id: 'dark', label: '暗夜', color: '#3A3A40' },
] as const;

interface LayoutProps {
  children: React.ReactNode;
  currentView: ViewMode;
  onChangeView: (view: ViewMode) => void;
  user: User;
  sessions: SessionInfo[];
  activeSessionId: number | null;
  onNewChat: () => void;
  onSelectSession: (id: number) => void;
  onDeleteSession: (id: number) => void;
}

export function Layout({ children, currentView, onChangeView, user, sessions, activeSessionId, onNewChat, onSelectSession, onDeleteSession }: LayoutProps): React.ReactElement {
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [isImportModalOpen, setIsImportModalOpen] = useState(false);
  const [isThemeOpen, setIsThemeOpen] = useState(false);
  const [currentTheme, setCurrentTheme] = useState(() => localStorage.getItem('theme') || 'warm');
  const [uploadStatus, setUploadStatus] = useState<'idle' | 'uploading' | 'preview' | 'confirming' | 'success' | 'error'>('idle');
  const [previewData, setPreviewData] = useState<PreviewResult | null>(null);
  const [importResult, setImportResult] = useState<ConfirmResult | null>(null);
  const [importError, setImportError] = useState('');
  const [memberDecisions, setMemberDecisions] = useState<Record<string, MemberDecision>>({});
  const [importTeams, setImportTeams] = useState<Team[]>([]);
  const [elapsedTime, setElapsedTime] = useState(0);
  const timerRef = React.useRef<ReturnType<typeof setInterval> | null>(null);
  const [avatarOk, setAvatarOk] = useState(true);
  const [avatarSeed] = useState(() => `${user.name}-${Date.now()}`);
  const avatarUrl = `https://picsum.photos/seed/${encodeURIComponent(avatarSeed)}/80/80`;
  const fileInputRef = React.useRef<HTMLInputElement>(null);
  const menuRef = React.useRef<HTMLDivElement>(null);
  const [isFeedbackOpen, setIsFeedbackOpen] = useState(false);
  const [feedbackList, setFeedbackList] = useState<FeedbackItem[]>([]);
  const [feedbackText, setFeedbackText] = useState('');
  const [feedbackLoading, setFeedbackLoading] = useState(false);

  async function loadFeedback() {
    setFeedbackList(await listFeedback());
  }
  async function handleSubmitFeedback() {
    if (!feedbackText.trim()) return;
    setFeedbackLoading(true);
    await submitFeedback(feedbackText.trim());
    setFeedbackText('');
    await loadFeedback();
    setFeedbackLoading(false);
  }

  useEffect(() => {
    if (!isThemeOpen) return;
    const handler = (e: MouseEvent) => { if (menuRef.current && !menuRef.current.contains(e.target as Node)) setIsThemeOpen(false); };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [isThemeOpen]);

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', currentTheme);
  }, [currentTheme]);

  function switchTheme(id: string) {
    setCurrentTheme(id);
    localStorage.setItem('theme', id);
    setIsThemeOpen(false);
  }

  function handleCloseModal(): void {
    setIsImportModalOpen(false);
    setTimeout(() => { setUploadStatus('idle'); setPreviewData(null); setImportResult(null); setImportError(''); setMemberDecisions({}); setElapsedTime(0); if (timerRef.current) clearInterval(timerRef.current); }, 300);
  }

  async function handleFileUpload(file: File) {
    setUploadStatus('uploading');
    setElapsedTime(0);
    timerRef.current = setInterval(() => setElapsedTime(t => t + 1), 1000);
    try {
      const result = await previewImport(file);
      setPreviewData(result);
      // Init decisions: default to "create" for each unmatched member
      const decisions: Record<string, MemberDecision> = {};
      for (const name of result.unmatched_members) {
        decisions[name] = { action: 'create', name };
      }
      setMemberDecisions(decisions);
      getTeams().then(setImportTeams);
      if (timerRef.current) clearInterval(timerRef.current);
      setUploadStatus('preview');
    } catch (e: any) {
      if (timerRef.current) clearInterval(timerRef.current);
      setImportError(e.message || '解析失败');
      setUploadStatus('error');
    }
  }

  async function handleConfirmImport() {
    if (!previewData?.token) return;
    setUploadStatus('confirming');
    try {
      const result = await confirmImport(previewData.token, Object.keys(memberDecisions).length > 0 ? memberDecisions : undefined);
      setImportResult(result);
      setUploadStatus('success');
    } catch (e: any) {
      setImportError(e.message || '导入失败');
      setUploadStatus('error');
    }
  }

  return (
    <div className="flex h-screen overflow-hidden" style={{ background: 'var(--bg-page)', fontFamily: "'Inter', sans-serif" }}>
      {/* Mobile Header */}
      <div className="md:hidden fixed top-0 w-full h-16 border-b z-50 flex items-center px-4" style={{ background: 'var(--bg-page)', borderColor: 'var(--border)' }}>
        <button onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)} className="p-2" style={{ color: 'var(--text-secondary)' }}>
          {isMobileMenuOpen ? <X size={24} /> : <Menu size={24} />}
        </button>
        <div className="flex-1 flex items-center justify-center space-x-2">
          <div className="w-7 h-7 rounded-lg flex items-center justify-center overflow-hidden">
            <img src={MO_LOGO} alt="MOI" className="w-full h-full object-contain" />
          </div>
          <span className="font-semibold" style={{ color: 'var(--text-primary)' }}>智能日报</span>
        </div>
        <div className="w-10" />
      </div>

      {/* Sidebar */}
      <aside className={`
        fixed md:static inset-y-0 left-0 z-40 w-64 transform transition-transform duration-300 ease-in-out flex flex-col
        ${isMobileMenuOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'}
      `} style={{ background: 'var(--bg-sidebar)' }}>
        <div className="h-16 flex items-center px-6">
          <div className="w-8 h-8 rounded-lg flex items-center justify-center mr-3 overflow-hidden">
            <img src={MO_LOGO} alt="MOI" className="w-full h-full object-contain" />
          </div>
          <span className="font-semibold text-lg tracking-tight" style={{ color: 'var(--text-primary)' }}>智能日报</span>
        </div>

        <div className="flex-1 px-3 py-6 space-y-1 overflow-hidden flex flex-col">
          <div className="text-xs font-semibold uppercase tracking-wider px-3 mb-3" style={{ color: 'var(--text-muted)' }}>菜单</div>
          <NavItem icon={MessageSquare} label="AI 助手" active={currentView === 'chat' && !activeSessionId} onClick={() => { onNewChat(); setIsMobileMenuOpen(false); }} />
          <NavItem icon={CalendarDays} label="我的日历" active={currentView === 'calendar'} onClick={() => { onChangeView('calendar'); setIsMobileMenuOpen(false); }} />
          <NavItem icon={LayoutDashboard} label="团队动态" active={currentView === 'feed'} onClick={() => { onChangeView('feed'); setIsMobileMenuOpen(false); }} />
          <NavItem icon={PieChart} label="数据洞察" active={currentView === 'stats'} onClick={() => { onChangeView('stats'); setIsMobileMenuOpen(false); }} />

          {sessions.length > 0 && (
            <div className="mt-4 flex-1 min-h-0 flex flex-col">
              <div className="text-xs font-semibold uppercase tracking-wider px-3 mb-2" style={{ color: 'var(--text-muted)' }}>对话历史</div>
              <div className="flex-1 overflow-y-auto space-y-0.5 px-1">
                {sessions.map(s => (
                  <div key={s.id} className="group flex items-center px-3 py-2.5 rounded-lg cursor-pointer text-sm transition-colors"
                    style={{ background: s.id === activeSessionId ? 'var(--bg-active)' : 'transparent', color: s.id === activeSessionId ? 'var(--text-primary)' : 'var(--text-secondary)', fontWeight: s.id === activeSessionId ? 500 : 400 }}
                    onMouseEnter={e => { if (s.id !== activeSessionId) { e.currentTarget.style.background = 'var(--bg-hover)'; e.currentTarget.style.color = 'var(--text-primary)'; }}}
                    onMouseLeave={e => { if (s.id !== activeSessionId) { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = 'var(--text-secondary)'; }}}
                    onClick={() => { onSelectSession(s.id); setIsMobileMenuOpen(false); }}>
                    <MessageSquare size={15} className="mr-2.5 flex-shrink-0" style={{ color: 'var(--text-muted)' }} />
                    <span className="flex-1 truncate">{s.title || '未命名对话'}</span>
                    <button className="opacity-100 md:opacity-0 group-hover:opacity-100 p-1 hover:text-red-500 transition-opacity"
                      onClick={e => { e.stopPropagation(); onDeleteSession(s.id); }}>
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="p-4 border-t" style={{ borderColor: 'var(--border)' }}>
          <div className="relative" ref={menuRef}>
            <button onClick={() => setIsThemeOpen(!isThemeOpen)}
              className="w-full flex items-center space-x-3 px-2 py-2 rounded-lg transition-colors cursor-pointer"
              style={{ background: isThemeOpen ? 'var(--bg-active)' : 'transparent' }}
              onMouseEnter={e => { if (!isThemeOpen) e.currentTarget.style.background = 'var(--bg-hover)'; }}
              onMouseLeave={e => { if (!isThemeOpen) e.currentTarget.style.background = 'transparent'; }}>
              {avatarOk
                ? <img src={avatarUrl} alt={user.name} className="w-8 h-8 rounded-full flex-shrink-0" onError={() => setAvatarOk(false)} />
                : (() => {
                    const colors = ['#B8A692', '#A3B5A6', '#B5A3A8', '#A6ADB8', '#C4B08F'];
                    return <div className="w-8 h-8 rounded-full text-white flex items-center justify-center text-sm font-semibold flex-shrink-0" style={{ background: colors[user.name.charCodeAt(0) % colors.length] }}>{user.name.slice(-2)}</div>;
                  })()
              }
              <div className="flex-1 min-w-0 text-left">
                <p className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>{user.name}</p>
                <p className="text-xs truncate" style={{ color: 'var(--text-muted)' }}>{user.role}</p>
              </div>
            </button>

            {isThemeOpen && (
              <div className="absolute bottom-full left-0 right-0 mb-2 rounded-xl shadow-lg border overflow-hidden" style={{ background: 'var(--bg-card)', borderColor: 'var(--border)' }}>
                <div className="px-4 py-3 flex items-center justify-between" style={{ borderBottom: '1px solid var(--border-light)' }}>
                  <span className="text-xs font-medium" style={{ color: 'var(--text-muted)' }}>主题</span>
                  <div className="flex gap-2">
                    {THEMES.map(t => (
                      <button key={t.id} onClick={() => switchTheme(t.id)} title={t.label}
                        className="w-6 h-6 rounded-full transition-transform hover:scale-110 flex items-center justify-center"
                        style={{ background: t.color, boxShadow: currentTheme === t.id ? '0 0 0 2px var(--text-primary)' : 'none' }}>
                        {currentTheme === t.id && <span className="text-white text-[10px]">✓</span>}
                      </button>
                    ))}
                  </div>
                </div>
                <button onClick={() => { setIsThemeOpen(false); setIsImportModalOpen(true); }}
                  className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm transition-colors cursor-pointer"
                  style={{ color: 'var(--text-dim)' }}
                  onMouseEnter={e => e.currentTarget.style.background = 'var(--bg-hover)'}
                  onMouseLeave={e => e.currentTarget.style.background = 'transparent'}>
                  <UploadCloud size={15} /> 导入历史数据
                </button>
                <button onClick={() => { setIsThemeOpen(false); setIsFeedbackOpen(true); loadFeedback(); }}
                  className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm transition-colors cursor-pointer"
                  style={{ color: 'var(--text-dim)' }}
                  onMouseEnter={e => e.currentTarget.style.background = 'var(--bg-hover)'}
                  onMouseLeave={e => e.currentTarget.style.background = 'transparent'}>
                  <MessageCircle size={15} /> 意见反馈
                </button>
                <button onClick={() => { logout(); window.location.reload(); }}
                  className="w-full flex items-center gap-2.5 px-4 py-2.5 text-sm transition-colors cursor-pointer"
                  style={{ color: '#dc2626' }}
                  onMouseEnter={e => e.currentTarget.style.background = 'var(--bg-hover)'}
                  onMouseLeave={e => e.currentTarget.style.background = 'transparent'}>
                  <LogOut size={15} /> 退出登录
                </button>
              </div>
            )}
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col h-full w-full relative pt-16 md:pt-0" style={{ background: 'var(--bg-page)' }}>
        {children}
      </main>

      {/* Mobile Overlay */}
      {isMobileMenuOpen && (
        <div className="fixed inset-0 bg-black/20 backdrop-blur-sm z-30 md:hidden" onClick={() => setIsMobileMenuOpen(false)} />
      )}

      {/* Import Modal */}
      {isImportModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm animate-fade-in">
          <div className={`rounded-2xl shadow-xl w-full overflow-hidden animate-scale-in ${uploadStatus === 'preview' ? 'max-w-3xl' : 'max-w-md'}`} style={{ background: 'var(--bg-page)' }}>
            <div className="flex items-center justify-between px-6 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
              <h3 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>导入团队历史日报</h3>
              <button onClick={handleCloseModal} className="transition-colors" style={{ color: 'var(--text-secondary)' }}><X size={20} /></button>
            </div>

            <div className="p-6 overflow-y-auto" style={{ maxHeight: 'calc(80vh - 120px)' }}>
              <input ref={fileInputRef} type="file" accept=".docx" className="hidden"
                onChange={e => { const f = e.target.files?.[0]; if (f) handleFileUpload(f); e.target.value = ''; }} />
              {uploadStatus === 'idle' && (
                <div onClick={() => fileInputRef.current?.click()} className="border-2 border-dashed rounded-xl p-8 flex flex-col items-center justify-center text-center transition-all cursor-pointer group" style={{ borderColor: 'var(--border)' }}>
                  <div className="w-12 h-12 rounded-full flex items-center justify-center mb-4 transition-colors" style={{ background: 'var(--bg-accent)' }}>
                    <FileUp size={24} style={{ color: 'var(--text-secondary)' }} />
                  </div>
                  <p className="text-sm font-medium mb-1" style={{ color: 'var(--text-primary)' }}>点击上传日报文档</p>
                  <p className="text-xs" style={{ color: 'var(--text-muted)' }}>支持 .docx 格式</p>
                </div>
              )}
              {uploadStatus === 'uploading' && (
                <div className="py-8 text-center space-y-4">
                  <div className="w-16 h-16 mx-auto relative flex items-center justify-center">
                    <svg className="animate-spin w-full h-full" style={{ color: 'var(--text-primary)' }} viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                    </svg>
                  </div>
                  <p className="text-sm font-medium" style={{ color: 'var(--text-dim)' }}>正在解析文档，AI 提取中... {elapsedTime}s</p>
                </div>
              )}
              {uploadStatus === 'preview' && previewData && (
                <div className="space-y-4">
                  <div className="flex items-center gap-3 text-sm" style={{ color: 'var(--text-dim)' }}>
                    <span className="px-2 py-1 rounded-md font-medium" style={{ background: 'var(--bg-accent)', color: 'var(--text-dim)' }}>{previewData.entries.length} 条记录</span>
                    <span className="px-2 py-1 rounded-md" style={{ background: 'var(--bg-accent)', color: 'var(--text-dim)' }}>{new Set(previewData.entries.map(e => e.date)).size} 天</span>
                    <span className="px-2 py-1 rounded-md" style={{ background: 'var(--bg-accent)', color: 'var(--text-dim)' }}>{new Set(previewData.entries.map(e => e.name)).size} 人</span>
                    <span className="px-2 py-1 rounded-md" style={{ background: 'var(--bg-accent)', color: 'var(--text-dim)' }}>{elapsedTime}s</span>
                  </div>
                  {previewData.unmatched_members.length > 0 && (
                    <div className="space-y-2">
                      <div className="text-xs font-medium text-amber-700 bg-amber-50 px-3 py-2 rounded-lg">
                        ⚠ {previewData.unmatched_members.length} 个未匹配成员，请复核：
                      </div>
                      <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--border)' }}>
                        <table className="w-full text-xs">
                          <thead style={{ background: 'var(--bg-sidebar)' }}>
                            <tr>
                              <th className="px-3 py-2 text-left font-medium" style={{ color: 'var(--text-dim)' }}>识别名称</th>
                              <th className="px-3 py-2 text-left font-medium" style={{ color: 'var(--text-dim)' }}>操作</th>
                              <th className="px-3 py-2 text-left font-medium" style={{ color: 'var(--text-dim)' }}>详情</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y" style={{ borderColor: 'var(--border-light)' }}>
                            {previewData.unmatched_members.map(name => {
                              const d = memberDecisions[name] || { action: 'create' as const, name };
                              const entryCount = previewData.entries.filter(e => e.name === name).length;
                              return (
                                <tr key={name}>
                                  <td className="px-3 py-2 whitespace-nowrap" style={{ color: 'var(--text-primary)' }}>
                                    {name} <span className="text-[10px]" style={{ color: 'var(--text-muted)' }}>({entryCount}条)</span>
                                  </td>
                                  <td className="px-3 py-2">
                                    <select className="text-xs rounded px-2 py-1" style={{ border: '1px solid var(--border)', background: 'var(--bg-card)' }}
                                      value={d.action}
                                      onChange={e => setMemberDecisions(prev => ({
                                        ...prev,
                                        [name]: e.target.value === 'create' ? { action: 'create', name }
                                          : e.target.value === 'map' ? { action: 'map', member_id: previewData.members?.[0]?.id }
                                          : { action: 'ignore' }
                                      }))}>
                                      <option value="create">创建新成员</option>
                                      <option value="map">关联已有成员</option>
                                      <option value="ignore">忽略</option>
                                    </select>
                                  </td>
                                  <td className="px-3 py-2">
                                    {d.action === 'create' && (
                                      <div className="flex items-center gap-1 flex-wrap">
                                        <input className="text-xs rounded px-2 py-1 w-20" style={{ border: '1px solid var(--border)' }} value={d.name || name} placeholder="姓名"
                                          onChange={e => setMemberDecisions(prev => ({ ...prev, [name]: { ...prev[name], name: e.target.value } }))} />
                                        <select className="text-xs rounded px-2 py-1" style={{ border: '1px solid var(--border)', background: 'var(--bg-card)' }}
                                          value={d.team_id || 0} onChange={e => setMemberDecisions(prev => ({ ...prev, [name]: { ...prev[name], team_id: Number(e.target.value) } }))}>
                                          <option value={0}>团队</option>
                                          {importTeams.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
                                        </select>
                                        <select className="text-xs rounded px-2 py-1" style={{ border: '1px solid var(--border)', background: 'var(--bg-card)' }}
                                          value={d.role || ''} onChange={e => setMemberDecisions(prev => ({ ...prev, [name]: { ...prev[name], role: e.target.value } }))}>
                                          <option value="">职位</option>
                                          <option value="Leader">Leader</option>
                                          <option value="后端开发">后端开发</option>
                                          <option value="前端开发">前端开发</option>
                                          <option value="测试">测试</option>
                                        </select>
                                      </div>
                                    )}
                                    {d.action === 'map' && previewData.members && (
                                      <select className="text-xs rounded px-2 py-1" style={{ border: '1px solid var(--border)', background: 'var(--bg-card)' }}
                                        value={d.member_id || ''} onChange={e => setMemberDecisions(prev => ({ ...prev, [name]: { ...prev[name], member_id: Number(e.target.value) } }))}>
                                        <option value="">选择成员</option>
                                        {previewData.members.map(m => <option key={m.id} value={m.id}>{m.name}</option>)}
                                      </select>
                                    )}
                                    {d.action === 'ignore' && <span style={{ color: 'var(--text-muted)' }}>跳过 {entryCount} 条日报</span>}
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  )}
                  <div className="max-h-64 overflow-y-auto rounded-lg" style={{ border: '1px solid var(--border)' }}>
                    <table className="w-full text-xs">
                      <thead className="sticky top-0" style={{ background: 'var(--bg-sidebar)' }}>
                        <tr>
                          <th className="px-3 py-2 text-left font-medium" style={{ color: 'var(--text-dim)' }}>日期</th>
                          <th className="px-3 py-2 text-left font-medium" style={{ color: 'var(--text-dim)' }}>成员</th>
                          <th className="px-3 py-2 text-left font-medium" style={{ color: 'var(--text-dim)' }}>工作内容</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y" style={{ borderColor: 'var(--border-light)' }}>
                        {previewData.entries.map((e, i) => (
                          <tr key={i}>
                            <td className="px-3 py-1.5 whitespace-nowrap" style={{ color: 'var(--text-dim)' }}>{e.date}</td>
                            <td className="px-3 py-1.5 whitespace-nowrap" style={{ color: 'var(--text-primary)' }}>{e.name}</td>
                            <td className="px-3 py-1.5 break-all" style={{ color: 'var(--text-dim)' }}>{e.content}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
              )}
              {uploadStatus === 'confirming' && (
                <div className="py-8 text-center space-y-4">
                  <div className="w-16 h-16 mx-auto relative flex items-center justify-center">
                    <svg className="animate-spin w-full h-full" style={{ color: 'var(--text-primary)' }} viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                    </svg>
                  </div>
                  <div className="space-y-1">
                    <p className="text-sm font-medium" style={{ color: 'var(--text-dim)' }}>正在写入数据库并同步至 MOI...</p>
                  </div>
                </div>
              )}
              {uploadStatus === 'success' && importResult && (
                <div className="py-6 text-center space-y-3">
                  <div className="w-14 h-14 bg-green-50 text-green-500 rounded-full flex items-center justify-center mx-auto mb-2">
                    <CheckCircle2 size={32} />
                  </div>
                  <h4 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>导入完成</h4>
                  <div className="text-sm space-y-1" style={{ color: 'var(--text-secondary)' }}>
                    <p>新增 {importResult.imported} 条，覆盖 {importResult.merged} 条，跳过 {importResult.skipped} 条</p>
                    <p className="text-xs">数据已同步至 MOI</p>
                    {importResult.skipped_members?.length > 0 && (
                      <p className="text-xs">未匹配成员：{importResult.skipped_members.join('、')}</p>
                    )}
                  </div>
                </div>
              )}
              {uploadStatus === 'error' && (
                <div className="py-6 text-center space-y-3">
                  <div className="w-14 h-14 bg-red-50 text-red-500 rounded-full flex items-center justify-center mx-auto mb-2">
                    <X size={32} />
                  </div>
                  <h4 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>操作失败</h4>
                  <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>{importError}</p>
                </div>
              )}
            </div>

            <div className="px-6 py-4 flex justify-end space-x-3" style={{ background: 'var(--bg-sidebar)' }}>
              {uploadStatus === 'preview' ? (<>
                <button onClick={handleCloseModal} className="px-4 py-2 text-sm font-medium rounded-lg transition-colors" style={{ color: 'var(--text-dim)' }}>取消</button>
                <button onClick={handleConfirmImport} className="px-4 py-2 text-sm font-medium text-white rounded-lg transition-colors" style={{ background: 'var(--btn-primary)' }}>确认导入</button>
              </>) : (uploadStatus === 'success' || uploadStatus === 'error') ? (
                <button onClick={handleCloseModal} className="px-4 py-2 text-sm font-medium text-white rounded-lg transition-colors" style={{ background: 'var(--btn-primary)' }}>完成</button>
              ) : uploadStatus === 'idle' ? (
                <button onClick={handleCloseModal} className="px-4 py-2 text-sm font-medium rounded-lg transition-colors" style={{ color: 'var(--text-dim)' }}>取消</button>
              ) : null}
            </div>
          </div>
        </div>
      )}

      {/* Feedback Modal */}
      {isFeedbackOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm animate-fade-in">
          <div className="rounded-2xl shadow-xl w-full max-w-md overflow-hidden animate-scale-in" style={{ background: 'var(--bg-page)' }}>
            <div className="flex items-center justify-between px-6 py-4" style={{ borderBottom: '1px solid var(--border)' }}>
              <h3 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>意见反馈</h3>
              <button onClick={() => setIsFeedbackOpen(false)} className="p-1 rounded-lg transition-colors cursor-pointer" style={{ color: 'var(--text-muted)' }}><X size={18} /></button>
            </div>
            <div className="px-6 py-4 space-y-4">
              <div className="flex gap-2">
                <input value={feedbackText} onChange={e => setFeedbackText(e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && handleSubmitFeedback()}
                  placeholder="写下你的意见或建议..."
                  className="flex-1 px-3 py-2 rounded-lg text-sm border outline-none"
                  style={{ background: 'var(--bg-input)', borderColor: 'var(--border)', color: 'var(--text-primary)' }} />
                <button onClick={handleSubmitFeedback} disabled={feedbackLoading || !feedbackText.trim()}
                  className="px-3 py-2 rounded-lg text-white text-sm transition-opacity disabled:opacity-40 cursor-pointer"
                  style={{ background: 'var(--btn-primary)' }}>
                  <Send size={15} />
                </button>
              </div>
              <div className="max-h-72 overflow-y-auto space-y-2">
                {feedbackList.length === 0 && <p className="text-center text-sm py-4" style={{ color: 'var(--text-muted)' }}>暂无反馈</p>}
                {feedbackList.map(fb => (
                  <div key={fb.id} className="px-3 py-2.5 rounded-lg text-sm" style={{ background: 'var(--bg-input)', border: '1px solid var(--border-light)' }}>
                    <div className="flex items-center justify-between mb-1">
                      <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{fb.member_name}</span>
                      <div className="flex items-center gap-2">
                        <span className="text-xs" style={{ color: fb.status === 'open' ? '#D06050' : '#5A9A6A' }}>{fb.status === 'open' ? '待处理' : '已关闭'}</span>
                        {user.is_admin && fb.status === 'open' && (
                          <button onClick={async () => { await closeFeedback(fb.id); loadFeedback(); }} className="cursor-pointer" style={{ color: 'var(--text-muted)' }} title="关闭"><XCircle size={14} /></button>
                        )}
                        {user.is_admin && (
                          <button onClick={async () => { await deleteFeedback(fb.id); loadFeedback(); }} className="cursor-pointer" style={{ color: '#dc2626' }} title="删除"><Trash2 size={14} /></button>
                        )}
                      </div>
                    </div>
                    <p style={{ color: 'var(--text-secondary)' }}>{fb.content}</p>
                    <p className="text-xs mt-1" style={{ color: 'var(--text-muted)' }}>{new Date(fb.created_at).toLocaleString('zh-CN')}</p>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ============ Sub-components ============

function NavItem({ icon: Icon, label, active, onClick }: {
  icon: React.ElementType; label: string; active: boolean; onClick: () => void;
}): React.ReactElement {
  return (
    <button
      onClick={onClick}
      className="w-full flex items-center space-x-3 px-3 py-2.5 rounded-lg transition-all duration-200 group"
      style={{ background: active ? 'var(--bg-active)' : 'transparent', color: active ? 'var(--text-primary)' : 'var(--text-secondary)', fontWeight: active ? 500 : 400 }}
    >
      <Icon className="w-5 h-5" style={{ color: active ? 'var(--text-primary)' : 'var(--text-muted)' }} />
      <span>{label}</span>
    </button>
  );
}
