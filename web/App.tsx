import React, { useState, useEffect } from 'react';
import { Layout } from './components/Layout';
import { ChatInterface } from './components/ChatInterface';
import { DailyFeed } from './components/DailyFeed';
import { Stats } from './components/Stats';
import { MyCalendar } from './components/MyCalendar';
import { ViewMode, User } from './types';
import { getCurrentUser, isLoggedIn, login, listSessions, deleteSession, SessionInfo } from './services/apiService';

function LoginPage({ onLogin }: { onLogin: (user: User) => void }): React.ReactElement {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [showPwd, setShowPwd] = useState(false);

  async function handleSubmit(e: React.FormEvent): Promise<void> {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      const user = await login(username, password);
      onLogin(user);
    } catch {
      setError('用户名或密码错误');
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex" style={{ background: '#FAF9F6', fontFamily: "'Inter', sans-serif" }}>
      <link href="https://fonts.googleapis.com/css2?family=Playfair+Display:wght@400;500;600;700&family=Inter:wght@300;400;500;600&display=swap" rel="stylesheet" />

      {/* Left brand panel */}
      <div className="hidden lg:flex flex-col justify-center w-1/2 px-16 py-12 relative">
        <div className="absolute right-0 top-[10%] h-[80%] w-px" style={{ background: 'linear-gradient(to bottom, transparent, #D4C5A9 30%, #D4C5A9 70%, transparent)' }} />

        {/* Top-left logo */}
        <div className="absolute top-12 left-16 flex items-center gap-3">
          <img src="/mo-logo.png" alt="MatrixOrigin" className="h-10 opacity-80" />
          <span className="text-xs font-semibold tracking-[3px] uppercase" style={{ color: '#2C2C2C' }}>MatrixOrigin</span>
        </div>

        {/* Center content */}
        <div>
          <div className="flex items-center gap-4 mb-4">
            <span className="text-5xl font-semibold" style={{ fontFamily: "'Playfair Display', serif", color: '#2C2C2C', letterSpacing: '-1px' }}>智能日报</span>
          </div>
          <p className="text-base leading-relaxed mb-12" style={{ color: '#8B7E6A', fontWeight: 300, letterSpacing: '2px' }}>
            以智驭繁 · 日有所记 · 事有所察
          </p>
          <div className="flex flex-col gap-4">
            {['LLM 流式摘要生成', '自然语言数据查询', '智能周报一键生成'].map(f => (
              <div key={f} className="flex items-center gap-3 text-sm" style={{ color: '#A09484' }}>
                <span style={{ fontSize: '6px', color: '#D4C5A9' }}>◆</span>
                {f}
              </div>
            ))}
          </div>
        </div>

        <p className="absolute bottom-12 left-16 text-xs" style={{ color: '#C4B8A4' }}>© 2026 矩阵起源 MatrixOrigin</p>
      </div>

      {/* Right login panel */}
      <div className="flex-1 flex items-center justify-center p-8">
        <div className="w-full max-w-sm">

          {/* Mobile logo */}
          <div className="lg:hidden flex flex-col items-center mb-10">
            <img src="/mo-logo.png" alt="MatrixOrigin" className="h-14 mb-3" />
            <span className="text-2xl font-semibold" style={{ fontFamily: "'Playfair Display', serif", color: '#2C2C2C' }}>智能日报</span>
            <p className="text-xs mt-1" style={{ color: '#A09484', letterSpacing: '2px' }}>以智驭繁 · 日有所记 · 事有所察</p>
          </div>

          <div className="mb-10 hidden lg:block">
            <h1 className="text-3xl font-medium mb-2" style={{ fontFamily: "'Playfair Display', serif", color: '#2C2C2C' }}>欢迎回来</h1>
            <p className="text-sm" style={{ color: '#A09484' }}>登录您的账号以继续</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-7">
            <div>
              <label className="block text-xs font-semibold mb-2 tracking-[1.5px] uppercase" style={{ color: '#8B7E6A' }}>用户名</label>
              <input
                type="text"
                value={username}
                onChange={e => setUsername(e.target.value)}
                placeholder="请输入用户名"
                autoFocus
                className="w-full py-3 text-sm focus:outline-none transition-all bg-transparent"
                style={{ border: 'none', borderBottom: '2px solid #E5DDD0', color: '#2C2C2C', WebkitBoxShadow: '0 0 0 30px #FAF9F6 inset' }}
                onFocus={e => e.target.style.borderBottomColor = '#2C2C2C'}
                onBlur={e => e.target.style.borderBottomColor = '#E5DDD0'}
              />
            </div>
            <div>
              <label className="block text-xs font-semibold mb-2 tracking-[1.5px] uppercase" style={{ color: '#8B7E6A' }}>密码</label>
              <div className="relative">
                <input
                  type={showPwd ? 'text' : 'password'}
                  value={password}
                  onChange={e => setPassword(e.target.value)}
                  placeholder="请输入密码"
                  className="w-full py-3 pr-10 text-sm focus:outline-none transition-all bg-transparent"
                  style={{ border: 'none', borderBottom: '2px solid #E5DDD0', color: '#2C2C2C', WebkitBoxShadow: '0 0 0 30px #FAF9F6 inset' }}
                  onFocus={e => e.target.style.borderBottomColor = '#2C2C2C'}
                  onBlur={e => e.target.style.borderBottomColor = '#E5DDD0'}
                />
                <button type="button" onClick={() => setShowPwd(v => !v)}
                  className="absolute right-0 top-1/2 -translate-y-1/2 transition-opacity hover:opacity-70"
                  style={{ color: '#A09484' }}>
                  {showPwd ? (
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94"/><path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                  ) : (
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                  )}
                </button>
              </div>
            </div>

            {error && (
              <p className="text-red-500 text-xs flex items-center gap-1.5 bg-red-50 px-3 py-2 rounded-lg border border-red-100">
                <span>⚠</span> {error}
              </p>
            )}

            <button
              type="submit"
              disabled={loading || !username || !password}
              className="w-full py-4 text-sm font-medium text-white mt-2 transition-all disabled:opacity-40 disabled:cursor-not-allowed hover:opacity-90 active:scale-[0.99]"
              style={{ background: '#2C2C2C', borderRadius: '8px', fontFamily: "'Playfair Display', serif", letterSpacing: '2px' }}
            >
              {loading ? '登录中...' : '登 录'}
            </button>
          </form>

          <p className="text-center text-xs mt-8" style={{ color: '#C4B8A4', lineHeight: 2 }}>
            测试账号：test / 123456
          </p>
          <p className="text-center text-xs" style={{ color: '#C4B8A4' }}>
            Powered by MatrixOne Intelligence
          </p>
        </div>
      </div>
    </div>
  );
}

