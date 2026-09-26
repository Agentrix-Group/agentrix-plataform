import React, { useEffect, useState } from 'react';
import {
  ArrowLeft,
  Download,
  RotateCcw,
  Terminal,
  Trophy,
  ShieldAlert,
  Loader2,
  FileCode2,
  Activity
} from 'lucide-react';
import { api } from '../api/client';
import { Match, ReplayData } from '../types';
import { ReplayViewer2D } from '../components/ReplayViewer2D';
import { StatusBadge } from '../components/StatusBadge';

interface MatchDetailPageProps {
  matchId: number;
  onBack: () => void;
  onRerun: (matchId: number) => void;
}

export const MatchDetailPage: React.FC<MatchDetailPageProps> = ({ matchId, onBack, onRerun }) => {
  const [match, setMatch] = useState<Match | null>(null);
  const [replayData, setReplayData] = useState<ReplayData | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'scoreboard' | 'logs'>('scoreboard');

  useEffect(() => {
    let mounted = true;
    setLoading(true);
    setError(null);

    Promise.all([api.getMatch(matchId), api.getReplay(matchId)])
      .then(([m, r]) => {
        if (!mounted) return;
        setMatch(m);
        setReplayData(r);
      })
      .catch((err) => {
        if (!mounted) return;
        setError(err.message || 'Failed to load match and replay data');
      })
      .finally(() => {
        if (mounted) setLoading(false);
      });

    return () => {
      mounted = false;
    };
  }, [matchId]);

  if (loading) {
    return (
      <div className="py-20 flex flex-col items-center justify-center gap-3 text-slate-400">
        <Loader2 className="w-8 h-8 animate-spin text-sky-400" />
        <span className="text-xs font-mono">Loading simulation telemetry & replay frames...</span>
      </div>
    );
  }

  if (error || !match) {
    return (
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-8 text-center space-y-3">
        <div className="text-rose-400 font-semibold text-sm">Failed to load match #{matchId}</div>
        <div className="text-xs text-slate-400 font-mono">{error}</div>
        <button
          onClick={onBack}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs transition"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Back to Matches</span>
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Top Header Bar */}
      <div className="bg-slate-900 border border-slate-800 rounded-lg p-4 shadow-sm flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <button
            onClick={onBack}
            className="p-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition"
          >
            <ArrowLeft className="w-4 h-4" />
          </button>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-base font-bold font-mono text-slate-100">
                Match #{match.id}
              </h1>
              <StatusBadge status={match.status} />
            </div>
            <div className="text-[11px] text-slate-400 font-mono mt-0.5">
              <span>{match.arena_name || 'Arena'}</span>
              <span className="mx-1.5">•</span>
              <span>Seed: {match.seed}</span>
              <span className="mx-1.5">•</span>
              <span>Duration: {match.ticks_played} ticks ({((match.ticks_played * 50) / 1000).toFixed(1)}s)</span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <a
            href={`/api/v1/matches/${match.id}/download`}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
          >
            <Download className="w-3.5 h-3.5 text-slate-400" />
            <span>Download Replay (.gz)</span>
          </a>

          <button
            onClick={() => onRerun(match.id)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 transition"
          >
            <RotateCcw className="w-3.5 h-3.5 text-sky-400" />
            <span>Re-run Simulation</span>
          </button>
        </div>
      </div>

      {/* Main 2D Canvas Replay Player */}
      {replayData ? (
        <ReplayViewer2D replayData={replayData} matchId={match.id} />
      ) : (
        <div className="bg-slate-900 border border-slate-800 rounded-lg p-8 text-center text-slate-400 text-xs">
          Replay not available for this match.
        </div>
      )}

      {/* Tabs: Scoreboard / Execution Logs */}
      <div className="bg-slate-900 border border-slate-800 rounded-lg overflow-hidden shadow-sm">
        <div className="flex items-center border-b border-slate-800 bg-slate-950/60 px-4 pt-2">
          <button
            onClick={() => setActiveTab('scoreboard')}
            className={`flex items-center gap-2 px-3 py-2 text-xs font-semibold border-b-2 transition ${
              activeTab === 'scoreboard'
                ? 'border-sky-500 text-sky-400 bg-slate-900/60'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Trophy className="w-3.5 h-3.5" />
            <span>Match Results & Rating Breakdown</span>
          </button>
          <button
            onClick={() => setActiveTab('logs')}
            className={`flex items-center gap-2 px-3 py-2 text-xs font-semibold border-b-2 transition ${
              activeTab === 'logs'
                ? 'border-sky-500 text-sky-400 bg-slate-900/60'
                : 'border-transparent text-slate-400 hover:text-slate-200'
            }`}
          >
            <Terminal className="w-3.5 h-3.5" />
            <span>Arbiter Execution Logs</span>
          </button>
        </div>

        <div className="p-4">
          {activeTab === 'scoreboard' ? (
            <div className="overflow-x-auto">
              <table className="dj-table">
                <thead>
                  <tr>
                    <th className="w-12 text-center">Place</th>
                    <th>Seat</th>
                    <th>Agent / Bot</th>
                    <th>Team</th>
                    <th className="text-right">Kills</th>
                    <th className="text-right">Survival Ticks</th>
                    <th className="text-right">Score</th>
                    <th className="text-right">Rating Before</th>
                    <th className="text-right">Rating After</th>
                    <th className="text-right">Elo Delta</th>
                  </tr>
                </thead>
                <tbody>
                  {[...(match.participants || [])]
                    .sort((a, b) => a.rank_place - b.rank_place)
                    .map((p) => {
                      const isPositive = p.rating_delta > 0;
                      const isNegative = p.rating_delta < 0;

                      return (
                        <tr key={p.id}>
                          <td className="text-center font-mono font-bold">
                            {p.disqualified ? (
                              <span className="text-rose-400 text-xs">DQ</span>
                            ) : (
                              <span>{p.rank_place}º</span>
                            )}
                          </td>
                          <td className="font-mono text-slate-400">P{p.seat}</td>
                          <td className="font-semibold text-slate-100">
                            {p.agent_name}
                            {p.disqualification_reason && (
                              <span className="block text-[10px] text-rose-400 font-normal mt-0.5">
                                {p.disqualification_reason}
                              </span>
                            )}
                          </td>
                          <td className="text-slate-400">{p.team_name}</td>
                          <td className="text-right font-mono text-slate-300">{p.kills}</td>
                          <td className="text-right font-mono text-slate-300">{p.survival_ticks}</td>
                          <td className="text-right font-mono font-semibold text-slate-200">
                            {p.score.toFixed(1)}
                          </td>
                          <td className="text-right font-mono text-slate-400">{Math.round(p.old_rating)}</td>
                          <td className="text-right font-mono text-slate-100 font-bold">{Math.round(p.new_rating)}</td>
                          <td className="text-right font-mono font-bold">
                            <span
                              className={
                                isPositive
                                  ? 'text-emerald-400'
                                  : isNegative
                                  ? 'text-rose-400'
                                  : 'text-slate-400'
                              }
                            >
                              {isPositive ? `+${p.rating_delta.toFixed(1)}` : p.rating_delta.toFixed(1)}
                            </span>
                          </td>
                        </tr>
                      );
                    })}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="bg-slate-950 p-4 rounded border border-slate-800/80 font-mono text-xs text-slate-300 overflow-x-auto max-h-96 whitespace-pre">
              {match.execution_log || 'No execution logs recorded.'}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
