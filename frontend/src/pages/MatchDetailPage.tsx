import React, { useEffect, useState } from 'react';
import {
  ArrowLeft,
  Download,
  RotateCcw,
  Terminal,
  Trophy,
  Loader2,
  Copy,
  Check,
  Award,
  Swords,
  Timer,
  Hash
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

const BALLOON_COLORS = [
  'bg-red-500 text-white',
  'bg-blue-600 text-white',
  'bg-emerald-600 text-white',
  'bg-amber-500 text-white',
  'bg-purple-600 text-white',
  'bg-pink-500 text-white',
  'bg-cyan-600 text-white',
  'bg-orange-500 text-white',
];

export const MatchDetailPage: React.FC<MatchDetailPageProps> = ({ matchId, onBack, onRerun }) => {
  const [match, setMatch] = useState<Match | null>(null);
  const [replayData, setReplayData] = useState<ReplayData | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'scoreboard' | 'logs'>('scoreboard');
  const [copiedLogs, setCopiedLogs] = useState(false);

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

  const handleCopyLogs = () => {
    if (!match?.execution_log) return;
    navigator.clipboard.writeText(match.execution_log);
    setCopiedLogs(true);
    setTimeout(() => setCopiedLogs(false), 2000);
  };

  if (loading) {
    return (
      <div className="py-24 flex flex-col items-center justify-center gap-3 text-slate-500">
        <Loader2 className="w-9 h-9 animate-spin text-blue-600" />
        <span className="text-sm font-semibold tracking-wide">Loading simulation telemetry & replay frames...</span>
      </div>
    );
  }

  if (error || !match) {
    return (
      <div className="bg-white border border-slate-200 rounded-xl p-8 text-center space-y-4 shadow-sm">
        <div className="text-rose-600 font-bold text-base">Failed to load match #{matchId}</div>
        <div className="text-xs text-slate-500 font-mono bg-slate-50 p-3 rounded-lg border border-slate-200 inline-block max-w-lg">
          {error}
        </div>
        <div>
          <button
            onClick={onBack}
            className="inline-flex items-center gap-1.5 px-4 py-2 rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-700 text-xs font-semibold transition"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Back to Matches</span>
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-5 animate-fade-in">
      {/* Top Header Bar */}
      <div className="bg-white border border-slate-200 rounded-xl p-4 sm:p-5 shadow-sm flex flex-col lg:flex-row lg:items-center justify-between gap-4">
        <div className="flex items-center gap-3.5">
          <button
            onClick={onBack}
            title="Back to Matches"
            className="p-2 rounded-lg bg-slate-100 hover:bg-blue-50 hover:text-blue-600 text-slate-600 transition shadow-xs"
          >
            <ArrowLeft className="w-4 h-4" />
          </button>
          <div>
            <div className="flex items-center gap-2.5 flex-wrap">
              <span className="px-2.5 py-0.5 rounded-full text-xs font-mono font-bold bg-blue-100 text-blue-800 border border-blue-200">
                Match #{match.id}
              </span>
              <h1 className="text-lg font-black text-slate-900 tracking-tight">
                {match.arena_name || 'Arena Contest'}
              </h1>
              <StatusBadge status={match.status} />
            </div>

            <div className="flex items-center gap-2.5 text-xs text-slate-500 font-mono mt-1 flex-wrap">
              <span className="inline-flex items-center gap-1 bg-slate-50 px-2 py-0.5 rounded border border-slate-200">
                <Hash className="w-3 h-3 text-slate-400" />
                Seed: {match.seed}
              </span>
              <span className="inline-flex items-center gap-1 bg-slate-50 px-2 py-0.5 rounded border border-slate-200">
                <Timer className="w-3 h-3 text-slate-400" />
                {match.ticks_played} ticks ({((match.ticks_played * 50) / 1000).toFixed(1)}s)
              </span>
              <span className="inline-flex items-center gap-1 bg-slate-50 px-2 py-0.5 rounded border border-slate-200">
                <Swords className="w-3 h-3 text-slate-400" />
                {match.participants?.length || 0} Contenders
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2.5 flex-wrap">
          <a
            href={`/api/v1/matches/${match.id}/download`}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-semibold bg-white hover:bg-slate-50 text-slate-700 border border-slate-300 shadow-xs hover:shadow transition"
          >
            <Download className="w-3.5 h-3.5 text-slate-500" />
            <span>Download Replay (.gz)</span>
          </a>

          <button
            onClick={() => onRerun(match.id)}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-bold bg-blue-50 hover:bg-blue-100 text-blue-700 border border-blue-200 shadow-xs hover:shadow transition"
          >
            <RotateCcw className="w-3.5 h-3.5 text-blue-600" />
            <span>Re-run Simulation</span>
          </button>
        </div>
      </div>

      {/* Main 2D Canvas Replay Player */}
      {replayData ? (
        <ReplayViewer2D replayData={replayData} matchId={match.id} />
      ) : (
        <div className="bg-white border border-slate-200 rounded-xl p-8 text-center text-slate-500 text-xs shadow-sm">
          Replay telemetry frames are not available for this match.
        </div>
      )}

      {/* Tabs: Scoreboard / Execution Logs */}
      <div className="bg-white border border-slate-200 rounded-xl overflow-hidden shadow-sm">
        <div className="flex items-center border-b border-slate-200 bg-slate-50/80 px-4 pt-2">
          <button
            onClick={() => setActiveTab('scoreboard')}
            className={`flex items-center gap-2 px-4 py-2.5 text-xs font-bold border-b-2 transition ${
              activeTab === 'scoreboard'
                ? 'border-blue-600 text-blue-700 bg-white rounded-t-lg shadow-xs'
                : 'border-transparent text-slate-600 hover:text-slate-900'
            }`}
          >
            <Trophy className="w-4 h-4 text-amber-500" />
            <span>Scoreboard & Elo Rating Breakdown</span>
          </button>
          <button
            onClick={() => setActiveTab('logs')}
            className={`flex items-center gap-2 px-4 py-2.5 text-xs font-bold border-b-2 transition ${
              activeTab === 'logs'
                ? 'border-blue-600 text-blue-700 bg-white rounded-t-lg shadow-xs'
                : 'border-transparent text-slate-600 hover:text-slate-900'
            }`}
          >
            <Terminal className="w-4 h-4 text-slate-500" />
            <span>Arbiter Sandbox Execution Logs</span>
          </button>
        </div>

        <div className="p-4 sm:p-5">
          {activeTab === 'scoreboard' ? (
            <div className="overflow-x-auto">
              <table className="dj-table">
                <thead>
                  <tr>
                    <th className="w-16 text-center">Place</th>
                    <th className="w-16">Seat</th>
                    <th>Agent / Bot</th>
                    <th>Team</th>
                    <th className="text-right">Kills</th>
                    <th className="text-right">Survival</th>
                    <th className="text-right">Score</th>
                    <th className="text-right">Old Rating</th>
                    <th className="text-right">New Rating</th>
                    <th className="text-right">Elo Delta</th>
                  </tr>
                </thead>
                <tbody>
                  {[...(match.participants || [])]
                    .sort((a, b) => a.rank_place - b.rank_place)
                    .map((p, idx) => {
                      const isPositive = p.rating_delta > 0;
                      const isNegative = p.rating_delta < 0;
                      const balloonColor = BALLOON_COLORS[p.seat % BALLOON_COLORS.length];

                      return (
                        <tr key={p.id} className="hover:bg-blue-50/70 transition-colors">
                          <td className="text-center font-mono font-bold">
                            {p.disqualified ? (
                              <span className="inline-flex px-2 py-0.5 rounded text-[11px] font-bold bg-rose-100 text-rose-800 border border-rose-200">
                                DQ
                              </span>
                            ) : p.rank_place === 1 ? (
                              <span className="inline-flex items-center justify-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-black bg-gradient-to-r from-amber-400 to-yellow-500 text-white shadow-xs">
                                <Award className="w-3.5 h-3.5 fill-current" />
                                1st
                              </span>
                            ) : p.rank_place === 2 ? (
                              <span className="inline-flex items-center justify-center px-2 py-0.5 rounded-full text-xs font-bold bg-slate-200 text-slate-700 border border-slate-300">
                                2nd
                              </span>
                            ) : p.rank_place === 3 ? (
                              <span className="inline-flex items-center justify-center px-2 py-0.5 rounded-full text-xs font-bold bg-amber-100 text-amber-800 border border-amber-300">
                                3rd
                              </span>
                            ) : (
                              <span className="text-slate-600 font-mono text-xs">{p.rank_place}th</span>
                            )}
                          </td>
                          <td className="font-mono text-slate-500 text-xs font-bold">P{p.seat}</td>
                          <td>
                            <div className="flex items-center gap-2">
                              <span
                                className={`w-5 h-5 rounded-full text-[10px] font-black flex items-center justify-center shadow-xs shrink-0 ${balloonColor}`}
                              >
                                {p.agent_name.charAt(0).toUpperCase()}
                              </span>
                              <div>
                                <span className="font-bold text-slate-900 text-xs">{p.agent_name}</span>
                                {p.disqualification_reason && (
                                  <span className="block text-[10px] text-rose-600 font-medium">
                                    Reason: {p.disqualification_reason}
                                  </span>
                                )}
                              </div>
                            </div>
                          </td>
                          <td className="text-slate-600 text-xs font-medium">{p.team_name}</td>
                          <td className="text-right font-mono text-slate-700 text-xs font-semibold">{p.kills}</td>
                          <td className="text-right font-mono text-slate-700 text-xs">{p.survival_ticks}</td>
                          <td className="text-right font-mono font-bold text-slate-900 text-xs">
                            {p.score.toFixed(1)}
                          </td>
                          <td className="text-right font-mono text-slate-500 text-xs">{Math.round(p.old_rating)}</td>
                          <td className="text-right font-mono text-slate-900 font-extrabold text-xs">{Math.round(p.new_rating)}</td>
                          <td className="text-right font-mono font-bold text-xs">
                            <span
                              className={`inline-block px-2 py-0.5 rounded-full text-[11px] font-extrabold ${
                                isPositive
                                  ? 'bg-emerald-100 text-emerald-800 border border-emerald-300'
                                  : isNegative
                                  ? 'bg-rose-100 text-rose-800 border border-rose-300'
                                  : 'bg-slate-100 text-slate-600 border border-slate-200'
                              }`}
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
            <div className="space-y-3">
              <div className="flex items-center justify-between text-xs text-slate-500">
                <span className="font-mono">Arbiter Stdout / Stderr Telemetry Log</span>
                <button
                  onClick={handleCopyLogs}
                  className="inline-flex items-center gap-1.5 px-3 py-1 rounded bg-slate-100 hover:bg-slate-200 text-slate-700 font-semibold transition"
                >
                  {copiedLogs ? (
                    <>
                      <Check className="w-3.5 h-3.5 text-emerald-600" />
                      <span className="text-emerald-700">Copied to Clipboard!</span>
                    </>
                  ) : (
                    <>
                      <Copy className="w-3.5 h-3.5 text-slate-500" />
                      <span>Copy Logs</span>
                    </>
                  )}
                </button>
              </div>
              <div className="bg-slate-950 p-4 rounded-xl border border-slate-800 font-mono text-xs text-emerald-400 overflow-x-auto max-h-96 whitespace-pre shadow-inner">
                {match.execution_log || 'No execution logs recorded.'}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
