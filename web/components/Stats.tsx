import React from 'react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts';
import { ArrowUpRight, CheckCircle, AlertTriangle, Clock } from 'lucide-react';

const data = [
  { name: '周一', value: 12 },
  { name: '周二', value: 18 },
  { name: '周三', value: 15 },
  { name: '周四', value: 22 },
  { name: '周五', value: 20 },
];

const StatCard = ({ label, value, icon: Icon, trend, color }: any) => (
  <div className="bg-white p-6 rounded-xl border border-gray-100 shadow-sm">
    <div className="flex items-start justify-between">
       <div>
          <p className="text-sm text-gray-500 font-medium mb-1">{label}</p>
          <h3 className="text-2xl font-bold text-gray-900 tracking-tight">{value}</h3>
       </div>
       <div className={`p-2 rounded-lg ${color}`}>
          <Icon size={20} className="opacity-80" />
       </div>
    </div>
    {trend && (
       <div className="mt-4 flex items-center text-xs font-medium text-green-600">
          <ArrowUpRight size={12} className="mr-1" />
          {trend}
       </div>
    )}
  </div>
);

export function Stats(): React.ReactElement {
  return (
    <div className="max-w-5xl mx-auto w-full px-4 py-8 space-y-8">
       <div>
          <h1 className="text-2xl font-semibold text-gray-900 tracking-tight">周度洞察</h1>
          <p className="text-gray-500 text-sm mt-1">团队效能与风险概览。</p>
       </div>

       <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <StatCard 
             label="已提交日报" 
             value="87" 
             icon={CheckCircle} 
             color="bg-green-50 text-green-600"
             trend="较上周 +12%"
          />
          <StatCard 
             label="识别风险" 
             value="3" 
             icon={AlertTriangle} 
             color="bg-red-50 text-red-600"
          />
          <StatCard 
             label="平均提交时间" 
             value="17:45" 
             icon={Clock} 
             color="bg-blue-50 text-blue-600"
          />
       </div>

       <div className="bg-white p-6 rounded-xl border border-gray-100 shadow-sm">
          <h3 className="text-base font-semibold text-gray-900 mb-6">活跃度趋势</h3>
          <div className="h-64 w-full">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={data}>
                <XAxis 
                    dataKey="name" 
                    axisLine={false} 
                    tickLine={false} 
                    tick={{fill: '#9ca3af', fontSize: 12}} 
                    dy={10}
                />
                <YAxis hide />
                <Tooltip 
                    cursor={{fill: '#f3f4f6'}}
                    contentStyle={{ borderRadius: '8px', border: 'none', boxShadow: '0 4px 6px -1px rgba(0, 0, 0, 0.1)' }}
                />
                <Bar dataKey="value" radius={[4, 4, 4, 4]} barSize={40}>
                  {data.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={index === 3 ? '#111827' : '#e5e7eb'} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
       </div>
    </div>
  );
};
