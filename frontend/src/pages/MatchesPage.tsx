import React, { useEffect, useRef, useState } from 'react';
import { api } from '../api/client';
import { Swords, Eye, RotateCcw, Terminal, RefreshCw, Play, Filter, Download } from 'lucide-react';
import { Match, Arena } from '../types';
import { StatusBadge } from '../components/StatusBadge';

interface MatchesPageProps {
  matches: Match[];
  arenas: Arena[];
  arenaId: number;
  loading: boolean;
  onRefresh: () => void;
  onSelectMatch: (matchId: number) => void;
  onRerunMatch: (matchId: number) => void;
  onTriggerMatch: () => void;
}

export const MatchesPage: React.FC<MatchesPageProps> = ({
  matches,
  arenas,
  arenaId,
  loading,
  onRefresh,
  onSelectMatch,
  onRerunMatch,
  onTriggerMatch,
}) => {
  const [selectedArenaFilter, setSelectedArenaFilter] = useState<string>('');
  const [selectedStatusFilter, setSelectedStatusFilter] = useState<string>('');
  const [olderMatches, setOlderMatches] = useState<Match[]>([]);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const [hasOlder, setHasOlder] = useState(matches.length === 50);
  const [pageError, setPageError] = useState('');
  const generation = useRef(0);
  useEffect(() => {
    generation.current++;
    setOlderMatches([]);
    setHasOlder(matches.length === 50);
    setLoadingOlder(false);
    setPageError('');
    return () => { generation.current++; };
  }, [matches, arenaId]);

  const loadOlder = async () => {
    const currentGeneration = generation.current;
    const all = [...matches, ...olderMatches];
    if (!all.length || loadingOlder) return;
    setLoadingOlder(true);
    setPageError('');
    try {
      const page = await api.getMatches(arenaId, undefined, all[all.length - 1].id);
      if (generation.current !== currentGeneration) return;
      setOlderMatches(previous => [...previous, ...page]);
      setHasOlder(page.length === 50);
    } catch {
      if (generation.current === currentGeneration) setPageError('Could not load older matches. Please retry.');
    } finally {
      if (generation.current === currentGeneration) setLoadingOlder(false);
    }
  };

  const filteredMatches = [...matches, ...olderMatches].filter((m) => {
    if (selectedArenaFilter && m.arena_id.toString() !== selectedArenaFilter) return false;
    if (selectedStatusFilter && m.status !== selectedStatusFilter) return false;
    return true;
  });

  return (
    <div className="space-y-5 animate-fade-in">
      {/* Top Banner Bar */}
      <div className="bg-white border border-slate-200/90 rounded-2xl p-5 shadow-xs flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="flex items-center gap-2.5">
          <div className="w-9 h-9 rounded-xl bg-blue-100 border border-blue-200 flex items-center justify-center shadow-xs">
            <Swords className="w-5 h-5 text-blue-600" />
          </div>
          <div>
            <h1 className="text-xl font-extrabold text-slate-900 tracking-tight font-sans">
              Match History & Arena Simulations
            </h1>
            <p className="text-xs text-slate-500 font-medium mt-0.5">
              Deterministic 5-bot battle royale runs and verifiable replay recordings.
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2.5">
          <button
            onClick={onRefresh}
            disabled={loading}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-bold bg-slate-100 hover:bg-slate-200 text-slate-700 border border-slate-300/80 transition-all shadow-2xs hover:-translate-y-0.5 active:translate-y-0 cursor-pointer"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            <span>Refresh</span>
          </button>
          <button
            onClick={onTriggerMatch}
            className="inline-flex items-center gap-1.5 px-4 py-2 rounded-lg text-xs font-bold bg-emerald-600 hover:bg-emerald-700 text-white shadow-sm shadow-emerald-500/20 transition-all hover:-translate-y-0.5 active:translate-y-0 cursor-pointer"
          >
            <Play className="w-3.5 h-3.5 fill-white" />
            <span>Launch Match</span>
          </button>
        </div>
      </div>

      {/* Filter and Table Container */}
      <div className="bg-white border border-slate-200/90 rounded-2xl overflow-hidden shadow-xs">
        {/* Filters Toolbar */}
        <div className="px-5 py-3.5 border-b border-slate-200 bg-slate-50/70 flex flex-wrap items-center justify-between gap-3 text-xs">
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-1.5 text-slate-500 font-bold">
              <Filter className="w-3.5 h-3.5 text-blue-600" />
              <span>Filter:</span>
            </div>

            <select
              value={selectedArenaFilter}
              onChange={(e) => setSelectedArenaFilter(e.target.value)}
              className="bg-white border border-slate-300 rounded-lg px-3 py-1.5 text-slate-800 font-semibold focus:outline-none focus:ring-2 focus:ring-blue-500 shadow-2xs"
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
              className="bg-white border border-slate-300 rounded-lg px-3 py-1.5 text-slate-800 font-semibold focus:outline-none focus:ring-2 focus:ring-blue-500 shadow-2xs"
            >
              <option value="">All Statuses</option>
              <option value="finished">Finished</option>
              <option value="running">Running</option>
              <option value="failed">Failed</option>
            </select>
          </div>

          <div className="text-slate-500 font-mono text-xs font-medium">
            Showing <span className="font-bold text-slate-800">{filteredMatches.length}</span> matches
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
                <th className="text-right pr-5">Replay & Action</th>
              </tr>
            </thead>
            <tbody>
              {filteredMatches.length === 0 ? (
                <tr>
                  <td colSpan={8} className="text-center py-12 text-slate-400 font-medium">
                    No matches found matching the selected criteria.
                  </td>
                </tr>
              ) : (
                filteredMatches.map((match) => {
                  const sortedParticipants = [...(match.participants || [])].sort(
                    (a, b) => a.rank_place - b.rank_place
                  );

                  return (
                    <tr key={match.id} className="group">
                      {/* ID */}
                      <td className="text-center font-mono font-bold text-blue-600">
                        #{match.id}
                      </td>

                      {/* Time */}
                      <td className="font-mono text-slate-500 text-[11px] font-medium">
                        {new Date(match.created_at).toLocaleTimeString([], {
                          hour: '2-digit',
                          minute: '2-digit',
                          second: '2-digit',
                        })}
                      </td>

                      {/* Arena */}
                      <td className="text-slate-800 font-bold">
                        {match.arena_name || 'Battle Royale 5P'}
                      </td>

                      {/* Seed */}
                      <td className="font-mono text-slate-500 text-[11px]">
                        <span className="px-1.5 py-0.5 rounded bg-slate-100 border border-slate-200 text-slate-700">
                          {match.seed}
                        </span>
                      </td>

                      {/* 5 Bot Placements with DOMjudge Balloon-style Badges */}
                      <td>
                        <div className="flex items-center gap-1.5 flex-wrap">
                          {sortedParticipants.map((p, pIdx) => {
                            const isWin = p.rank_place === 1 && !p.disqualified;
                            const is2nd = p.rank_place === 2 && !p.disqualified;
                            const is3rd = p.rank_place === 3 && !p.disqualified;

                            return (
                              <span
                                key={pIdx}
                                title={`${p.rank_place}º: ${p.agent_name} (${p.team_name}) - Score: ${p.score}`}
                                className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-mono border shadow-2xs transition-transform group-hover:scale-102 ${
                                  p.disqualified
                                    ? 'bg-rose-50 border-rose-300 text-rose-700 line-through font-medium'
                                    : isWin
                                    ? 'bg-amber-100 border-amber-300 text-amber-900 font-black'
                                    : is2nd
                                    ? 'bg-slate-100 border-slate-300 text-slate-800 font-bold'
                                    : is3rd
                                    ? 'bg-orange-50 border-orange-300 text-orange-900 font-bold'
                                    : 'bg-slate-50 border-slate-200 text-slate-700'
                                }`}
                              >
                                <span className={isWin ? 'text-amber-800 font-black' : 'text-slate-500'}>
                                  {p.rank_place > 0 ? `${p.rank_place}º` : `P${p.seat}`}
                                </span>
                                <span className="max-w-[85px] truncate font-semibold">{p.agent_name}</span>
                              </span>
                            );
                          })}
                        </div>
                      </td>

                      {/* Duration */}
                      <td className="text-right font-mono text-slate-700 font-medium">
                        {match.ticks_played > 0 ? (
                          <>
                            <span className="font-bold">{match.ticks_played}</span>
                            <span className="text-[10px] text-slate-400 ml-1">
                              ({((match.ticks_played * 50) / 1000).toFixed(1)}s)
                            </span>
                          </>
                        ) : (
                          <span className="text-slate-400">-</span>
                        )}
                      </td>

                      {/* Status */}
                      <td className="text-center">
                        <StatusBadge status={match.status} />
                      </td>

                      {/* Actions */}
                      <td className="text-right pr-5">
                        <div className="flex items-center justify-end gap-2">
                          {match.status === 'finished' && (
                            <button
                              onClick={() => onSelectMatch(match.id)}
                              className="inline-flex items-center gap-1.5 px-3 py-1 rounded-lg bg-blue-50 hover:bg-blue-100 border border-blue-200 text-blue-700 text-xs font-bold transition-all shadow-2xs hover:-translate-y-0.5 active:translate-y-0 cursor-pointer"
                            >
                              <Eye className="w-3.5 h-3.5 text-blue-600" />
                              <span>2D Replay</span>
                            </button>
                          )}

                          <button
                            onClick={() => onRerunMatch(match.id)}
                            title="Re-run simulation with identical seed and bots"
                            className="p-1.5 rounded-lg hover:bg-slate-100 text-slate-400 hover:text-slate-700 transition border border-transparent hover:border-slate-200 cursor-pointer"
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
        <div className="p-4 text-center text-xs text-slate-500 bg-slate-50/50 border-t border-slate-200">
          {pageError && <p role="alert" className="text-rose-600 font-semibold mb-2">{pageError}</p>}
          {hasOlder && (
            <button
              onClick={loadOlder}
              disabled={loading || loadingOlder}
              className="px-4 py-2 rounded-lg bg-white border border-slate-300 text-slate-700 font-bold hover:bg-slate-50 shadow-2xs transition disabled:opacity-50 cursor-pointer"
            >
              {loadingOlder ? 'Loading older matches…' : 'Load Older Matches'}
            </button>
          )}
        </div>
      </div>
    </div>
  );
};
