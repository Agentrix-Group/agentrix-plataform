import React from 'react';
import { Users, Building, Calendar, School, Award } from 'lucide-react';
import { Team, getDeterministicColor } from '../types';

interface TeamsPageProps {
  teams: Team[];
}

export const TeamsPage: React.FC<TeamsPageProps> = ({ teams }) => {
  return (
    <div className="space-y-4 animate-fade-in">
      {/* Top Banner Bar */}
      <div className="bg-white border border-slate-200 rounded-xl p-4 sm:p-5 shadow-sm flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-purple-50 border border-purple-100 flex items-center justify-center text-purple-600 shadow-xs">
            <Users className="w-5 h-5" />
          </div>
          <div>
            <h1 className="text-base font-black text-slate-900 tracking-tight uppercase">
              Participating Teams & Academic Affiliations
            </h1>
            <p className="text-xs text-slate-500 font-medium">
              Registered collegiate teams, research laboratories, and algorithmic clubs
            </p>
          </div>
        </div>
      </div>

      {/* KPI Stats */}
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
        <div className="bg-white border border-slate-200 rounded-xl p-3 shadow-xs">
          <div className="text-[10px] uppercase font-bold text-slate-400">Total Teams</div>
          <div className="text-lg font-black text-slate-900 font-mono mt-0.5">{teams.length}</div>
        </div>
        <div className="bg-white border border-slate-200 rounded-xl p-3 shadow-xs">
          <div className="text-[10px] uppercase font-bold text-purple-600">Collegiate Affiliations</div>
          <div className="text-lg font-black text-purple-700 font-mono mt-0.5">
            {new Set(teams.map((t) => t.affiliation || 'Independent')).size}
          </div>
        </div>
        <div className="bg-white border border-slate-200 rounded-xl p-3 shadow-xs col-span-2 sm:col-span-1">
          <div className="text-[10px] uppercase font-bold text-emerald-600">Eligibility Status</div>
          <div className="text-lg font-black text-emerald-700 font-mono mt-0.5">100% Certified</div>
        </div>
      </div>

      {/* Teams High-Density Table */}
      <div className="bg-white border border-slate-200 rounded-xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="dj-table">
            <thead>
              <tr>
                <th className="w-16 text-center">ID</th>
                <th>Team / Delegation</th>
                <th>Institutional Affiliation</th>
                <th className="text-right pr-6">Registered Date</th>
              </tr>
            </thead>
            <tbody>
              {teams.length === 0 ? (
                <tr>
                  <td colSpan={4} className="text-center py-12 text-slate-400 font-medium">
                    No teams registered yet.
                  </td>
                </tr>
              ) : (
                teams.map((t) => {
                  const teamColor = getDeterministicColor(t.name);
                  return (
                    <tr key={t.id} className="hover:bg-blue-50/70 transition-colors">
                      <td className="text-center font-mono font-bold text-slate-500 text-xs">
                        #{t.id}
                      </td>
                      <td>
                        <div className="flex items-center gap-2.5">
                          <span
                            className={`w-7 h-7 rounded-full text-xs font-black flex items-center justify-center shadow-xs shrink-0 ${teamColor.bgClass}`}
                          >
                            {t.name.charAt(0).toUpperCase()}
                          </span>
                          <div>
                            <span className="font-bold text-slate-900 text-xs block">{t.name}</span>
                          </div>
                        </div>
                      </td>
                      <td>
                        <div className="flex items-center gap-1.5 text-slate-700 text-xs">
                          <Building className="w-3.5 h-3.5 text-slate-400 shrink-0" />
                          <span className="font-medium">{t.affiliation || 'Independent Organization'}</span>
                        </div>
                      </td>
                      <td className="text-right pr-6 font-mono text-slate-500 text-xs">
                        {new Date(t.created_at).toLocaleDateString(undefined, {
                          year: 'numeric',
                          month: 'short',
                          day: 'numeric',
                        })}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
