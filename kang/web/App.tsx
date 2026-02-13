import React, { useState } from 'react';
import { Layout } from './components/Layout';
import { ChatInterface } from './components/ChatInterface';
import { DailyFeed } from './components/DailyFeed';
import { Stats } from './components/Stats';
import { ViewMode, User } from './types';
import { getCurrentUser, isLoggedIn, login } from './services/apiService';

function LoginPage({ onLogin }: { onLogin: (user: User) => void }): React.ReactElement {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

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
    <div className="min-h-screen bg-white flex items-center justify-center">
      <div className="w-full max-w-sm px-8">
        <div className="text-center mb-8">
          <h1 className="text-2xl font-semibold text-gray-900 tracking-tight">MOI 智能日报</h1>
          <p className="text-gray-500 text-sm mt-2">登录以继续</p>
        </div>
        <form onSubmit={handleSubmit} className="space-y-4">
          <input type="text" value={username} onChange={e => setUsername(e.target.value)} placeholder="用户名" autoFocus
            className="w-full px-4 py-3 border border-gray-200 rounded-xl text-sm focus:outline-none focus:border-gray-400 transition-colors" />
          <input type="password" value={password} onChange={e => setPassword(e.target.value)} placeholder="密码"
            className="w-full px-4 py-3 border border-gray-200 rounded-xl text-sm focus:outline-none focus:border-gray-400 transition-colors" />
          {error && <p className="text-red-500 text-xs">{error}</p>}
          <button type="submit" disabled={loading || !username || !password}
            className="w-full py-3 bg-gray-900 text-white text-sm font-medium rounded-xl hover:bg-black transition-colors disabled:opacity-50">
            {loading ? '登录中...' : '登录'}
          </button>
        </form>
        <p className="text-center text-xs text-gray-400 mt-6">测试账号：chenalei / 123456</p>
      </div>
    </div>
  );
}

export default function App(): React.ReactElement {
  const [currentView, setCurrentView] = useState<ViewMode>('chat');
  const [loggedIn, setLoggedIn] = useState(isLoggedIn());
  const [user, setUser] = useState<User>(getCurrentUser());

  if (!loggedIn) {
    return <LoginPage onLogin={(u) => { setUser(u); setLoggedIn(true); }} />;
  }

  function renderContent(): React.ReactElement {
    switch (currentView) {
      case 'feed': return <DailyFeed />;
      case 'stats': return <Stats />;
      default: return <ChatInterface user={user} onReportSubmitted={() => {}} />;
    }
  }

  return (
    <Layout currentView={currentView} onChangeView={setCurrentView} user={user}>
      {renderContent()}
    </Layout>
  );
}
