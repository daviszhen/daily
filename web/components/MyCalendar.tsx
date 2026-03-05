import React, { useEffect, useState, useCallback } from 'react';
import { getCalendar, getDaySummary, CalendarDay, DaySummary } from '../services/apiService';
import { ChevronLeft, ChevronRight, FileText, AlertTriangle, Pencil } from 'lucide-react';

interface Props {
  onSupplement: (date: string) => void;
}

const WEEKDAYS = ['日', '一', '二', '三', '四', '五', '六'];

export function MyCalendar({ onSupplement }: Props): React.ReactElement {
  const [month, setMonth] = useState(() => {
    const now = new Date();
    return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
  });
  const [days, setDays] = useState<CalendarDay[]>([]);
  const [workdays, setWorkdays] = useState(0);
  const [filled, setFilled] = useState(0);
  const [selected, setSelected] = useState<string | null>(null);
  const [daySummary, setDaySummary] = useState<DaySummary | null>(null);
  const [loadingSummary, setLoadingSummary] = useState(false);

  const today = (() => {
    const now = new Date();
    return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
  })();

  useEffect(() => {
    getCalendar(month).then(d => {
      setDays(d.days);
      setWorkdays(d.workdays);
      setFilled(d.filled_workdays);
      setSelected(null);
      setDaySummary(null);
    });
  }, [month]);

  const handleDayClick = useCallback((date: string, isFuture: boolean) => {
    if (isFuture) return;
    if (date === selected) {
      setSelected(null);
      setDaySummary(null);
      return;
    }
    setSelected(date);
    setDaySummary(null);
    setLoadingSummary(true);
    getDaySummary(date).then(s => {
      setDaySummary(s);
      setLoadingSummary(false);
    }).catch(() => setLoadingSummary(false));
  }, [selected]);

  function changeMonth(delta: number) {
    const [y, m] = month.split('-').map(Number);
    const d = new Date(y, m - 1 + delta, 1);
    setMonth(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`);
  }

  const firstWeekday = days.length > 0 ? days[0].weekday : 0;
  const grid: (CalendarDay | null)[] = Array(firstWeekday).fill(null).concat(days);
  const rate = workdays > 0 ? Math.round((filled / workdays) * 100) : 100;
  const selectedDay = days.find(d => d.date === selected);

  return (
    <div className="h-full overflow-y-auto" style={{ background: '#FAF9F6' }}>
      <div className="max-w-2xl mx-auto px-4 py-8">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <h1 className="text-xl font-semibold" style={{ color: '#2C2C2C' }}>我的日历</h1>
          <div className="flex items-center space-x-3">
            <button onClick={() => changeMonth(-1)} className="p-2 rounded-lg hover:bg-[#E5DDD0] transition-all active:scale-95 cursor-pointer" style={{ color: '#6B5E4F' }}>
              <ChevronLeft size={20} />
            </button>
            <span className="text-base font-medium min-w-[120px] text-center select-none" style={{ color: '#2C2C2C' }}>
              {month.replace('-', ' 年 ')} 月
            </span>
            <button onClick={() => changeMonth(1)} className="p-2 rounded-lg hover:bg-[#E5DDD0] transition-all active:scale-95 cursor-pointer" style={{ color: '#6B5E4F' }}>
              <ChevronRight size={20} />
            </button>
          </div>
        </div>

        {/* Stats bar */}
        <div className="rounded-xl px-5 py-3.5 mb-6 flex items-center justify-between" style={{ background: '#F5F0E8' }}>
          <div className="flex items-center gap-2">
            <FileText size={16} style={{ color: '#8B7E6A' }} />
            <span style={{ color: '#6B5E4F' }}>
              工作日 <b className="text-base">{filled}</b> / {workdays} 天已提交
            </span>
          </div>
          <div className="flex items-center gap-2">
            <div className="w-20 h-2 rounded-full overflow-hidden" style={{ background: '#E5DDD0' }}>
              <div className="h-full rounded-full transition-all duration-500" style={{
                width: `${rate}%`,
                background: rate === 100 ? '#5A9A6A' : rate >= 80 ? '#B8A040' : '#C07050',
              }} />
            </div>
            <span className="text-sm font-semibold min-w-[36px] text-right" style={{ color: rate === 100 ? '#5A9A6A' : rate >= 80 ? '#B8A040' : '#C07050' }}>
              {rate}%
            </span>
          </div>
        </div>

        {/* Calendar grid */}
        <div className="rounded-xl overflow-hidden border shadow-sm" style={{ borderColor: '#E5DDD0', background: '#FFFFFF' }}>
          {/* Weekday header */}
          <div className="grid grid-cols-7" style={{ background: '#FAFAF7' }}>
            {WEEKDAYS.map((w, i) => (
              <div key={w} className="py-2.5 text-center text-xs font-semibold tracking-wide" style={{ color: i === 0 || i === 6 ? '#C07050' : '#8B7E6A' }}>{w}</div>
            ))}
          </div>

          {/* Day cells */}
          <div className="grid grid-cols-7">
            {grid.map((day, i) => {
              if (!day) return <div key={`e-${i}`} className="aspect-square border-t" style={{ borderColor: '#F5F0E8', background: '#FDFCFA' }} />;

              const isFuture = day.date > today;
              const isToday = day.date === today;
              const isSelected = day.date === selected;
              const missedWorkday = day.is_workday && !day.submitted && !isFuture && !isToday;
              const isWeekend = day.weekday === 0 || day.weekday === 6;

              return (
                <div
                  key={day.date}
                  onClick={() => handleDayClick(day.date, isFuture)}
                  className={`aspect-square border-t flex flex-col items-center justify-center relative transition-all duration-150
                    ${isFuture ? 'opacity-30' : 'cursor-pointer'}
                    ${isSelected ? 'z-10' : ''}
                  `}
                  style={{
                    borderColor: '#F5F0E8',
                    background: isSelected ? '#F5F0E8' : isFuture ? '#FDFCFA' : 'transparent',
                    boxShadow: isSelected ? 'inset 0 0 0 2px #8B7E6A' : 'none',
                    borderRadius: isSelected ? '8px' : '0',
                  }}
                >
                  <span
                    className={`text-sm font-medium leading-none flex items-center justify-center
                      ${isToday ? 'w-7 h-7 rounded-full text-white' : ''}
                    `}
                    style={{
                      color: isToday ? '#FFF' : isWeekend && !day.is_workday ? '#C07050' : day.is_workday ? '#2C2C2C' : '#A09484',
                      background: isToday ? '#2C2C2C' : 'transparent',
                    }}
                  >
                    {parseInt(day.date.slice(8))}
                  </span>

                  {day.holiday && (
                    <span className="text-[9px] leading-tight mt-0.5 truncate max-w-full px-0.5 font-medium" style={{ color: day.is_workday ? '#B8A040' : '#C07050' }}>
                      {day.holiday.length > 3 ? day.holiday.slice(0, 3) + '..' : day.holiday}
                    </span>
                  )}

                  {/* Status indicator */}
                  {!isFuture && !isToday && day.submitted && (
                    <span className="absolute bottom-1.5 w-1.5 h-1.5 rounded-full" style={{ background: '#5A9A6A' }} />
                  )}
                  {missedWorkday && (
                    <span className="absolute bottom-1.5 w-1.5 h-1.5 rounded-full animate-pulse" style={{ background: '#D06050' }} />
                  )}
                </div>
              );
            })}
          </div>
        </div>

        {/* Legend */}
        <div className="flex items-center space-x-5 mt-3 text-xs" style={{ color: '#A09484' }}>
          <span className="flex items-center gap-1.5"><span className="w-2 h-2 rounded-full" style={{ background: '#5A9A6A' }} />已提交</span>
          <span className="flex items-center gap-1.5"><span className="w-2 h-2 rounded-full" style={{ background: '#D06050' }} />未提交</span>
          <span className="flex items-center gap-1.5"><span className="w-3.5 h-3.5 rounded-full bg-[#2C2C2C] text-white text-[8px] flex items-center justify-center">5</span>今天</span>
        </div>

        {/* Selected day detail panel */}
        {selected && selected <= today && (
          <div className="mt-5 rounded-xl overflow-hidden border transition-all duration-300 animate-in" style={{ borderColor: '#E5DDD0', background: '#FFFFFF' }}>
            {/* Detail header */}
            <div className="px-5 py-3 flex items-center justify-between" style={{ background: '#FAFAF7', borderBottom: '1px solid #F0EBE3' }}>
              <div className="flex items-center gap-2">
                <span className="font-medium" style={{ color: '#2C2C2C' }}>{selected}</span>
                <span className="text-xs px-2 py-0.5 rounded-full" style={{
                  background: selectedDay?.submitted ? '#EDF7EF' : selectedDay?.is_workday ? '#FEF2F0' : '#F5F0E8',
                  color: selectedDay?.submitted ? '#3D7A4A' : selectedDay?.is_workday ? '#C05040' : '#8B7E6A',
                }}>
                  {selectedDay?.holiday || (selectedDay?.is_workday ? '工作日' : '休息日')}
                  {selectedDay?.submitted ? ' · 已提交' : ' · 未提交'}
                </span>
              </div>
              <button
                onClick={() => onSupplement(selected)}
                className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg text-sm font-medium text-white transition-all hover:opacity-90 active:scale-95 cursor-pointer"
                style={{ background: '#2C2C2C' }}
              >
                <Pencil size={13} />
                {selectedDay?.submitted ? '重新填写' : '去补填'}
              </button>
            </div>

            {/* Summary content */}
            {loadingSummary && (
              <div className="px-5 py-8 flex justify-center">
                <div className="flex items-center gap-2 text-sm" style={{ color: '#A09484' }}>
                  <div className="w-4 h-4 border-2 border-current border-t-transparent rounded-full animate-spin" />
                  加载中...
                </div>
              </div>
            )}

            {!loadingSummary && daySummary?.submitted && daySummary.summary && (
              <div className="px-5 py-4 space-y-3">
                <div className="text-sm leading-relaxed whitespace-pre-wrap" style={{ color: '#4B4539' }}>
                  {daySummary.summary}
                </div>
                {daySummary.risk && (
                  <div className="flex items-start gap-2 px-3 py-2 rounded-lg text-sm" style={{ background: '#FEF8F0', color: '#B8862D' }}>
                    <AlertTriangle size={14} className="mt-0.5 flex-shrink-0" />
                    <span>{daySummary.risk}</span>
                  </div>
                )}
              </div>
            )}

            {!loadingSummary && (!daySummary || !daySummary.submitted) && (
              <div className="px-5 py-6 text-center text-sm" style={{ color: '#A09484' }}>
                当天暂无日报记录
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
