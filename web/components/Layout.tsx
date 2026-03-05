import React, { useState } from 'react';
import { LayoutDashboard, MessageSquare, PieChart, CalendarDays, Menu, X, Bell, UploadCloud, FileUp, CheckCircle2, LogOut, Trash2, Eye, PlusCircle } from 'lucide-react';
import { ViewMode, User } from '../types';
import { MO_LOGO, logout, SessionInfo, previewImport, confirmImport, PreviewEntry, PreviewResult, ConfirmResult, MemberDecision, getTeams, Team } from '../services/apiService';

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
  const [uploadStatus, setUploadStatus] = useState<'idle' | 'uploading' | 'preview' | 'confirming' | 'success' | 'error'>('idle');
  const [previewData, setPreviewData] = useState<PreviewResult | null>(null);
  const [importResult, setImportResult] = useState<ConfirmResult | null>(null);
  const [importError, setImportError] = useState('');
  const [memberDecisions, setMemberDecisions] = useState<Record<string, MemberDecision>>({});
  const [importTeams, setImportTeams] = useState<Team[]>([]);
  const [elapsedTime, setElapsedTime] = useState(0);
  const timerRef = React.useRef<ReturnType<typeof setInterval> | null>(null);
  const fileInputRef = React.useRef<HTMLInputElement>(null);

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
    <div className="flex h-screen overflow-hidden" style={{ background: '#FAF9F6', fontFamily: "'Inter', sans-serif" }}>
      {/* Mobile Header */}
      <div className="md:hidden fixed top-0 w-full h-16 border-b z-50 flex items-center px-4" style={{ background: '#FAF9F6', borderColor: '#E5DDD0' }}>
        <button onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)} className="p-2" style={{ color: '#8B7E6A' }}>
          {isMobileMenuOpen ? <X size={24} /> : <Menu size={24} />}
        </button>
        <div className="flex-1 flex items-center justify-center space-x-2">
          <div className="w-7 h-7 rounded-lg flex items-center justify-center overflow-hidden">
            <img src={MO_LOGO} alt="MOI" className="w-full h-full object-contain" />
          </div>
          <span className="font-semibold" style={{ color: '#2C2C2C' }}>智能日报</span>
        </div>
        <div className="w-10" />
      </div>

      {/* Sidebar */}
      <aside className={`
        fixed md:static inset-y-0 left-0 z-40 w-64 transform transition-transform duration-300 ease-in-out flex flex-col
        ${isMobileMenuOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'}
      `} style={{ background: '#F5F0E8' }}>
        <div className="h-16 flex items-center px-6">
          <div className="w-8 h-8 rounded-lg flex items-center justify-center mr-3 overflow-hidden">
            <img src={MO_LOGO} alt="MOI" className="w-full h-full object-contain" />
          </div>
          <span className="font-semibold text-lg tracking-tight" style={{ color: '#2C2C2C' }}>智能日报</span>
        </div>

        <div className="flex-1 px-3 py-6 space-y-1 overflow-hidden flex flex-col">
          <div className="text-xs font-semibold uppercase tracking-wider px-3 mb-3" style={{ color: '#A09484' }}>菜单</div>
          <NavItem icon={MessageSquare} label="AI 助手" active={currentView === 'chat' && !activeSessionId} onClick={() => { onNewChat(); setIsMobileMenuOpen(false); }} />
          <NavItem icon={CalendarDays} label="我的日历" active={currentView === 'calendar'} onClick={() => { onChangeView('calendar'); setIsMobileMenuOpen(false); }} />
          <NavItem icon={LayoutDashboard} label="团队动态" active={currentView === 'feed'} onClick={() => { onChangeView('feed'); setIsMobileMenuOpen(false); }} />
          <NavItem icon={PieChart} label="数据洞察" active={currentView === 'stats'} onClick={() => { onChangeView('stats'); setIsMobileMenuOpen(false); }} />

          {sessions.length > 0 && (
            <div className="mt-4 flex-1 min-h-0 flex flex-col">
              <div className="text-xs font-semibold uppercase tracking-wider px-3 mb-2" style={{ color: '#A09484' }}>对话历史</div>
              <div className="flex-1 overflow-y-auto space-y-0.5 px-1">
                {sessions.map(s => (
                  <div key={s.id} className="group flex items-center px-3 py-2.5 rounded-lg cursor-pointer text-sm transition-colors"
                    style={{ background: s.id === activeSessionId ? '#E5DDD0' : 'transparent', color: s.id === activeSessionId ? '#2C2C2C' : '#8B7E6A', fontWeight: s.id === activeSessionId ? 500 : 400 }}
                    onMouseEnter={e => { if (s.id !== activeSessionId) { e.currentTarget.style.background = '#EDE8E0'; e.currentTarget.style.color = '#2C2C2C'; }}}
                    onMouseLeave={e => { if (s.id !== activeSessionId) { e.currentTarget.style.background = 'transparent'; e.currentTarget.style.color = '#8B7E6A'; }}}
                    onClick={() => { onSelectSession(s.id); setIsMobileMenuOpen(false); }}>
                    <MessageSquare size={15} className="mr-2.5 flex-shrink-0" style={{ color: '#A09484' }} />
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

        <div className="p-4 border-t" style={{ borderColor: '#E5DDD0' }}>
          <div className="flex items-center space-x-3 px-2 py-2">
            {(() => {
              const colors = ['#B8A692', '#A3B5A6', '#B5A3A8', '#A6ADB8', '#C4B08F'];
              const bg = colors[user.name.charCodeAt(0) % colors.length];
              return <div className="w-8 h-8 rounded-full text-white flex items-center justify-center text-sm font-semibold" style={{ background: bg }}>{user.name.slice(-2)}</div>;
            })()}
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium truncate" style={{ color: '#2C2C2C' }}>{user.name}</p>
              <p className="text-xs truncate" style={{ color: '#A09484' }}>{user.role}</p>
            </div>
            <div className="flex items-center space-x-1">
              <button onClick={() => setIsImportModalOpen(true)} className="p-1.5 rounded-lg transition-colors" style={{ color: '#8B7E6A' }} title="导入历史数据">
                <UploadCloud size={16} />
              </button>
              <button className="p-1.5 rounded-lg transition-colors" style={{ color: '#8B7E6A' }} title="通知">
                <Bell size={16} />
              </button>
              <button onClick={() => { logout(); window.location.reload(); }} className="p-1.5 hover:text-red-500 rounded-lg transition-colors" style={{ color: '#8B7E6A' }} title="退出登录">
                <LogOut size={16} />
              </button>
            </div>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col h-full w-full relative pt-16 md:pt-0" style={{ background: '#FAF9F6' }}>
        {children}
      </main>

      {/* Mobile Overlay */}
      {isMobileMenuOpen && (
        <div className="fixed inset-0 bg-black/20 backdrop-blur-sm z-30 md:hidden" onClick={() => setIsMobileMenuOpen(false)} />
      )}

      {/* Import Modal */}
      {isImportModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm animate-fade-in">
          <div className={`rounded-2xl shadow-xl w-full overflow-hidden animate-scale-in ${uploadStatus === 'preview' ? 'max-w-3xl' : 'max-w-md'}`} style={{ background: '#FAF9F6' }}>
            <div className="flex items-center justify-between px-6 py-4" style={{ borderBottom: '1px solid #E5DDD0' }}>
              <h3 className="text-lg font-semibold" style={{ color: '#2C2C2C' }}>导入团队历史日报</h3>
              <button onClick={handleCloseModal} className="transition-colors" style={{ color: '#8B7E6A' }}><X size={20} /></button>
            </div>

            <div className="p-6 overflow-y-auto" style={{ maxHeight: 'calc(80vh - 120px)' }}>
              <input ref={fileInputRef} type="file" accept=".docx" className="hidden"
                onChange={e => { const f = e.target.files?.[0]; if (f) handleFileUpload(f); e.target.value = ''; }} />
              {uploadStatus === 'idle' && (
                <div onClick={() => fileInputRef.current?.click()} className="border-2 border-dashed rounded-xl p-8 flex flex-col items-center justify-center text-center transition-all cursor-pointer group" style={{ borderColor: '#E5DDD0' }}>
                  <div className="w-12 h-12 rounded-full flex items-center justify-center mb-4 transition-colors" style={{ background: '#F0EBE3' }}>
                    <FileUp size={24} style={{ color: '#8B7E6A' }} />
                  </div>
                  <p className="text-sm font-medium mb-1" style={{ color: '#2C2C2C' }}>点击上传日报文档</p>
                  <p className="text-xs" style={{ color: '#A09484' }}>支持 .docx 格式</p>
                </div>
              )}
              {uploadStatus === 'uploading' && (
                <div className="py-8 text-center space-y-4">
                  <div className="w-16 h-16 mx-auto relative flex items-center justify-center">
                    <svg className="animate-spin w-full h-full" style={{ color: '#2C2C2C' }} viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                    </svg>
                  </div>
                  <p className="text-sm font-medium" style={{ color: '#6B5E4F' }}>正在解析文档，AI 提取中... {elapsedTime}s</p>
                </div>
              )}
              {uploadStatus === 'preview' && previewData && (
                <div className="space-y-4">
                  <div className="flex items-center gap-3 text-sm" style={{ color: '#6B5E4F' }}>
                    <span className="px-2 py-1 rounded-md font-medium" style={{ background: '#F0EBE3', color: '#6B5E4F' }}>{previewData.entries.length} 条记录</span>
                    <span className="px-2 py-1 rounded-md" style={{ background: '#F0EBE3', color: '#6B5E4F' }}>{new Set(previewData.entries.map(e => e.date)).size} 天</span>
                    <span className="px-2 py-1 rounded-md" style={{ background: '#F0EBE3', color: '#6B5E4F' }}>{new Set(previewData.entries.map(e => e.name)).size} 人</span>
                    <span className="px-2 py-1 rounded-md" style={{ background: '#F0EBE3', color: '#6B5E4F' }}>{elapsedTime}s</span>
                  </div>
                  {previewData.unmatched_members.length > 0 && (
                    <div className="space-y-2">
                      <div className="text-xs font-medium text-amber-700 bg-amber-50 px-3 py-2 rounded-lg">
                        ⚠ {previewData.unmatched_members.length} 个未匹配成员，请复核：
                      </div>
                      <div className="rounded-lg overflow-hidden" style={{ border: '1px solid #E5DDD0' }}>
                        <table className="w-full text-xs">
                          <thead style={{ background: '#F5F0E8' }}>
                            <tr>
                              <th className="px-3 py-2 text-left font-medium" style={{ color: '#6B5E4F' }}>识别名称</th>
                              <th className="px-3 py-2 text-left font-medium" style={{ color: '#6B5E4F' }}>操作</th>
                              <th className="px-3 py-2 text-left font-medium" style={{ color: '#6B5E4F' }}>详情</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y" style={{ borderColor: '#F0EBE3' }}>
                            {previewData.unmatched_members.map(name => {
                              const d = memberDecisions[name] || { action: 'create' as const, name };
                              const entryCount = previewData.entries.filter(e => e.name === name).length;
                              return (
                                <tr key={name}>
                                  <td className="px-3 py-2 whitespace-nowrap" style={{ color: '#2C2C2C' }}>
                                    {name} <span className="text-[10px]" style={{ color: '#A09484' }}>({entryCount}条)</span>
                                  </td>
                                  <td className="px-3 py-2">
                                    <select className="text-xs rounded px-2 py-1" style={{ border: '1px solid #E5DDD0', background: '#FFFCF8' }}
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
                                        <input className="text-xs rounded px-2 py-1 w-20" style={{ border: '1px solid #E5DDD0' }} value={d.name || name} placeholder="姓名"
                                          onChange={e => setMemberDecisions(prev => ({ ...prev, [name]: { ...prev[name], name: e.target.value } }))} />
                                        <select className="text-xs rounded px-2 py-1" style={{ border: '1px solid #E5DDD0', background: '#FFFCF8' }}
                                          value={d.team_id || 0} onChange={e => setMemberDecisions(prev => ({ ...prev, [name]: { ...prev[name], team_id: Number(e.target.value) } }))}>
                                          <option value={0}>团队</option>
                                          {importTeams.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
                                        </select>
                                        <select className="text-xs rounded px-2 py-1" style={{ border: '1px solid #E5DDD0', background: '#FFFCF8' }}
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
                                      <select className="text-xs rounded px-2 py-1" style={{ border: '1px solid #E5DDD0', background: '#FFFCF8' }}
                                        value={d.member_id || ''} onChange={e => setMemberDecisions(prev => ({ ...prev, [name]: { ...prev[name], member_id: Number(e.target.value) } }))}>
                                        <option value="">选择成员</option>
                                        {previewData.members.map(m => <option key={m.id} value={m.id}>{m.name}</option>)}
                                      </select>
                                    )}
                                    {d.action === 'ignore' && <span style={{ color: '#A09484' }}>跳过 {entryCount} 条日报</span>}
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  )}
                  <div className="max-h-64 overflow-y-auto rounded-lg" style={{ border: '1px solid #E5DDD0' }}>
                    <table className="w-full text-xs">
                      <thead className="sticky top-0" style={{ background: '#F5F0E8' }}>
                        <tr>
                          <th className="px-3 py-2 text-left font-medium" style={{ color: '#6B5E4F' }}>日期</th>
                          <th className="px-3 py-2 text-left font-medium" style={{ color: '#6B5E4F' }}>成员</th>
                          <th className="px-3 py-2 text-left font-medium" style={{ color: '#6B5E4F' }}>工作内容</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y" style={{ borderColor: '#F0EBE3' }}>
                        {previewData.entries.map((e, i) => (
                          <tr key={i}>
                            <td className="px-3 py-1.5 whitespace-nowrap" style={{ color: '#6B5E4F' }}>{e.date}</td>
                            <td className="px-3 py-1.5 whitespace-nowrap" style={{ color: '#2C2C2C' }}>{e.name}</td>
                            <td className="px-3 py-1.5 break-all" style={{ color: '#6B5E4F' }}>{e.content}</td>
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
                    <svg className="animate-spin w-full h-full" style={{ color: '#2C2C2C' }} viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                    </svg>
                  </div>
                  <div className="space-y-1">
                    <p className="text-sm font-medium" style={{ color: '#6B5E4F' }}>正在写入数据库并同步至 MOI...</p>
                  </div>
                </div>
              )}
              {uploadStatus === 'success' && importResult && (
                <div className="py-6 text-center space-y-3">
                  <div className="w-14 h-14 bg-green-50 text-green-500 rounded-full flex items-center justify-center mx-auto mb-2">
                    <CheckCircle2 size={32} />
                  </div>
                  <h4 className="text-lg font-semibold" style={{ color: '#2C2C2C' }}>导入完成</h4>
                  <div className="text-sm space-y-1" style={{ color: '#8B7E6A' }}>
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
                  <h4 className="text-lg font-semibold" style={{ color: '#2C2C2C' }}>操作失败</h4>
                  <p className="text-sm" style={{ color: '#8B7E6A' }}>{importError}</p>
                </div>
              )}
            </div>

            <div className="px-6 py-4 flex justify-end space-x-3" style={{ background: '#F5F0E8' }}>
              {uploadStatus === 'preview' ? (<>
                <button onClick={handleCloseModal} className="px-4 py-2 text-sm font-medium rounded-lg transition-colors" style={{ color: '#6B5E4F' }}>取消</button>
                <button onClick={handleConfirmImport} className="px-4 py-2 text-sm font-medium text-white rounded-lg transition-colors" style={{ background: '#2C2C2C' }}>确认导入</button>
              </>) : (uploadStatus === 'success' || uploadStatus === 'error') ? (
                <button onClick={handleCloseModal} className="px-4 py-2 text-sm font-medium text-white rounded-lg transition-colors" style={{ background: '#2C2C2C' }}>完成</button>
              ) : uploadStatus === 'idle' ? (
                <button onClick={handleCloseModal} className="px-4 py-2 text-sm font-medium rounded-lg transition-colors" style={{ color: '#6B5E4F' }}>取消</button>
              ) : null}
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
      style={{ background: active ? '#E5DDD0' : 'transparent', color: active ? '#2C2C2C' : '#8B7E6A', fontWeight: active ? 500 : 400 }}
    >
      <Icon className="w-5 h-5" style={{ color: active ? '#2C2C2C' : '#A09484' }} />
      <span>{label}</span>
    </button>
  );
}