export default function App(): React.ReactElement {
  const [currentView, setCurrentView] = useState<ViewMode>('chat');
  const [loggedIn, setLoggedIn] = useState(isLoggedIn());
  const [user, setUser] = useState<User>(getCurrentUser());
  const [sessionId, setSessionId] = useState<number | null>(null);
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [fadeIn, setFadeIn] = useState(false);

  useEffect(() => {
    if (loggedIn) { refreshSessions(); requestAnimationFrame(() => setFadeIn(true)); }
  }, [loggedIn]);

  function refreshSessions() {
    listSessions().then(setSessions).catch(() => {});
  }

  const [supplementDate, setSupplementDate] = useState<string | null>(null);

  function handleNewChat() {
    setSessionId(null);
    setSupplementDate(null);
    setCurrentView('chat');
  }

  function handleSelectSession(id: number) {
    setSessionId(id);
    setCurrentView('chat');
  }

  async function handleDeleteSession(id: number) {
    await deleteSession(id);
    if (sessionId === id) setSessionId(null);
    refreshSessions();
  }

  function handleSessionCreated(id: number) {
    setSessionId(id);
    refreshSessions();
  }

  if (!loggedIn) {
    return <LoginPage onLogin={(u) => { setUser(u); setFadeIn(false); setLoggedIn(true); }} />;
  }

  function handleSupplement(date: string) {
    setSupplementDate(date);
    setSessionId(null);
    setCurrentView('chat');
  }

  function renderContent(): React.ReactElement {
    return (
      <>
        <div style={{ display: currentView === 'chat' ? 'flex' : 'none', flexDirection: 'column', height: '100%' }}>
          <ChatInterface user={user} onReportSubmitted={() => {}} sessionId={sessionId} onSessionCreated={handleSessionCreated} supplementDate={supplementDate} onSupplementConsumed={() => setSupplementDate(null)} />
        </div>
        {currentView === 'feed' && <DailyFeed />}
        {currentView === 'stats' && <Stats />}
        {currentView === 'calendar' && <MyCalendar onSupplement={handleSupplement} />}
      </>
    );
  }

  return (
    <div style={{ opacity: fadeIn ? 1 : 0, transform: fadeIn ? 'scale(1)' : 'scale(0.98)', transition: 'opacity 0.4s ease, transform 0.4s ease' }}>
    <Layout currentView={currentView} onChangeView={setCurrentView} user={user}
      sessions={sessions} activeSessionId={sessionId}
      onNewChat={handleNewChat} onSelectSession={handleSelectSession} onDeleteSession={handleDeleteSession}>
      {renderContent()}
    </Layout>
    </div>
  );
}
