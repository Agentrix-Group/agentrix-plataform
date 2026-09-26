import React from 'react';
import { Users, Building, Calendar } from 'lucide-react';
import { Team } from '../types';

interface TeamsPageProps {
  teams: Team[];
}

export const TeamsPage: React.FC<TeamsPageProps> = ({ teams }) => {
  return (
    <div className="space-y-4">
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 shadow-sm flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Users className="w-5 h-5 text-sky-400" />
          <h1 className="text-base font-bold text-slate-100 uppercase tracking-wide">
            Participating Teams & Organizations
          </h1>
        </div>
      </div>

      <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="dj-table">
            <thead>
              <tr>
                <th className="w-16 text-center">ID</th>
                <th>Team Name</th>
                <th>Institutional Affiliation</th>
                <th>Registered Date</th>
              </tr>
            </thead>
            <tbody>
              {teams.length === 0 ? (
                <tr>
                  <td colSpan={4} className="text-center py-8 text-slate-500">
                    No teams registered yet.
                  </td>
                </tr>
              ) : (
                teams.map((t) => (
                  <tr key={t.id}>
                    <td className="text-center font-mono font-bold text-slate-400">
                      #{t.id}
                    </td>
                    <td className="font-semibold text-slate-100 flex items-center gap-1.5">
                      <Users className="w-3.5 h-3.5 text-sky-400" />
                      <span>{t.name}</span>
                    </td>
                    <td className="text-slate-300 flex items-center gap-1">
                      <Building className="w-3 h-3 text-slate-500" />
                      <span>{t.affiliation || 'Independent'}</span>
                    </td>
                    <td className="font-mono text-slate-400 text-[11px]">
                      {new Date(t.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
