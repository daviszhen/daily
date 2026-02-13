import React, { useEffect, useState } from 'react';
import { DailyReport } from '../types';
import { getTeamReports } from '../services/apiService';
import { Calendar, Filter, Search, ChevronRight } from 'lucide-react';

export function DailyFeed(): React.ReactElement {
  const [reports, setReports] = useState<DailyReport[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getTeamReports().then(data => {
      setReports(data);
      setLoading(false);
    });
  }, []);

  return (
    <div className="flex flex-col h-full max-w-5xl mx-auto w-full px-4 py-8">
      <div className="flex flex-col md:flex-row md:items-center justify-between mb-8 gap-4">
        <div>
           <h1 className="text-2xl font-semibold text-gray-900 tracking-tight">团队动态</h1>
           <p className="text-gray-500 text-sm mt-1">实时查看研发团队进度。</p>
        </div>
        
        <div className="flex space-x-2">
           <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" size={16} />
              <input 
                type="text" 
                placeholder="搜索动态..." 
                className="pl-9 pr-4 py-2 bg-white border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-brand-100 focus:border-brand-300 w-full md:w-64"
              />
           </div>
           <button className="flex items-center space-x-2 px-3 py-2 bg-white border border-gray-200 rounded-lg text-sm text-gray-600 hover:bg-gray-50">
             <Filter size={16} />
             <span className="hidden sm:inline">筛选</span>
           </button>
           <button className="flex items-center space-x-2 px-3 py-2 bg-white border border-gray-200 rounded-lg text-sm text-gray-600 hover:bg-gray-50">
             <Calendar size={16} />
             <span className="hidden sm:inline">日期</span>
           </button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto -mx-4 px-4 pb-10">
        {loading ? (
            <div className="space-y-4">
               {[1, 2, 3].map(i => (
                 <div key={i} className="animate-pulse bg-white p-6 rounded-xl border border-gray-100">
                    <div className="flex items-center space-x-4 mb-4">
                        <div className="w-10 h-10 bg-gray-200 rounded-full"></div>
                        <div className="space-y-2">
                             <div className="h-4 bg-gray-200 rounded w-32"></div>
                             <div className="h-3 bg-gray-200 rounded w-24"></div>
                        </div>
                    </div>
                    <div className="h-4 bg-gray-200 rounded w-3/4 mb-2"></div>
                    <div className="h-4 bg-gray-200 rounded w-1/2"></div>
                 </div>
               ))}
            </div>
        ) : (
            <div className="space-y-4">
               {reports.map((report) => (
                  <div key={report.id} className="group bg-white rounded-xl p-6 border border-gray-100 hover:border-brand-200 hover:shadow-sm transition-all duration-200">
                     <div className="flex items-start justify-between">
                        <div className="flex items-center space-x-3 mb-3">
                           <img src={report.userAvatar} alt={report.userName} className="w-10 h-10 rounded-full bg-gray-100 object-cover border border-white shadow-sm" />
                           <div>
                              <h3 className="text-sm font-semibold text-gray-900">{report.userName}</h3>
                              <span className="text-xs text-gray-500">
                                {new Date(report.timestamp).toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'})}
                              </span>
                           </div>
                        </div>
                        <div className="flex space-x-2">
                           {report.tags.map(tag => (
                               <span key={tag} className="px-2 py-1 text-[10px] font-medium uppercase tracking-wider text-gray-500 bg-gray-50 rounded border border-gray-100">
                                   {tag}
                               </span>
                           ))}
                        </div>
                     </div>
                     
                     <div className="pl-13 text-gray-700 text-base leading-relaxed">
                        {report.content}
                     </div>

                     {report.risks && report.risks.length > 0 && (
                         <div className="mt-4 bg-red-50 border border-red-100 rounded-lg p-3 flex items-start space-x-2">
                            <div className="w-1 h-1 bg-red-500 rounded-full mt-2 flex-shrink-0" />
                            <p className="text-sm text-red-700 font-medium">风险: {report.risks[0]}</p>
                         </div>
                     )}

                     <div className="mt-4 pt-4 border-t border-gray-50 flex items-center justify-end opacity-0 group-hover:opacity-100 transition-opacity">
                        <button className="text-xs text-brand-600 font-medium flex items-center hover:underline">
                            查看详情 <ChevronRight size={12} className="ml-1" />
                        </button>
                     </div>
                  </div>
               ))}
            </div>
        )}
      </div>
    </div>
  );
};
