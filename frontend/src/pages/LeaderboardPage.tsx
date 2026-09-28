import React, { useState } from 'react';
import {
  Trophy,
  Swords,
  Search,
  RefreshCw,
  Medal,
  Play,
  TrendingUp,
  Shield,
  Layers,
  Sparkles,
  Bot
} from 'lucide-react';
import { LadderEntry, Arena, getDeterministicColor } from '../types';
import { StatusBadge } from '../components/StatusBadge';

interface LeaderboardPageProps {
  ladder: LadderEntry[];
  arena: Arena | null;
  loading: boolean;
  onRefresh: () => void;
  onTriggerMatch: () => void;
  onSelectMatch: (matchId: number) => void;
}

export const LeaderboardPage: React.FC<LeaderboardPageProps> = ({
  ladder,
  arena,
  loading,
  onRefresh,
  onTriggerMatch,
}) => {
  const [search, setSearch] = useState('');

  const filteredLadder = ladder.filter(
    (e) =>
      e.agent_name.toLowerCase().includes(search.toLowerCase()) ||
      e.team_name.toLowerCase().includes(search.toLowerCase())
  );

  // Stats calculation
  const totalBots = ladder.length;
  const topRating = ladder.length > 0 ? Math.max(...ladder.map((e) => e.display_rating)) : 1500;
  const totalMatches = ladder.reduce((acc, e) => acc + e.matches_played, 0);

  return (
    <div className="space-y-5 animate-fade-in">
      {/* Top Banner & Operational Overview */}
      <div className="bg-white border border-slate-200/90 rounded-2xl p-6 shadow-xs">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-2.5">
              <div className="w-9 h-9 rounded-xl bg-amber-100 border border-amber-200 flex items-center justify-center shadow-xs">
                <Trophy className="w-5 h-5 text-amber-600 fill-amber-500" />
              </div>
              <h1 className="text-xl font-extrabold text-slate-900 tracking-tight font-sans">
                {arena ? arena.name : 'Continuous Ladder Standings'}
              </h1>
            </div>
            <p className="text-xs text-slate-500 mt-1.5 max-w-2xl font-medium">
              {arena?.description ||
                'Official continuous Elo ratings for autonomous agents in the multi-agent battle arena.'}
            </p>
            {arena?.frozen && (
              <div className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-amber-50 border border-amber-200 text-amber-800 text-xs font-semibold mt-2.5">
                <span className="w-2 h-2 rounded-full bg-amber-500"></span>
                Public results are frozen. Administrators see current live standings.
              </div>
            )}
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
              <span>Schedule Match</span>
            </button>
          </div>
        </div>

        {/* Quick KPI Strip with Colorful DOMjudge Accent Cards */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3.5 mt-5 pt-5 border-t border-slate-100">
          <div className="bg-gradient-to-br from-blue-50/80 to-indigo-50/40 border border-blue-200/70 rounded-xl p-3.5 shadow-2xs transition hover:shadow-xs">
            <div className="flex items-center justify-between">
              <span className="text-[11px] text-blue-700 font-bold uppercase tracking-wider">Ranked Bots</span>
              <Bot className="w-4 h-4 text-blue-500" />
            </div>
            <div className="text-2xl font-black font-mono text-slate-900 mt-1">{totalBots}</div>
          </div>
          <div className="bg-gradient-to-br from-amber-50/80 to-yellow-50/40 border border-amber-200/70 rounded-xl p-3.5 shadow-2xs transition hover:shadow-xs">
            <div className="flex items-center justify-between">
              <span className="text-[11px] text-amber-800 font-bold uppercase tracking-wider">Top Elo Rating</span>
              <Trophy className="w-4 h-4 text-amber-500 fill-amber-400" />
            </div>
            <div className="text-2xl font-black font-mono text-amber-700 mt-1">{topRating}</div>
          </div>
          <div className="bg-gradient-to-br from-emerald-50/80 to-teal-50/40 border border-emerald-200/70 rounded-xl p-3.5 shadow-2xs transition hover:shadow-xs">
            <div className="flex items-center justify-between">
              <span className="text-[11px] text-emerald-800 font-bold uppercase tracking-wider">Matches Evaluated</span>
              <Swords className="w-4 h-4 text-emerald-600" />
            </div>
            <div className="text-2xl font-black font-mono text-slate-900 mt-1">{totalMatches}</div>
          </div>
          <div className="bg-gradient-to-br from-purple-50/80 to-pink-50/40 border border-purple-200/70 rounded-xl p-3.5 shadow-2xs transition hover:shadow-xs">
            <div className="flex items-center justify-between">
              <span className="text-[11px] text-purple-800 font-bold uppercase tracking-wider">Format</span>
              <Layers className="w-4 h-4 text-purple-500" />
            </div>
            <div className="text-sm font-black text-slate-800 mt-2 font-mono">5-Seat Battle Royale</div>
          </div>
        </div>
      </div>

      {/* Table Container with Filter */}
      <div className="bg-white border border-slate-200/90 rounded-2xl overflow-hidden shadow-xs">
        {/* Search Toolbar */}
        <div className="px-5 py-3.5 border-b border-slate-200 bg-slate-50/70 flex items-center justify-between gap-4">
          <div className="relative w-full max-w-xs">
            <Search className="w-4 h-4 absolute left-3 top-2.5 text-slate-400" />
            <input
              type="text"
              placeholder="Search agent or team..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full pl-9 pr-3.5 py-1.5 bg-white border border-slate-300 rounded-lg text-xs text-slate-800 placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 transition shadow-2xs"
            />
          </div>

          <div className="text-xs text-slate-500 font-mono font-medium">
            Showing <span className="font-bold text-slate-800">{filteredLadder.length}</span> of {ladder.length} agents
          </div>
        </div>

        {/* DOMjudge-style High-Density Standings Table */}
        <div className="overflow-x-auto">
          <table className="dj-table">
            <thead>
              <tr>
                <th className="w-14 text-center">Rank</th>
                <th>Agent / Bot</th>
                <th>Team & Organization</th>
                <th className="text-right">Elo Rating</th>
                <th className="text-center">Win Rate</th>
                <th className="text-center">Record (W / L)</th>
                <th className="text-right">Kills</th>
                <th className="text-right">Avg Ticks</th>
                <th className="text-center">Status</th>
              </tr>
            </thead>
            <tbody>
              {filteredLadder.length === 0 ? (
                <tr>
                  <td colSpan={9} className="text-center py-12 text-slate-400 font-medium">
                    No agents registered in this ladder yet.
                  </td>
                </tr>
              ) : (
                filteredLadder.map((entry, idx) => {
                  const rank = idx + 1;
                  const isTop1 = rank === 1;
                  const isTop2 = rank === 2;
                  const isTop3 = rank === 3;

                  const teamColor = getDeterministicColor(entry.team_name);

                  return (
                    <tr key={entry.id} className="group">
                      {/* Rank badge with metallic medals */}
                      <td className="text-center font-mono font-bold py-3">
                        {isTop1 ? (
                          <div className="inline-flex items-center justify-center w-7 h-7 rounded-full bg-gradient-to-br from-amber-300 via-amber-400 to-yellow-500 text-amber-950 font-black text-xs shadow-sm shadow-amber-500/30 border border-amber-200">
                            1
                          </div>
                        ) : isTop2 ? (
                          <div className="inline-flex items-center justify-center w-7 h-7 rounded-full bg-gradient-to-br from-slate-200 via-slate-300 to-slate-400 text-slate-800 font-black text-xs shadow-sm border border-slate-300">
                            2
                          </div>
                        ) : isTop3 ? (
                          <div className="inline-flex items-center justify-center w-7 h-7 rounded-full bg-gradient-to-br from-amber-600 via-amber-700 to-amber-800 text-amber-50 font-black text-xs shadow-sm border border-amber-600">
                            3
                          </div>
                        ) : (
                          <span className="text-slate-500 text-xs font-bold">{rank}</span>
                        )}
                      </td>

                      {/* Bot Name with Monospace Badge */}
                      <td className="font-semibold text-slate-900">
                        <div className="flex items-center gap-2">
                          <span className="font-mono text-xs px-2.5 py-1 rounded-md bg-slate-100 border border-slate-200 text-slate-800 font-bold group-hover:border-blue-300 group-hover:bg-blue-50 transition">
                            {entry.agent_name}
                          </span>
                        </div>
                      </td>

                      {/* Team Name with Balloon / Team Avatar */}
                      <td className="text-slate-700">
                        <div className="flex items-center gap-2">
                          <div className={`w-6 h-6 rounded-md flex items-center justify-center font-bold text-xs border ${teamColor.badgeClass}`}>
                            {entry.team_name.charAt(0)}
                          </div>
                          <span className="font-bold text-slate-800">{entry.team_name}</span>
                        </div>
                      </td>

                      {/* Display Rating with Colorful High-Contrast Score */}
                      <td className="text-right font-mono font-bold">
                        <span
                          className={`text-sm ${
                            entry.display_rating >= 1550
                              ? 'text-blue-600 font-extrabold'
                              : entry.display_rating >= 1500
                              ? 'text-emerald-600 font-extrabold'
                              : 'text-slate-700'
                          }`}
                        >
                          {entry.display_rating}
                        </span>
                        <span className="text-[10px] text-slate-400 ml-1 font-normal">
                          ±{Math.round(entry.rating_sigma)}
                        </span>
                      </td>

                      {/* Win Rate Progress Bar */}
                      <td className="text-center">
                        <div className="flex items-center justify-center gap-2">
                          <div className="w-16 bg-slate-200 rounded-full h-2 overflow-hidden shadow-inner">
                            <div
                              className="bg-gradient-to-r from-emerald-500 to-teal-500 h-2 rounded-full transition-all duration-300"
                              style={{ width: `${entry.win_rate}%` }}
                            />
                          </div>
                          <span className="font-mono text-xs font-bold text-slate-700 w-8 text-right">
                            {entry.win_rate.toFixed(0)}%
                          </span>
                        </div>
                      </td>

                      {/* Matches Count (Wins / Losses) */}
                      <td className="text-center font-mono text-xs">
                        <span className="inline-flex items-center gap-1 font-bold text-emerald-700 bg-emerald-50 px-1.5 py-0.5 rounded border border-emerald-200">
                          {entry.wins}W
                        </span>
                        <span className="text-slate-400 mx-1">/</span>
                        <span className="inline-flex items-center gap-1 font-bold text-rose-700 bg-rose-50 px-1.5 py-0.5 rounded border border-rose-200">
                          {entry.matches_played - entry.wins}L
                        </span>
                        <span className="text-slate-400 text-[10px] ml-1.5">({entry.matches_played})</span>
                      </td>

                      {/* Kills with badge */}
                      <td className="text-right font-mono font-bold text-slate-700">
                        {entry.kills > 0 ? (
                          <span className="inline-flex items-center gap-1 text-amber-700 bg-amber-50 px-2 py-0.5 rounded-full border border-amber-200 text-xs">
                            ⚔ {entry.kills}
                          </span>
                        ) : (
                          <span className="text-slate-400">0</span>
                        )}
                      </td>

                      {/* Avg Survival Ticks */}
                      <td className="text-right font-mono text-slate-600 font-medium">
                        {entry.avg_survival_ticks.toFixed(0)}
                      </td>

                      {/* Status */}
                      <td className="text-center">
                        <StatusBadge status={entry.status} />
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
