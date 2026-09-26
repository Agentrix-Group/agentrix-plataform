import React, { useState } from 'react';
import { Swords, Eye, RotateCcw, Terminal, RefreshCw, Play, Filter, Download } from 'lucide-react';
import { Match, Arena } from '../types';
import { StatusBadge } from '../components/StatusBadge';

interface MatchesPageProps {
  matches: Match[];
  arenas: Arena[];
  loading: boolean;
  onRefresh: () => void;
  onSelectMatch: (matchId: number) => void;
  onRerunMatch: (matchId: number) => void;
  onTriggerMatch: () => void;
}

export const MatchesPage: React.FC<MatchesPageProps> = ({
  matches,
  arenas,
  loading,
  onRefresh,
  onSelectMatch,
  onRerunMatch,
  onTriggerMatch,
}) => {
  const [selectedArenaFilter, setSelectedArenaFilter] = useState<string>('');
  const [selectedStatusFilter, setSelectedStatusFilter] = useState<string>('');

  const filteredMatches = matches.filter((m) => {
    if (selectedArenaFilter && m.arena_id.toString() !== selectedArenaFilter) return false;
    if (selectedStatusFilter && m.status !== selectedStatusFilter) return false;
    return true;
  });

  return (
    <div className="space-y-4">
      {/* Top Banner Bar */}
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 shadow-sm flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Swords className="w-5 h-5 text-sky-400" />
          <h1 className="text-base font-bold text-slate-100 uppercase tracking-wide">
            Match History & Simulations
          </h1>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={onRefresh}
            disabled={loading}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            <span>Refresh</span>
          </button>
          <button
            onClick={onTriggerMatch}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-semibold bg-emerald-600 hover:bg-emerald-500 text-white shadow-sm transition"
          >
            <Play className="w-3.5 h-3.5" />
            <span>Launch New Match</span>
          </button>
        </div>
      </div>

      {/* Filter and Table Container */}
      <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden shadow-sm">
        {/* Filters Toolbar */}
        <div className="px-4 py-3 border-b border-slate-800 bg-slate-950/60 flex flex-wrap items-center justify-between gap-3 text-xs">
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-1.5 text-slate-400">
              <Filter className="w-3.5 h-3.5" />
              <span>Filters:</span>
            </div>

            <select
              value={selectedArenaFilter}
              onChange={(e) => setSelectedArenaFilter(e.target.value)}
              className="bg-slate-900 border border-slate-800 rounded px-2.5 py-1 text-slate-200 focus:outline-none focus:border-sky-500"
            >
              <option value="">All Arenas</option>
              {arenas.map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </select>

            <select
              value={selectedStatusFilter}
              onChange={(e) => setSelectedStatusFilter(e.target.value)}
              className="bg-slate-900 border border-slate-800 rounded px-2.5 py-1 text-slate-200 focus:outline-none focus:border-sky-500"
            >
              <option value="">All Statuses</option>
              <option value="finished">Finished</option>
              <option value="running">Running</option>
              <option value="failed">Failed</option>
            </select>
          </div>

          <div className="text-slate-400 font-mono">
            Showing {filteredMatches.length} matches
          </div>
        </div>

        {/* DOMjudge-style Dense Table */}
        <div className="overflow-x-auto">
          <table className="dj-table">
            <thead>
              <tr>
                <th className="w-16 text-center">ID</th>
                <th>Time / Date</th>
                <th>Arena</th>
                <th>Seed</th>
                <th>Finishing Placement (1st .. 5th)</th>
                <th className="text-right">Duration</th>
                <th className="text-center">Status</th>
                <th className="text-right pr-4">Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredMatches.length === 0 ? (
                <tr>
                  <td colSpan={8} className="text-center py-8 text-slate-500">
                    No matches found matching criteria.
                  </td>
                </tr>
              ) : (
                filteredMatches.map((match) => {
                  const sortedParticipants = [...(match.participants || [])].sort(
                    (a, b) => a.rank_place - b.rank_place
                  );

                  return (
                    <tr key={match.id}>
                      {/* ID */}
                      <td className="text-center font-mono font-bold text-sky-400">
                        #{match.id}
                      </td>

                      {/* Time */}
                      <td className="font-mono text-slate-400 text-[11px]">
                        {new Date(match.created_at).toLocaleTimeString([], {
                          hour: '2-digit',
                          minute: '2-digit',
                          second: '2-digit',
                        })}
                      </td>

                      {/* Arena */}
                      <td className="text-slate-300 font-medium">
                        {match.arena_name || 'Battle Royale 5P'}
                      </td>

                      {/* Seed */}
                      <td className="font-mono text-slate-400 text-[11px]">
                        {match.seed}
                      </td>

                      {/* 5 Bot Placements */}
                      <td>
                        <div className="flex items-center gap-1.5 flex-wrap">
                          {sortedParticipants.map((p, pIdx) => {
                            const isWin = p.rank_place === 1 && !p.disqualified;
                            return (
                              <span
                                key={pIdx}
                                title={`${p.rank_place}º: ${p.agent_name} (${p.team_name}) - Score: ${p.score}`}
                                className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] font-mono border ${
                                  p.disqualified
                                    ? 'bg-rose-950/60 border-rose-800 text-rose-300 line-through'
                                    : isWin
                                    ? 'bg-amber-950/80 border-amber-600/80 text-amber-300 font-bold'
                                    : 'bg-slate-800 border-slate-700 text-slate-300'
                                }`}
                              >
                                <span>{p.rank_place > 0 ? `${p.rank_place}º` : `P${p.seat}`}</span>
                                <span className="max-w-[70px] truncate">{p.agent_name}</span>
                              </span>
                            );
                          })}
                        </div>
                      </td>

                      {/* Duration */}
                      <td className="text-right font-mono text-slate-300">
                        {match.ticks_played > 0 ? (
                          <>
                            <span>{match.ticks_played} ticks</span>
                            <span className="text-[10px] text-slate-500 ml-1">
                              ({((match.ticks_played * 50) / 1000).toFixed(1)}s)
                            </span>
                          </>
                        ) : (
                          <span className="text-slate-500">-</span>
                        )}
                      </td>

                      {/* Status */}
                      <td className="text-center">
                        <StatusBadge status={match.status} />
                      </td>

                      {/* Actions */}
                      <td className="text-right pr-4">
                        <div className="flex items-center justify-end gap-1.5">
                          {match.status === 'finished' && (
                            <button
                              onClick={() => onSelectMatch(match.id)}
                              className="inline-flex items-center gap-1 px-2 py-1 rounded bg-sky-950/80 hover:bg-sky-900 border border-sky-800/80 text-sky-300 text-[11px] font-medium transition"
                            >
                              <Eye className="w-3 h-3" />
                              <span>2D Replay</span>
                            </button>
                          )}

                          <button
                            onClick={() => onRerunMatch(match.id)}
                            title="Re-run simulation with identical seed and bots"
                            className="p-1 rounded hover:bg-slate-800 text-slate-400 hover:text-slate-200 transition"
                          >
                            <RotateCcw className="w-3.5 h-3.5" />
                          </button>
                        </div>
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
