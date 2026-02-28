import React, { useState } from 'react';
import { LayoutDashboard, MessageSquare, PieChart, Menu, X, Bell, UploadCloud, FileUp, CheckCircle2, LogOut, Trash2, Eye } from 'lucide-react';
import { ViewMode, User } from '../types';
import { MO_LOGO, logout, SessionInfo, previewImport, confirmImport, PreviewEntry, PreviewResult, ConfirmResult } from '../services/apiService';

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
  const fileInputRef = React.useRef<HTMLInputElement>(null);

  function handleCloseModal(): void {
    setIsImportModalOpen(false);
    setTimeout(() => { setUploadStatus('idle'); setPreviewData(null); setImportResult(null); setImportError(''); }, 300);
  }

  async function handleFileUpload(file: File) {
    setUploadStatus('uploading');
    try {
      const result = await previewImport(file);
      setPreviewData(result);
      setUploadStatus('preview');
    } catch (e: any) {
      setImportError(e.message || '解析失败');
      setUploadStatus('error');
    }
  }

  async function handleConfirmImport() {
    if (!previewData?.token) return;
    setUploadStatus('confirming');
    try {
      const result = await confirmImport(previewData.token);
      setImportResult(result);
      setUploadStatus('success');
    } catch (e: any) {
      setImportError(e.message || '导入失败');
      setUploadStatus('error');
    }
  }

  return (
    <div className="flex h-screen bg-white overflow-hidden">
      {/* Mobile Header */}
      <div className="md:hidden fixed top-0 w-full h-16 bg-white border-b border-gray-100 z-50 flex items-center justify-between px-4">
        <div className="flex items-center space-x-2">
          <div className="w-8 h-8 rounded-lg flex items-center justify-center overflow-hidden">
            <img src={MO_LOGO} alt="MOI" className="w-full h-full object-contain" />
          </div>
          <span className="font-semibold text-gray-900">MOI 智能日报</span>
        </div>
        <button onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)} className="p-2 text-gray-500">
          {isMobileMenuOpen ? <X size={24} /> : <Menu size={24} />}
        </button>
      </div>

      {/* Sidebar */}
      <aside className={`
        fixed md:static inset-y-0 left-0 z-40 w-64 bg-[#f9f9f9] transform transition-transform duration-300 ease-in-out flex flex-col
        ${isMobileMenuOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'}
      `}>
        <div className="h-16 flex items-center px-6">
          <div className="w-8 h-8 rounded-lg flex items-center justify-center mr-3 overflow-hidden">
            <img src={MO_LOGO} alt="MOI" className="w-full h-full object-contain" />
          </div>
          <span className="font-semibold text-lg tracking-tight text-gray-900">MOI 智能日报</span>
        </div>

        <div className="flex-1 px-3 py-6 space-y-1 overflow-hidden flex flex-col">
          <div className="text-xs font-semibold text-gray-400 uppercase tracking-wider px-3 mb-3">菜单</div>
          <NavItem icon={MessageSquare} label="AI 助手" active={currentView === 'chat' && !activeSessionId} onClick={() => { onNewChat(); setIsMobileMenuOpen(false); }} />
          <NavItem icon={LayoutDashboard} label="团队动态" active={currentView === 'feed'} onClick={() => { onChangeView('feed'); setIsMobileMenuOpen(false); }} />
          <NavItem icon={PieChart} label="数据洞察" active={currentView === 'stats'} onClick={() => { onChangeView('stats'); setIsMobileMenuOpen(false); }} />

          {sessions.length > 0 && (
            <div className="mt-4 flex-1 min-h-0 flex flex-col">
              <div className="text-xs font-semibold text-gray-400 uppercase tracking-wider px-3 mb-2">对话历史</div>
              <div className="flex-1 overflow-y-auto space-y-0.5 px-1">
                {sessions.map(s => (
                  <div key={s.id} className={`group flex items-center px-3 py-2.5 rounded-lg cursor-pointer text-sm transition-colors ${s.id === activeSessionId ? 'bg-gray-200 text-gray-900 font-medium' : 'text-gray-500 hover:bg-gray-100 hover:text-gray-900'}`}
                    onClick={() => onSelectSession(s.id)}>
                    <MessageSquare size={15} className="mr-2.5 flex-shrink-0 text-gray-500" />
                    <span className="flex-1 truncate">{s.title || '未命名对话'}</span>
                    <button className="opacity-0 group-hover:opacity-100 p-1 hover:text-red-500 transition-opacity"
                      onClick={e => { e.stopPropagation(); onDeleteSession(s.id); }}>
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="p-4 border-t border-gray-200">
          <div className="flex items-center space-x-3 px-2 py-2">
            {user.avatar ? (
              <img src={user.avatar} alt={user.name} className="w-8 h-8 rounded-full bg-gray-200 object-cover" />
            ) : (
              <div className="w-8 h-8 rounded-full bg-indigo-500 text-white flex items-center justify-center text-sm font-semibold">{user.name.slice(-2)}</div>
            )}
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium text-gray-900 truncate">{user.name}</p>
              <p className="text-xs text-gray-500 truncate">{user.role}</p>
            </div>
            <div className="flex items-center space-x-1">
              <button onClick={() => setIsImportModalOpen(true)} className="p-1.5 text-gray-500 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors" title="导入历史数据">
                <UploadCloud size={16} />
              </button>
              <button className="p-1.5 text-gray-500 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors" title="通知">
                <Bell size={16} />
              </button>
              <button onClick={() => { logout(); window.location.reload(); }} className="p-1.5 text-gray-500 hover:text-red-500 hover:bg-gray-100 rounded-lg transition-colors" title="退出登录">
                <LogOut size={16} />
              </button>
            </div>
          </div>
        </div>
      </aside>

      {/* Main Content */}
      <main className="flex-1 flex flex-col h-full w-full relative pt-16 md:pt-0 bg-white">
        {children}
      </main>

      {/* Mobile Overlay */}
      {isMobileMenuOpen && (
        <div className="fixed inset-0 bg-black/20 backdrop-blur-sm z-30 md:hidden" onClick={() => setIsMobileMenuOpen(false)} />
      )}

      {/* Import Modal */}
      {isImportModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40 backdrop-blur-sm animate-fade-in">
          <div className={`bg-white rounded-2xl shadow-xl w-full overflow-hidden animate-scale-in ${uploadStatus === 'preview' ? 'max-w-3xl' : 'max-w-md'}`}>
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
              <h3 className="text-lg font-semibold text-gray-900">导入团队历史日报</h3>
              <button onClick={handleCloseModal} className="text-gray-500 hover:text-gray-600 transition-colors"><X size={20} /></button>
            </div>

            <div className="p-6">
              <input ref={fileInputRef} type="file" accept=".docx" className="hidden"
                onChange={e => { const f = e.target.files?.[0]; if (f) handleFileUpload(f); e.target.value = ''; }} />
              {uploadStatus === 'idle' && (
                <div onClick={() => fileInputRef.current?.click()} className="border-2 border-dashed border-gray-300 rounded-xl p-8 flex flex-col items-center justify-center text-center hover:border-gray-500 transition-all cursor-pointer group">
                  <div className="w-12 h-12 bg-gray-100 rounded-full flex items-center justify-center mb-4 group-hover:bg-gray-200 transition-colors">
                    <FileUp className="text-gray-500" size={24} />
                  </div>
                  <p className="text-sm font-medium text-gray-900 mb-1">点击上传日报文档</p>
                  <p className="text-xs text-gray-500">支持 .docx 格式</p>
                </div>
              )}
              {uploadStatus === 'uploading' && (
                <div className="py-8 text-center space-y-4">
                  <div className="w-16 h-16 mx-auto relative flex items-center justify-center">
                    <svg className="animate-spin w-full h-full text-gray-900" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                    </svg>
                  </div>
                  <p className="text-sm text-gray-600 font-medium">正在解析文档，AI 提取中...</p>
                </div>
              )}
              {uploadStatus === 'preview' && previewData && (
                <div className="space-y-4">
                  <div className="flex items-center gap-3 text-sm text-gray-600">
                    <span className="px-2 py-1 bg-blue-50 text-blue-700 rounded-md font-medium">{previewData.entries.length} 条记录</span>
                    <span className="px-2 py-1 bg-gray-100 text-gray-600 rounded-md">{new Set(previewData.entries.map(e => e.date)).size} 天</span>
                    <span className="px-2 py-1 bg-gray-100 text-gray-600 rounded-md">{new Set(previewData.entries.map(e => e.name)).size} 人</span>
                  </div>
                  {previewData.unmatched_members.length > 0 && (
                    <div className="text-xs text-amber-600 bg-amber-50 px-3 py-2 rounded-lg">
                      ⚠ 未匹配成员（将跳过）：{previewData.unmatched_members.join('、')}
                    </div>
                  )}
                  <div className="max-h-64 overflow-y-auto border border-gray-200 rounded-lg">
                    <table className="w-full text-xs">
                      <thead className="bg-gray-50 sticky top-0">
                        <tr>
                          <th className="px-3 py-2 text-left text-gray-500 font-medium">日期</th>
                          <th className="px-3 py-2 text-left text-gray-500 font-medium">成员</th>
                          <th className="px-3 py-2 text-left text-gray-500 font-medium">工作内容</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-gray-100">
                        {previewData.entries.map((e, i) => (
                          <tr key={i} className="hover:bg-gray-50">
                            <td className="px-3 py-1.5 text-gray-600 whitespace-nowrap">{e.date}</td>
                            <td className="px-3 py-1.5 text-gray-900 whitespace-nowrap">{e.name}</td>
                            <td className="px-3 py-1.5 text-gray-600 break-all">{e.content}</td>
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
                    <svg className="animate-spin w-full h-full text-gray-900" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
                    </svg>
                  </div>
                  <div className="space-y-1">
                    <p className="text-sm text-gray-600 font-medium">正在写入数据库并同步至 MOI...</p>
                  </div>
                </div>
              )}
              {uploadStatus === 'success' && importResult && (
                <div className="py-6 text-center space-y-3">
                  <div className="w-14 h-14 bg-green-50 text-green-500 rounded-full flex items-center justify-center mx-auto mb-2">
                    <CheckCircle2 size={32} />
                  </div>
                  <h4 className="text-lg font-semibold text-gray-900">导入完成</h4>
                  <div className="text-sm text-gray-500 space-y-1">
                    <p>新增 {importResult.imported} 条，覆盖 {importResult.merged} 条，跳过 {importResult.skipped} 条</p>
                    <p className="text-xs text-gray-500">数据已同步至 MOI</p>
                    {importResult.skipped_members?.length > 0 && (
                      <p className="text-xs text-gray-500">未匹配成员：{importResult.skipped_members.join('、')}</p>
                    )}
                  </div>
                </div>
              )}
              {uploadStatus === 'error' && (
                <div className="py-6 text-center space-y-3">
                  <div className="w-14 h-14 bg-red-50 text-red-500 rounded-full flex items-center justify-center mx-auto mb-2">
                    <X size={32} />
                  </div>
                  <h4 className="text-lg font-semibold text-gray-900">操作失败</h4>
                  <p className="text-sm text-gray-500">{importError}</p>
                </div>
              )}
            </div>

            <div className="px-6 py-4 bg-gray-50 flex justify-end space-x-3">
              {uploadStatus === 'preview' ? (<>
                <button onClick={handleCloseModal} className="px-4 py-2 text-gray-600 text-sm font-medium hover:bg-gray-200 rounded-lg transition-colors">取消</button>
                <button onClick={handleConfirmImport} className="px-4 py-2 bg-[#f9f9f9] text-gray-900 text-sm font-medium rounded-lg hover:bg-black transition-colors">确认导入</button>
              </>) : (uploadStatus === 'success' || uploadStatus === 'error') ? (
                <button onClick={handleCloseModal} className="px-4 py-2 bg-[#f9f9f9] text-gray-900 text-sm font-medium rounded-lg hover:bg-black transition-colors">完成</button>
              ) : uploadStatus === 'idle' ? (
                <button onClick={handleCloseModal} className="px-4 py-2 text-gray-600 text-sm font-medium hover:bg-gray-200 rounded-lg transition-colors">取消</button>
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
      className={`w-full flex items-center space-x-3 px-3 py-2.5 rounded-lg transition-all duration-200 group ${
        active ? 'bg-gray-200 text-gray-900 font-medium' : 'text-gray-500 hover:bg-gray-100 hover:text-gray-900'
      }`}
    >
      <Icon className={`w-5 h-5 ${active ? 'text-gray-900' : 'text-gray-500 group-hover:text-gray-600'}`} />
      <span>{label}</span>
    </button>
  );
}
