import React, { useState } from 'react';
import {
  Gamepad2,
  Trophy,
  Play,
  X,
  ShieldCheck,
  CheckCircle2,
  AlertCircle,
  Loader2,
  Clock,
  Users,
  Eye,
  EyeOff,
  Flame
} from 'lucide-react';
import { Arena, AgentVersion } from '../types';
import { api } from '../api/client';

interface ArenasPageProps {
  arenas: Arena[];
  canFreeze: boolean;
  onFreeze: (id: number, frozen: boolean) => Promise<void>;
}

export const ArenasPage: React.FC<ArenasPageProps> = ({ arenas, canFreeze, onFreeze }) => {
  const [pending, setPending] = useState<number | null>(null);
  const [error, setError] = useState('');
  const [selectedArenaForRound, setSelectedArenaForRound] = useState<Arena | null>(null);
  const [loadingAgents, setLoadingAgents] = useState(false);
  const [arenaAgents, setArenaAgents] = useState<AgentVersion[]>([]);
  const [selectedAgentIds, setSelectedAgentIds] = useState<number[]>([]);
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const [seedInput, setSeedInput] = useState<string>('');
  const [schedulingRound, setSchedulingRound] = useState(false);
  const [roundReceipt, setRoundReceipt] = useState<{ id: number; total_matches: number; format_version: string } | null>(null);
  const [roundError, setRoundError] = useState('');

  const toggleFreeze = async (arena: Arena) => {
    setPending(arena.id);
    setError('');
    try {
      await onFreeze(arena.id, !arena.frozen);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not change results visibility');
    } finally {
      setPending(null);
    }
  };

  const openRoundModal = async (arena: Arena) => {
    setSelectedArenaForRound(arena);
    setLoadingAgents(true);
    setRoundReceipt(null);
    setRoundError('');
    setIdempotencyKey(`round-${arena.slug}-${Date.now()}`);
    setSeedInput(Math.floor(Math.random() * 1000000).toString());

    try {
      const agents = await api.getAgents(arena.id);
      const activeAgents = agents.filter((a) => a.status === 'active');
      setArenaAgents(activeAgents);

      // Select unique agents per team (latest version of each team)
      const teamMap = new Map<number, AgentVersion>();
      for (const a of activeAgents) {
        if (!teamMap.has(a.team_id)) {
          teamMap.set(a.team_id, a);
        }
      }
      setSelectedAgentIds(Array.from(teamMap.values()).map((a) => a.id));
    } catch (err) {
      setRoundError('Failed to fetch agents for this arena');
    } finally {
      setLoadingAgents(false);
    }
  };

  const handleToggleAgent = (id: number) => {
    setSelectedAgentIds((prev) =>
      prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]
    );
  };

  const handleScheduleRound = async () => {
    if (!selectedArenaForRound) return;
    if (selectedAgentIds.length < 5) {
      setRoundError('At least 5 distinct teams are required to schedule a round');
      return;
    }
    setSchedulingRound(true);
    setRoundError('');
    setRoundReceipt(null);

    try {
      const seedVal = seedInput.trim() !== '' ? parseInt(seedInput.trim(), 10) : undefined;
      const res = await api.scheduleRound(
        selectedArenaForRound.id,
        idempotencyKey.trim(),
        selectedAgentIds,
        seedVal
      );
      setRoundReceipt(res);
    } catch (err) {
      setRoundError(err instanceof Error ? err.message : 'Failed to schedule round');
    } finally {
      setSchedulingRound(false);
    }
  };

  return (
    <div className="space-y-4 animate-fade-in">
      <div className="bg-white border border-slate-200 rounded-xl p-4 sm:p-5 shadow-sm flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-blue-50 border border-blue-100 flex items-center justify-center text-blue-600 shadow-xs">
            <Gamepad2 className="w-5 h-5" />
          </div>
          <div>
            <h1 className="text-base font-black text-slate-900 tracking-tight uppercase">
              Competitive Game Arenas & Rules
            </h1>
            <p className="text-xs text-slate-500 font-medium">
              Simulation sandboxes, contest problem configurations, and scoreboard freeze controls
            </p>
          </div>
        </div>
      </div>

      {error && (
        <div role="alert" className="p-3.5 rounded-xl bg-rose-50 border border-rose-200 text-xs text-rose-700 flex items-center gap-2">
          <AlertCircle className="w-4 h-4 flex-shrink-0 text-rose-500" />
          <span className="font-semibold">{error}</span>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {arenas.map((arena) => (
          <div
            key={arena.id}
            className="bg-white border border-slate-200 rounded-xl p-5 shadow-sm hover:shadow-md hover:border-blue-300 transition-all space-y-4 flex flex-col justify-between"
          >
            <div className="space-y-3">
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="text-base font-bold text-slate-900">{arena.name}</h3>
                  <span className="inline-block font-mono text-[11px] text-blue-700 bg-blue-50 px-2 py-0.5 rounded border border-blue-100 font-semibold mt-1">
                    {arena.slug}
                  </span>
                </div>
                <span className="px-2.5 py-0.5 rounded-full text-[11px] font-mono font-bold bg-emerald-50 text-emerald-700 border border-emerald-200 shadow-xs">
                  {arena.is_active ? 'ACTIVE CONTEST' : 'INACTIVE'}
                </span>
              </div>

              <p className="text-xs text-slate-600 leading-relaxed font-normal">{arena.description}</p>

              {arena.frozen && (
                <div className="flex items-center gap-2 p-2.5 rounded-lg bg-amber-50 border border-amber-200 text-xs text-amber-800 font-medium">
                  <EyeOff className="w-4 h-4 text-amber-600 shrink-0" />
                  <span>Public scoreboard is frozen. Real-time results visible only to staff.</span>
                </div>
              )}
            </div>

            <div className="space-y-3 pt-2">
              {canFreeze && (
                <div className="flex flex-wrap items-center gap-2">
                  <button
                    disabled={pending !== null}
                    onClick={() => toggleFreeze(arena)}
                    className="inline-flex items-center gap-1.5 rounded-lg border border-slate-300 hover:bg-slate-50 px-3 py-1.5 text-xs font-semibold text-slate-700 disabled:opacity-50 transition shadow-xs"
                  >
                    {arena.frozen ? <Eye className="w-3.5 h-3.5 text-blue-600" /> : <EyeOff className="w-3.5 h-3.5 text-amber-600" />}
                    <span>{pending === arena.id ? 'Updating…' : arena.frozen ? 'Unfreeze Scoreboard' : 'Freeze Scoreboard'}</span>
                  </button>
                  <button
                    onClick={() => openRoundModal(arena)}
                    className="inline-flex items-center gap-1.5 rounded-lg bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-700 hover:to-indigo-700 text-white px-3.5 py-1.5 text-xs font-bold shadow-xs hover:shadow active:scale-[0.98] transition"
                  >
                    <Trophy className="w-3.5 h-3.5" />
                    <span>Schedule Tournament Round</span>
                  </button>
                </div>
              )}

              <div className="grid grid-cols-3 gap-2 text-xs pt-2 border-t border-slate-100">
                <div className="bg-slate-50 p-2.5 rounded-lg border border-slate-200 text-center">
                  <span className="text-[10px] text-slate-400 uppercase font-bold block">Capacity</span>
                  <span className="font-mono font-bold text-slate-800 text-xs mt-0.5 block">{arena.max_players} Players</span>
                </div>
                <div className="bg-slate-50 p-2.5 rounded-lg border border-slate-200 text-center">
                  <span className="text-[10px] text-slate-400 uppercase font-bold block">Max Duration</span>
                  <span className="font-mono font-bold text-slate-800 text-xs mt-0.5 block">{arena.max_ticks} Ticks</span>
                </div>
                <div className="bg-slate-50 p-2.5 rounded-lg border border-slate-200 text-center">
                  <span className="text-[10px] text-slate-400 uppercase font-bold block">Engine</span>
                  <span className="font-mono font-bold text-blue-700 text-xs mt-0.5 block">{arena.game_type}</span>
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Schedule Tournament Round Modal */}
      {selectedArenaForRound && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 backdrop-blur-sm p-4">
          <div className="bg-white border border-slate-200 rounded-2xl max-w-lg w-full p-5 sm:p-6 space-y-4 shadow-2xl animate-fade-in">
            <div className="flex items-center justify-between border-b border-slate-100 pb-3">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-amber-50 border border-amber-200 flex items-center justify-center text-amber-600">
                  <Trophy className="w-4 h-4" />
                </div>
                <div>
                  <h3 className="text-sm font-black text-slate-900 uppercase tracking-wide">
                    Schedule Round: {selectedArenaForRound.name}
                  </h3>
                  <p className="text-[11px] text-slate-400">Launch all-play-all or tournament pair matches</p>
                </div>
              </div>
              <button
                onClick={() => setSelectedArenaForRound(null)}
                className="text-slate-400 hover:text-slate-700 p-1 rounded-lg hover:bg-slate-100 transition"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            {loadingAgents ? (
              <div className="py-12 flex flex-col items-center justify-center gap-2.5 text-xs text-slate-500">
                <Loader2 className="w-6 h-6 animate-spin text-blue-600" />
                <span className="font-semibold">Loading active roster agents...</span>
              </div>
            ) : roundReceipt ? (
              <div className="space-y-4 py-2">
                <div className="p-4 bg-emerald-50 border border-emerald-200 rounded-xl text-xs text-emerald-900 space-y-1.5">
                  <div className="flex items-center gap-2 font-bold text-sm text-emerald-800">
                    <CheckCircle2 className="w-5 h-5 text-emerald-600" />
                    <span>Tournament Round Materialized!</span>
                  </div>
                  <p className="font-mono">Round ID: #{roundReceipt.id}</p>
                  <p className="font-mono">Total Materialized Matches: {roundReceipt.total_matches}</p>
                  <p className="font-mono">Format Version: {roundReceipt.format_version}</p>
                </div>
                <button
                  onClick={() => setSelectedArenaForRound(null)}
                  className="w-full py-2.5 rounded-xl bg-slate-900 hover:bg-slate-800 text-xs font-bold text-white transition shadow-sm"
                >
                  Close
                </button>
              </div>
            ) : (
              <div className="space-y-4">
                {roundError && (
                  <div className="p-3 rounded-xl bg-rose-50 border border-rose-200 text-xs text-rose-700 flex items-center gap-2 font-semibold">
                    <AlertCircle className="w-4 h-4 flex-shrink-0 text-rose-500" />
                    <span>{roundError}</span>
                  </div>
                )}

                <div>
                  <label className="block text-xs font-bold text-slate-700 mb-1">Idempotency Key</label>
                  <input
                    type="text"
                    value={idempotencyKey}
                    onChange={(e) => setIdempotencyKey(e.target.value)}
                    className="w-full bg-slate-50 border border-slate-300 rounded-lg px-3 py-2 text-xs font-mono text-slate-800 focus:bg-white focus:outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100 transition"
                  />
                </div>

                <div>
                  <label className="block text-xs font-bold text-slate-700 mb-1">PRNG Seed (Numeric, optional)</label>
                  <input
                    type="number"
                    value={seedInput}
                    onChange={(e) => setSeedInput(e.target.value)}
                    className="w-full bg-slate-50 border border-slate-300 rounded-lg px-3 py-2 text-xs font-mono text-slate-800 focus:bg-white focus:outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100 transition"
                  />
                </div>

                <div className="space-y-1.5">
                  <div className="flex items-center justify-between text-xs">
                    <span className="font-bold text-slate-700">
                      Roster Contenders ({selectedAgentIds.length} selected, min 5)
                    </span>
                    <span className="font-mono text-[11px] text-slate-500 font-medium">
                      1 agent per team
                    </span>
                  </div>
                  <div className="max-h-48 overflow-y-auto space-y-1 bg-slate-50 p-2 rounded-xl border border-slate-200 divide-y divide-slate-100">
                    {arenaAgents.length === 0 ? (
                      <p className="text-xs text-slate-400 p-2">No active agents found in this arena.</p>
                    ) : (
                      arenaAgents.map((agent) => (
                        <label
                          key={agent.id}
                          className="flex items-center gap-2.5 px-2.5 py-1.5 rounded-lg hover:bg-blue-50/70 cursor-pointer text-xs transition"
                        >
                          <input
                            type="checkbox"
                            checked={selectedAgentIds.includes(agent.id)}
                            onChange={() => handleToggleAgent(agent.id)}
                            className="rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                          />
                          <span className="font-bold text-slate-800">{agent.name}</span>
                          <span className="font-mono text-[10px] text-slate-500 bg-white px-1.5 py-0.2 rounded border border-slate-200">
                            v{agent.version}
                          </span>
                          <span className="ml-auto text-[10px] text-slate-500 font-mono">Team #{agent.team_id}</span>
                        </label>
                      ))
                    )}
                  </div>
                </div>

                <div className="flex items-center justify-end gap-2.5 pt-3 border-t border-slate-100">
                  <button
                    type="button"
                    onClick={() => setSelectedArenaForRound(null)}
                    className="px-4 py-2 rounded-lg border border-slate-300 hover:bg-slate-50 text-xs font-semibold text-slate-700 transition"
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    disabled={schedulingRound || selectedAgentIds.length < 5}
                    onClick={handleScheduleRound}
                    className="inline-flex items-center gap-1.5 px-4 py-2 rounded-lg bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-700 hover:to-indigo-700 text-xs font-bold text-white shadow-xs hover:shadow active:scale-[0.98] disabled:opacity-50 transition"
                  >
                    {schedulingRound ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Play className="w-3.5 h-3.5" />}
                    <span>Confirm & Schedule Round</span>
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
