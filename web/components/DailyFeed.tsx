import React, { useEffect, useState } from 'react';
import { Member } from '../types';
import {
  getMembers, updateMember, deleteMember, getTeams, createTeam, Team,
  getFeedByMember, getFeedByTopic, MemberFeed, TopicFeed,
} from '../services/apiService';
import { Users, BarChart2, Hash, Pencil, Check, X, Download, Trash2 } from 'lucide-react';
import { ConfirmModal } from './ConfirmModal';

type Tab = 'by-member' | 'by-topic' | 'members';

const STATUS_LABELS: Record<string, { label: string; color: string }> = {
  active:      { label: '在职',  color: 'bg-green-100 text-green-700' },
  resigned:    { label: '离职',  color: 'bg-red-100 text-red-600' },
  transferred: { label: '转岗',  color: 'bg-yellow-100 text-yellow-700' },
};
const ROLE_OPTIONS = ['Leader', '后端开发', '前端开发', '测试', '开发工程师'];

interface EditState { id: number; status: string; teamId: number; role: string }

export function DailyFeed(): React.ReactElement {
  const [tab, setTab] = useState<Tab>('by-member');
  const [members, setMembers] = useState<Member[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [loading, setLoading] = useState(false);
  const [editing, setEditing] = useState<EditState | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Member | null>(null);
  const [teamModal, setTeamModal] = useState(false);

  // Feed state
  const [memberFeeds, setMemberFeeds] = useState<MemberFeed[]>([]);
  const [topicFeeds, setTopicFeeds] = useState<TopicFeed[]>([]);
  const [dateRange, setDateRange] = useState({ start: '', end: '' });
  const [startInput, setStartInput] = useState('');
  const [endInput, setEndInput] = useState('');

  // Date range presets
  type Preset = '本周' | '上周' | '近7天' | '近30天';
  const [activePreset, setActivePreset] = useState<Preset>('近7天');

  function getPresetRange(preset: Preset): { start: string; end: string } {
    const today = new Date();
    const fmt = (d: Date) => d.toISOString().slice(0, 10);
    const dayOfWeek = today.getDay() || 7; // Monday=1
    if (preset === '本周') {
      const mon = new Date(today); mon.setDate(today.getDate() - dayOfWeek + 1);
      return { start: fmt(mon), end: fmt(today) };
    }
    if (preset === '上周') {
      const lastMon = new Date(today); lastMon.setDate(today.getDate() - dayOfWeek - 6);
      const lastSun = new Date(lastMon); lastSun.setDate(lastMon.getDate() + 6);
      return { start: fmt(lastMon), end: fmt(lastSun) };
    }
    if (preset === '近30天') {
      const d = new Date(today); d.setDate(today.getDate() - 29);
      return { start: fmt(d), end: fmt(today) };
    }
    // 近7天
    const d = new Date(today); d.setDate(today.getDate() - 6);
    return { start: fmt(d), end: fmt(today) };
  }

  function applyPreset(preset: Preset) {
    setActivePreset(preset);
    const { start, end } = getPresetRange(preset);
    setStartInput(start); setEndInput(end);
  }

  useEffect(() => {
    if (tab === 'members') {
      setLoading(true);
      Promise.all([getMembers(), getTeams()]).then(([m, t]) => {
        setMembers(m); setTeams(t); setLoading(false);
      });
    } else if (tab === 'by-member') {
      const s = startInput || getPresetRange(activePreset).start;
      const e = endInput || getPresetRange(activePreset).end;
      if (!startInput) { setStartInput(s); setEndInput(e); }
      setLoading(true);
      getFeedByMember(s, e).then(r => {
        setMemberFeeds(r.members); setDateRange({ start: r.start, end: r.end }); setLoading(false);
      });
    } else if (tab === 'by-topic') {
      const s = startInput || getPresetRange(activePreset).start;
      const e = endInput || getPresetRange(activePreset).end;
      if (!startInput) { setStartInput(s); setEndInput(e); }
      setLoading(true);
      getFeedByTopic(s, e).then(r => {
        setTopicFeeds(r.topics); setDateRange({ start: r.start, end: r.end }); setLoading(false);
      });
    }
  }, [tab]);

  function refreshFeed() {
    if (tab === 'by-member') {
      setLoading(true);
      getFeedByMember(startInput, endInput).then(r => {
        setMemberFeeds(r.members); setDateRange({ start: r.start, end: r.end }); setLoading(false);
      });
    } else if (tab === 'by-topic') {
      setLoading(true);
      getFeedByTopic(startInput, endInput).then(r => {
        setTopicFeeds(r.topics); setDateRange({ start: r.start, end: r.end }); setLoading(false);
      });
    }
  }

  function startEdit(m: Member) { setEditing({ id: m.id, status: m.status, teamId: m.team_id || 0, role: m.role || '' }); }
  function cancelEdit() { setEditing(null); }
  async function saveEdit(m: Member) {
    if (!editing) return;
    setSaving(true);
    try {
      await updateMember(m.id, { status: editing.status, team_id: editing.teamId, role: editing.role || undefined });
      const teamName = teams.find(t => t.id === editing.teamId)?.name || '';
      setMembers(prev => prev.map(x => x.id === m.id ? { ...x, status: editing.status as Member['status'], team_id: editing.teamId, team_name: teamName, role: editing.role } : x));
      setEditing(null);
    } finally { setSaving(false); }
  }

  const presets: Preset[] = ['本周', '上周', '近7天', '近30天'];

  function selectPreset(p: Preset) {
    applyPreset(p);
    // trigger refresh after state update
    const { start, end } = getPresetRange(p);
    if (tab === 'by-member') {
      setLoading(true);
      getFeedByMember(start, end).then(r => {
        setMemberFeeds(r.members); setDateRange({ start: r.start, end: r.end }); setLoading(false);
      });
    } else if (tab === 'by-topic') {
      setLoading(true);
      getFeedByTopic(start, end).then(r => {
        setTopicFeeds(r.topics); setDateRange({ start: r.start, end: r.end }); setLoading(false);
      });
    }
  }

  const DateRangeBar = () => (
    <div className="flex items-center space-x-2 text-sm flex-wrap gap-y-2">
      {presets.map(p => (
        <button key={p} onClick={() => selectPreset(p)}
          className="px-3 py-1.5 rounded-full text-sm transition-colors"
          style={activePreset === p
            ? { background: '#2C2C2C', color: '#fff' }
            : { background: '#F5F0E8', color: '#6B5E4F' }}>
          {p}
        </button>
      ))}
      <span className="text-xs" style={{ color: '#A09484' }}>{startInput} ~ {endInput}</span>
    </div>
  );

  return (
    <div className="h-full overflow-y-auto">
      <div className="max-w-5xl mx-auto w-full px-4 py-8">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold tracking-tight" style={{ color: '#2C2C2C' }}>团队动态</h1>
        <p className="text-sm mt-1" style={{ color: '#8B7E6A' }}>查看团队进度与成员信息。</p>
      </div>

      <div className="flex items-center justify-between mb-6 border-b" style={{ borderColor: '#E5DDD0' }}>
        <div className="flex space-x-1">
          {([['by-member', BarChart2, '按成员'], ['by-topic', Hash, '按 Topic'], ['members', Users, '成员管理']] as const).map(([key, Icon, label]) => (
            <button key={key} onClick={() => setTab(key as Tab)}
              className="flex items-center space-x-2 px-4 py-2 text-sm font-medium border-b-2 transition-colors"
              style={tab === key ? { borderColor: '#2C2C2C', color: '#2C2C2C' } : { borderColor: 'transparent', color: '#8B7E6A' }}>
              <Icon size={15} /><span>{label}</span>
            </button>
          ))}
        </div>
        <button onClick={async () => {
          const token = localStorage.getItem('token');
          const res = await fetch('/api/export/daily', { headers: { Authorization: `Bearer ${token}` } });
          const blob = await res.blob();
          const a = document.createElement('a');
          a.href = URL.createObjectURL(blob);
          a.download = decodeURIComponent(res.headers.get('Content-Disposition')?.match(/filename\*=UTF-8''(.+)/)?.[1] || '日报导出.xlsx');
          a.click(); URL.revokeObjectURL(a.href);
        }} className="flex items-center space-x-1 px-3 py-1.5 text-sm rounded-md transition-colors mb-1"
          style={{ color: '#6b7280', border: '1px solid #d1d5db' }}
          onMouseEnter={e => { e.currentTarget.style.background = '#f3f4f6'; }}
          onMouseLeave={e => { e.currentTarget.style.background = 'transparent'; }}>
          <Download size={14} /><span>导出日报</span>
        </button>
      </div>

      {/* 按成员 */}
      {tab === 'by-member' && (
        <div className="space-y-4">
          <DateRangeBar />
          {loading ? <LoadingSkeleton /> : memberFeeds.length === 0 ? (
            <Empty text="该时间段暂无日报数据" />
          ) : memberFeeds.map(mf => (
            <div key={mf.member_id} className="rounded-xl overflow-hidden" style={{ background: '#FFFCF8', border: '1px solid #E5DDD0' }}>
              <div className="px-4 py-3 font-medium text-sm" style={{ background: '#F5F0E8', color: '#2C2C2C' }}>
                {mf.member_name}
                <span className="ml-2 text-xs font-normal" style={{ color: '#8B7E6A' }}>{mf.items.length} 条记录</span>
              </div>
              <div className="divide-y" style={{ borderColor: '#F0EBE3' }}>
                {mf.items.map((item, i) => (
                  <div key={i} className="px-4 py-3 text-sm">
                    <div className="flex items-start justify-between">
                      <div className="flex-1">
                        <span className="text-xs font-mono mr-2" style={{ color: '#A09484' }}>{item.daily_date?.slice(0, 10)}</span>
                        <span style={{ color: '#2C2C2C' }}>{item.summary}</span>
                      </div>
                      {item.risk && (
                        <span className="ml-2 shrink-0 text-xs px-2 py-0.5 rounded-full bg-red-50 text-red-600">{item.risk}</span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* 按 Topic */}
      {tab === 'by-topic' && (
        <div className="space-y-4">
          <DateRangeBar />
          {loading ? <LoadingSkeleton /> : topicFeeds.length === 0 ? (
            <Empty text="该时间段暂无 Topic 数据（需先提交日报以提取 Topic）" />
          ) : topicFeeds.map(tf => (
            <div key={tf.topic} className="rounded-xl overflow-hidden" style={{ background: '#FFFCF8', border: '1px solid #E5DDD0' }}>
              <div className="px-4 py-3 flex items-center justify-between" style={{ background: '#F5F0E8' }}>
                <div>
                  <span className="font-medium text-sm" style={{ color: '#2C2C2C' }}>{tf.topic}</span>
                  <span className="ml-2 text-xs" style={{ color: '#8B7E6A' }}>{tf.items.length} 条活动</span>
                </div>
                <div className="flex space-x-1">
                  {tf.members.map(name => (
                    <span key={name} className="text-xs px-2 py-0.5 rounded-full" style={{ background: '#E5DDD0', color: '#6B5E4F' }}>{name}</span>
                  ))}
                </div>
              </div>
              <div className="divide-y" style={{ borderColor: '#F0EBE3' }}>
                {tf.items.map((item, i) => (
                  <div key={i} className="px-4 py-3 text-sm">
                    <span className="text-xs font-mono mr-2" style={{ color: '#A09484' }}>{item.daily_date?.slice(0, 10)}</span>
                    <span className="text-xs mr-2 font-medium" style={{ color: '#6B5E4F' }}>{item.member_name}</span>
                    <span style={{ color: '#2C2C2C' }}>{item.content}</span>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* 成员管理 */}
      {tab === 'members' && (
        <div>
          {loading ? <LoadingSkeleton /> : (
            <div className="rounded-xl overflow-hidden" style={{ background: '#FFFFFF', border: '1px solid #E5DDD0' }}>
              <table className="w-full text-sm">
                <thead>
                  <tr style={{ background: '#F5F0E8', borderBottom: '1px solid #E5DDD0' }}>
                    <th className="text-left px-4 py-3 font-medium" style={{ color: '#6B5E4F' }}>姓名</th>
                    <th className="text-left px-4 py-3 font-medium" style={{ color: '#6B5E4F' }}>职位</th>
                    <th className="text-left px-4 py-3 font-medium" style={{ color: '#6B5E4F' }}>团队</th>
                    <th className="text-left px-4 py-3 font-medium" style={{ color: '#6B5E4F' }}>状态</th>
                    <th className="px-4 py-3" />
                  </tr>
                </thead>
                <tbody className="divide-y" style={{ borderColor: '#F0EBE3' }}>
                  {members.map(m => {
                    const isEditing = editing?.id === m.id;
                    const statusInfo = STATUS_LABELS[m.status] || STATUS_LABELS.active;
                    return (
                      <tr key={m.id} className="transition-colors" style={{ cursor: 'default' }}
                        onMouseEnter={e => e.currentTarget.style.background = '#FAF7F2'}
                        onMouseLeave={e => e.currentTarget.style.background = 'transparent'}>
                        <td className="px-4 py-3 font-medium" style={{ color: '#2C2C2C' }}>{m.name}</td>
                        <td className="px-4 py-3">
                          {isEditing ? (
                            <select className="rounded px-2 py-1 text-sm focus:outline-none" style={{ border: '1px solid #E5DDD0' }}
                              value={editing.role} onChange={e => setEditing(prev => prev ? { ...prev, role: e.target.value } : prev)}>
                              <option value="">未设置</option>
                              {ROLE_OPTIONS.map(r => <option key={r} value={r}>{r}</option>)}
                            </select>
                          ) : <span style={{ color: '#8B7E6A' }}>{m.role || '—'}</span>}
                        </td>
                        <td className="px-4 py-3">
                          {isEditing ? (
                            <select className="rounded px-2 py-1 text-sm focus:outline-none" style={{ border: '1px solid #E5DDD0' }}
                              value={editing.teamId} onChange={e => {
                                if (e.target.value === '__new__') setTeamModal(true);
                                else setEditing(prev => prev ? { ...prev, teamId: Number(e.target.value) } : prev);
                              }}>
                              <option value={0}>未分配</option>
                              {teams.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
                              <option value="__new__">＋ 新建团队...</option>
                            </select>
                          ) : <span style={{ color: '#8B7E6A' }}>{m.team_name || '—'}</span>}
                        </td>
                        <td className="px-4 py-3">
                          {isEditing ? (
                            <select className="rounded px-2 py-1 text-sm focus:outline-none" style={{ border: '1px solid #E5DDD0' }}
                              value={editing.status} onChange={e => setEditing(prev => prev ? { ...prev, status: e.target.value } : prev)}>
                              <option value="active">在职</option><option value="resigned">离职</option><option value="transferred">转岗</option>
                            </select>
                          ) : <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${statusInfo.color}`}>{statusInfo.label}</span>}
                        </td>
                        <td className="px-4 py-3 text-right">
                          {isEditing ? (
                            <div className="flex items-center justify-end space-x-2">
                              <button onClick={() => saveEdit(m)} disabled={saving} className="p-1 text-green-600 hover:text-green-700 disabled:opacity-50" title="保存"><Check size={16} /></button>
                              <button onClick={cancelEdit} className="p-1 transition-colors" style={{ color: '#A09484' }} title="取消"><X size={16} /></button>
                            </div>
                          ) : (
                            <div className="flex items-center justify-end space-x-1">
                              <button onClick={() => startEdit(m)} className="p-1 transition-colors" style={{ color: '#A09484' }} title="编辑"><Pencil size={15} /></button>
                              <button onClick={() => setDeleteTarget(m)} className="p-1 transition-colors hover:text-red-500" style={{ color: '#A09484' }} title="删除"><Trash2 size={15} /></button>
                            </div>
                          )}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
              {members.length === 0 && <Empty text="暂无成员数据" />}
            </div>
          )}
        </div>
      )}

      <ConfirmModal open={!!deleteTarget} title="删除成员" message={`确定要删除「${deleteTarget?.name}」吗？该操作不可撤销。`}
        confirmText="删除" danger onConfirm={async () => {
          if (!deleteTarget) return;
          await deleteMember(deleteTarget.id);
          setMembers(prev => prev.filter(x => x.id !== deleteTarget.id));
          setDeleteTarget(null);
        }} onCancel={() => setDeleteTarget(null)} />
      <ConfirmModal open={teamModal} title="新建团队" inputMode inputPlaceholder="输入团队名称" confirmText="创建"
        onConfirm={async (name) => {
          if (!name) return;
          const t = await createTeam(name);
          setTeams(prev => [...prev, t]);
          setEditing(prev => prev ? { ...prev, teamId: t.id } : prev);
          setTeamModal(false);
        }} onCancel={() => setTeamModal(false)} />
    </div>
    </div>
  );
}

function LoadingSkeleton() {
  return <div className="space-y-3">{[1,2,3,4].map(i => <div key={i} className="animate-pulse h-14 rounded-lg" style={{ background: '#F0EBE3' }} />)}</div>;
}

function Empty({ text }: { text: string }) {
  return <div className="text-sm text-center py-16" style={{ color: '#A09484' }}>{text}</div>;
}
