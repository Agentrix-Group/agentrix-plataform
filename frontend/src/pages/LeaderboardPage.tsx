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
  Sparkles
} from 'lucide-react';
import { LadderEntry, Arena } from '../types';
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
    <div className="space-y-4">
      {/* Top Banner & Operational Overview */}
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-5 shadow-sm">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <div className="flex items-center gap-2">
              <Trophy className="w-5 h-5 text-amber-400" />
              <h1 className="text-base font-bold text-slate-100 uppercase tracking-wide">
                {arena ? arena.name : 'Continuous Ladder Standings'}
              </h1>
            </div>
            <p className="text-xs text-slate-400 mt-1 max-w-2xl">
              {arena?.description ||
                'Official continuous Elo ratings for autonomous agents in the multi-agent battle arena.'}
            </p>
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
              <span>Schedule Match</span>
            </button>
          </div>
        </div>

        {/* Quick KPI Strip */}
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mt-4 pt-4 border-t border-slate-800/80">
          <div className="bg-slate-950/60 border border-slate-800/80 rounded p-2.5">
            <div className="text-[11px] text-slate-500 font-medium">Ranked Bots</div>
            <div className="text-lg font-bold font-mono text-slate-200 mt-0.5">{totalBots}</div>
          </div>
          <div className="bg-slate-950/60 border border-slate-800/80 rounded p-2.5">
            <div className="text-[11px] text-slate-500 font-medium">Top Elo Rating</div>
            <div className="text-lg font-bold font-mono text-sky-400 mt-0.5">{topRating}</div>
          </div>
          <div className="bg-slate-950/60 border border-slate-800/80 rounded p-2.5">
            <div className="text-[11px] text-slate-500 font-medium">Total Matches Evaluated</div>
            <div className="text-lg font-bold font-mono text-slate-200 mt-0.5">{totalMatches}</div>
          </div>
          <div className="bg-slate-950/60 border border-slate-800/80 rounded p-2.5">
            <div className="text-[11px] text-slate-500 font-medium">Evaluation Format</div>
            <div className="text-xs font-semibold text-slate-300 mt-1">Multi-agent Elo (5 Bots)</div>
          </div>
        </div>
      </div>

      {/* Table Container with Filter */}
      <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden shadow-sm">
        {/* Search Toolbar */}
        <div className="px-4 py-3 border-b border-slate-800 bg-slate-950/60 flex items-center justify-between">
          <div className="relative w-full max-w-xs">
            <Search className="w-3.5 h-3.5 absolute left-3 top-2.5 text-slate-500" />
            <input
              type="text"
              placeholder="Search bot or team name..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full pl-8 pr-3 py-1.5 bg-slate-900 border border-slate-800 rounded text-xs text-slate-200 placeholder-slate-600 focus:outline-none focus:border-sky-500"
            />
          </div>

          <div className="text-xs text-slate-400 font-mono">
            Showing {filteredLadder.length} of {ladder.length} bots
          </div>
        </div>

        {/* DOMjudge-style High-Density Standings Table */}
        <div className="overflow-x-auto">
          <table className="dj-table">
            <thead>
              <tr>
                <th className="w-12 text-center">Rank</th>
                <th>Agent / Bot</th>
                <th>Team & Affiliation</th>
                <th className="text-right">Elo Rating</th>
                <th className="text-center">Win Rate</th>
                <th className="text-center">Matches (W/L)</th>
                <th className="text-right">Kills</th>
                <th className="text-right">Avg Ticks</th>
                <th className="text-center">Status</th>
              </tr>
            </thead>
            <tbody>
              {filteredLadder.length === 0 ? (
                <tr>
                  <td colSpan={9} className="text-center py-8 text-slate-500">
                    No agents registered in this ladder yet.
                  </td>
                </tr>
              ) : (
                filteredLadder.map((entry, idx) => {
                  const rank = idx + 1;
                  const isTop1 = rank === 1;
                  const isTop2 = rank === 2;
                  const isTop3 = rank === 3;

                  return (
                    <tr key={entry.id} className="transition">
                      {/* Rank badge */}
                      <td className="text-center font-mono font-bold">
                        {isTop1 ? (
                          <span className="inline-flex items-center justify-center w-6 h-6 rounded-full bg-amber-400/20 text-amber-300 border border-amber-400/40 text-xs">
                            1
                          </span>
                        ) : isTop2 ? (
                          <span className="inline-flex items-center justify-center w-6 h-6 rounded-full bg-slate-400/20 text-slate-300 border border-slate-400/40 text-xs">
                            2
                          </span>
                        ) : isTop3 ? (
                          <span className="inline-flex items-center justify-center w-6 h-6 rounded-full bg-amber-700/20 text-amber-500 border border-amber-700/40 text-xs">
                            3
                          </span>
                        ) : (
                          <span className="text-slate-500">{rank}</span>
                        )}
                      </td>

                      {/* Bot Name */}
                      <td className="font-semibold text-slate-100 flex items-center gap-1.5">
                        <Sparkles className="w-3 h-3 text-sky-400" />
                        <span>{entry.agent_name}</span>
                      </td>

                      {/* Team Name */}
                      <td className="text-slate-400">{entry.team_name}</td>

                      {/* Display Rating */}
                      <td className="text-right font-mono font-bold">
                        <span
                          className={
                            entry.display_rating >= 1550
                              ? 'text-sky-400'
                              : entry.display_rating >= 1500
                              ? 'text-emerald-400'
                              : 'text-slate-300'
                          }
                        >
                          {entry.display_rating}
                        </span>
                        <span className="text-[10px] text-slate-500 ml-1 font-normal">
                          ±{Math.round(entry.rating_sigma)}
                        </span>
                      </td>

                      {/* Win Rate Progress Bar */}
                      <td className="text-center">
                        <div className="flex items-center justify-center gap-2">
                          <div className="w-16 bg-slate-800 rounded-full h-1.5 overflow-hidden">
                            <div
                              className="bg-emerald-500 h-1.5 rounded-full"
                              style={{ width: `${entry.win_rate}%` }}
                            />
                          </div>
                          <span className="font-mono text-xs text-slate-300 w-8 text-right">
                            {entry.win_rate.toFixed(0)}%
                          </span>
                        </div>
                      </td>

                      {/* Matches Count (Wins / Losses) */}
                      <td className="text-center font-mono text-xs">
                        <span className="text-emerald-400 font-semibold">{entry.wins}W</span>
                        <span className="text-slate-600 mx-1">/</span>
                        <span className="text-rose-400">{entry.matches_played - entry.wins}L</span>
                        <span className="text-slate-500 text-[10px] ml-1">({entry.matches_played})</span>
                      </td>

                      {/* Kills */}
                      <td className="text-right font-mono text-slate-300">{entry.kills}</td>

                      {/* Avg Survival Ticks */}
                      <td className="text-right font-mono text-slate-400">
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
