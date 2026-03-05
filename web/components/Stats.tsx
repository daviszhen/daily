import React, { useEffect, useState } from 'react';
import { getInsights, InsightItem, resolveTopic, updateTopic } from '../services/apiService';
import { AlertTriangle, CheckCircle, Clock, Users, ChevronDown, ChevronUp, Pencil, Check, X } from 'lucide-react';

type RiskFilter = 'all' | 'high' | 'medium' | 'low';
type SortKey = 'days' | 'member_count' | 'entry_count';

const RISK_COLORS: Record<string, { bg: string; text: string; label: string }> = {
  high:   { bg: 'bg-red-50',    text: 'text-red-700',    label: '高风险' },
  medium: { bg: 'bg-yellow-50', text: 'text-yellow-700', label: '中风险' },
  low:    { bg: 'bg-green-50',  text: 'text-green-700',  label: '低风险' },
};
const PAGE_SIZE = 20;

export function Stats(): React.ReactElement {
  const [allInsights, setAllInsights] = useState<InsightItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [filter, setFilter] = useState<RiskFilter>('all');
  const [sortKey, setSortKey] = useState<SortKey>('days');
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [renaming, setRenaming] = useState<number | null>(null);
  const [renameValue, setRenameValue] = useState('');

  useEffect(() => {
    getInsights().then(r => { setAllInsights(r.insights || []); setLoading(false); });
  }, []);

  // Derived
  const filtered = allInsights
    .filter(i => filter === 'all' || i.risk_level === filter)
    .sort((a, b) => (b[sortKey] as number) - (a[sortKey] as number));
  const totalPages = Math.ceil(filtered.length / PAGE_SIZE);
  const paged = filtered.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE);

  const highCount = allInsights.filter(i => i.risk_level === 'high').length;
  const mediumCount = allInsights.filter(i => i.risk_level === 'medium').length;

  function toggleExpand(topic: string) {
    setExpanded(prev => { const s = new Set(prev); s.has(topic) ? s.delete(topic) : s.add(topic); return s; });
  }
  function toggleSelect(id: number) {
    setSelected(prev => { const s = new Set(prev); s.has(id) ? s.delete(id) : s.add(id); return s; });
  }
  function toggleSelectAll() {
    if (selected.size === paged.length) setSelected(new Set());
    else setSelected(new Set(paged.map(i => i.topic_id)));
  }

  async function handleResolve(id: number) {
    await resolveTopic(id);
    setAllInsights(prev => prev.filter(i => i.topic_id !== id));
    setSelected(prev => { const s = new Set(prev); s.delete(id); return s; });
  }
  async function handleBatchResolve() {
    const ids = [...selected];
    await Promise.all(ids.map(id => resolveTopic(id)));
    setAllInsights(prev => prev.filter(i => !selected.has(i.topic_id)));
    setSelected(new Set());
  }
  async function handleRename(id: number) {
    if (!renameValue.trim()) return;
    await updateTopic(id, { name: renameValue.trim() });
    setAllInsights(prev => prev.map(i => i.topic_id === id ? { ...i, topic: renameValue.trim() } : i));
    setRenaming(null);
  }

  // Reset page when filter changes
  useEffect(() => { setPage(0); setSelected(new Set()); }, [filter, sortKey]);

  return (
    <div className="h-full overflow-y-auto">
    <div className="max-w-5xl mx-auto w-full px-4 py-8 space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight" style={{ color: '#2C2C2C' }}>数据洞察</h1>
        <p className="text-sm mt-1" style={{ color: '#8B7E6A' }}>近 90 天活跃 Topic 风险看板。标记已解决的 Topic 将从看板移除。</p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-3 gap-4">
        {[
          { label: '高风险', value: highCount, icon: AlertTriangle, color: 'bg-red-50 text-red-600', f: 'high' as RiskFilter },
          { label: '中风险', value: mediumCount, icon: Clock, color: 'bg-yellow-50 text-yellow-600', f: 'medium' as RiskFilter },
          { label: '活跃总数', value: allInsights.length, icon: CheckCircle, color: 'bg-green-50 text-green-600', f: 'all' as RiskFilter },
        ].map(c => (
          <button key={c.label} onClick={() => setFilter(c.f)}
            className="p-5 rounded-xl text-left transition-shadow"
            style={{ background: filter === c.f ? '#F5F0E8' : '#FFFCF8', border: filter === c.f ? '2px solid #2C2C2C' : '1px solid #E5DDD0' }}>
            <div className="flex items-start justify-between">
              <div>
                <p className="text-sm font-medium mb-1" style={{ color: '#8B7E6A' }}>{c.label}</p>
                <h3 className="text-2xl font-bold tracking-tight" style={{ color: '#2C2C2C' }}>{c.value}</h3>
              </div>
              <div className={`p-2 rounded-lg ${c.color}`}><c.icon size={20} className="opacity-80" /></div>
            </div>
          </button>
        ))}
      </div>

      {/* Toolbar: sort + batch actions */}
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-3 text-sm">
          <span style={{ color: '#8B7E6A' }}>排序：</span>
          {([['days', '活跃天数'], ['member_count', '参与人数'], ['entry_count', '记录数']] as [SortKey, string][]).map(([k, label]) => (
            <button key={k} onClick={() => setSortKey(k)}
              className="px-3 py-1 rounded-full text-sm transition-colors"
              style={sortKey === k ? { background: '#2C2C2C', color: '#fff' } : { background: '#F5F0E8', color: '#6B5E4F' }}>
              {label}
            </button>
          ))}
        </div>
        {selected.size > 0 && (
          <button onClick={handleBatchResolve}
            className="px-4 py-1.5 rounded-lg text-sm font-medium transition-colors"
            style={{ background: '#2C2C2C', color: '#fff' }}>
            批量标记已解决（{selected.size}）
          </button>
        )}
      </div>

      {/* Topic list */}
      {loading ? (
        <div className="space-y-3">{[1,2,3].map(i => <div key={i} className="animate-pulse h-14 rounded-xl" style={{ background: '#F0EBE3' }} />)}</div>
      ) : paged.length === 0 ? (
        <div className="text-sm text-center py-16" style={{ color: '#A09484' }}>
          {filter !== 'all' ? '该风险等级下暂无 Topic' : '暂无 Topic 数据'}
        </div>
      ) : (
        <div className="space-y-2">
          {/* Select all */}
          <label className="flex items-center space-x-2 px-2 text-xs cursor-pointer" style={{ color: '#8B7E6A' }}>
            <input type="checkbox" checked={selected.size === paged.length && paged.length > 0}
              onChange={toggleSelectAll} className="rounded" />
            <span>全选当页</span>
          </label>

          {paged.map(item => {
            const risk = RISK_COLORS[item.risk_level] || RISK_COLORS.low;
            const isExpanded = expanded.has(item.topic);
            const isSelected = selected.has(item.topic_id);
            const isRenaming = renaming === item.topic_id;
            return (
              <div key={item.topic_id} className="rounded-xl overflow-hidden transition-shadow"
                style={{ background: isSelected ? '#F5F0E8' : '#FFFCF8', border: '1px solid #E5DDD0' }}>
                <div className="px-4 py-3 flex items-center justify-between">
                  <div className="flex items-center space-x-3">
                    <input type="checkbox" checked={isSelected} onChange={() => toggleSelect(item.topic_id)}
                      onClick={e => e.stopPropagation()} className="rounded" />
                    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${risk.bg} ${risk.text}`}>
                      {risk.label}
                    </span>
                    {isRenaming ? (
                      <div className="flex items-center space-x-1" onClick={e => e.stopPropagation()}>
                        <input value={renameValue} onChange={e => setRenameValue(e.target.value)}
                          onKeyDown={e => e.key === 'Enter' && handleRename(item.topic_id)}
                          className="text-sm px-2 py-0.5 rounded focus:outline-none" style={{ border: '1px solid #E5DDD0', width: 160 }}
                          autoFocus />
                        <button onClick={() => handleRename(item.topic_id)} className="p-0.5 text-green-600"><Check size={14} /></button>
                        <button onClick={() => setRenaming(null)} className="p-0.5" style={{ color: '#A09484' }}><X size={14} /></button>
                      </div>
                    ) : (
                      <span className="font-medium text-sm cursor-pointer" style={{ color: '#2C2C2C' }}
                        onClick={() => toggleExpand(item.topic)}>{item.topic}</span>
                    )}
                  </div>
                  <div className="flex items-center space-x-3 text-xs" style={{ color: '#8B7E6A' }}>
                    <span className="flex items-center space-x-1"><Clock size={12} /><span>活跃 {item.days} 天</span></span>
                    <span className="flex items-center space-x-1"><Users size={12} /><span>{item.member_count} 人</span></span>
                    <span>{item.entry_count} 条</span>
                    <button onClick={e => { e.stopPropagation(); setRenaming(item.topic_id); setRenameValue(item.topic); }}
                      className="p-1 rounded transition-colors hover:bg-gray-100" title="重命名"><Pencil size={13} /></button>
                    <button onClick={e => { e.stopPropagation(); handleResolve(item.topic_id); }}
                      className="px-2 py-0.5 rounded text-xs transition-colors hover:bg-green-100 text-green-700" title="标记已解决">
                      已解决
                    </button>
                    <button onClick={() => toggleExpand(item.topic)} className="p-0.5">
                      {isExpanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                    </button>
                  </div>
                </div>
                {isExpanded && (
                  <div className="px-4 pb-3 space-y-2" style={{ borderTop: '1px solid #F0EBE3' }}>
                    <div className="pt-2 text-xs" style={{ color: '#8B7E6A' }}>
                      首次: {item.first_date?.slice(0, 10)} → 最近: {item.last_date?.slice(0, 10)}
                    </div>
                    {item.risks && item.risks.length > 0 && (
                      <div className="space-y-1">
                        <div className="text-xs font-medium" style={{ color: '#6B5E4F' }}>关联风险项：</div>
                        {item.risks.map((r, i) => (
                          <div key={i} className="text-xs px-3 py-2 rounded-lg bg-red-50">
                            <span className="font-medium text-red-700">{r.member_name}</span>
                            <span className="mx-1 text-red-400">·</span>
                            <span className="text-red-600">{r.daily_date?.slice(0, 10)}</span>
                            <span className="mx-1 text-red-400">·</span>
                            <span className="text-red-600">{r.risk}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center space-x-2 text-sm pt-2">
          <button onClick={() => setPage(p => Math.max(0, p - 1))} disabled={page === 0}
            className="px-3 py-1 rounded transition-colors disabled:opacity-30"
            style={{ background: '#F5F0E8', color: '#6B5E4F' }}>上一页</button>
          <span style={{ color: '#8B7E6A' }}>{page + 1} / {totalPages}</span>
          <button onClick={() => setPage(p => Math.min(totalPages - 1, p + 1))} disabled={page >= totalPages - 1}
            className="px-3 py-1 rounded transition-colors disabled:opacity-30"
            style={{ background: '#F5F0E8', color: '#6B5E4F' }}>下一页</button>
        </div>
      )}
    </div>
    </div>
  );
}
