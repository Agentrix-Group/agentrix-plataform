import React, { useState } from 'react';
import { X, Play, Shuffle, CheckCircle2, AlertCircle, Loader2, Swords, Bot } from 'lucide-react';
import { api } from '../api/client';
import { Arena, AgentVersion } from '../types';

interface TriggerMatchModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (matchId: number) => void;
  arenas: Arena[];
  agents: AgentVersion[];
  defaultArenaId: number;
}

const BALLOON_COLORS = [
  'bg-blue-600 text-white',
  'bg-emerald-600 text-white',
  'bg-purple-600 text-white',
  'bg-amber-500 text-white',
  'bg-rose-600 text-white',
  'bg-cyan-600 text-white',
  'bg-indigo-600 text-white',
];

export const TriggerMatchModal: React.FC<TriggerMatchModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
  arenas,
  agents,
  defaultArenaId,
}) => {
  const [arenaId, setArenaId] = useState<number>(defaultArenaId);
  const [selectedAgentIds, setSelectedAgentIds] = useState<number[]>([]);
  const [seed, setSeed] = useState<number>(Math.floor(Math.random() * 900000) + 100000);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!isOpen) return null;

  const arenaAgents = agents.filter((a) => a.arena_id === arenaId && a.status === 'active');

  const handleToggleAgent = (id: number) => {
    if (selectedAgentIds.includes(id)) {
      setSelectedAgentIds(selectedAgentIds.filter((x) => x !== id));
    } else {
      if (selectedAgentIds.length >= 5) {
        setError('Maximum 5 bots can be selected for this arena match');
        return;
      }
      setSelectedAgentIds([...selectedAgentIds, id]);
      setError(null);
    }
  };

  const handleAutoFill = () => {
    const shuffled = [...arenaAgents].sort(() => 0.5 - Math.random());
    const picked = shuffled.slice(0, 5).map((a) => a.id);
    setSelectedAgentIds(picked);
    setError(null);
  };

  const handleRandomSeed = () => {
    setSeed(Math.floor(Math.random() * 900000) + 100000);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      const resp = await api.triggerMatch(
        arenaId,
        selectedAgentIds.length === 5 ? selectedAgentIds : undefined,
        seed
      );
      onSuccess(resp.match_id);
      onClose();
    } catch (err: any) {
      setError(err.message || 'Failed to trigger match');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/50 backdrop-blur-sm p-4">
      <div className="bg-white border border-slate-200 rounded-2xl shadow-2xl w-full max-w-lg overflow-hidden animate-fade-in">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-5 border-b border-slate-100 bg-slate-50/70">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-emerald-50 border border-emerald-100 flex items-center justify-center text-emerald-600 shadow-xs">
              <Swords className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-black text-slate-900 tracking-tight">Launch Arena Simulation</h3>
              <p className="text-[11px] text-slate-400 font-medium">Schedule immediate competitive match</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-700 p-1.5 rounded-lg hover:bg-slate-100 transition"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="p-6 space-y-4 text-xs">
          {error && (
            <div className="p-3.5 rounded-xl bg-rose-50 border border-rose-200 text-rose-700 flex items-start gap-2.5 font-medium">
              <AlertCircle className="w-4 h-4 mt-0.5 shrink-0 text-rose-500" />
              <span>{error}</span>
            </div>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-slate-700 font-bold mb-1.5">Contest Arena</label>
              <select
                value={arenaId}
                onChange={(e) => {
                  setArenaId(Number(e.target.value));
                  setSelectedAgentIds([]);
                }}
                className="w-full bg-slate-50 border border-slate-300 rounded-lg px-3 py-2 text-slate-900 focus:bg-white focus:outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100 transition"
              >
                {arenas.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-slate-700 font-bold mb-1.5">PRNG Match Seed</label>
              <div className="flex items-center gap-1.5">
                <input
                  type="number"
                  value={seed}
                  onChange={(e) => setSeed(Number(e.target.value))}
                  className="w-full font-mono bg-slate-50 border border-slate-300 rounded-lg px-3 py-2 text-slate-900 focus:bg-white focus:outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100 transition"
                />
                <button
                  type="button"
                  onClick={handleRandomSeed}
                  title="Generate Random Seed"
                  className="p-2 rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-700 border border-slate-300 transition shrink-0"
                >
                  <Shuffle className="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>

          {/* Participant Bots Selection */}
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="text-slate-700 font-bold">
                Select 5 Contenders ({selectedAgentIds.length}/5 selected)
              </label>
              <button
                type="button"
                onClick={handleAutoFill}
                className="text-blue-600 hover:text-blue-700 font-bold text-[11px] underline underline-offset-2 transition"
              >
                Auto-fill random 5
              </button>
            </div>

            <div className="border border-slate-200 rounded-xl bg-slate-50/60 max-h-48 overflow-y-auto divide-y divide-slate-100 p-1">
              {arenaAgents.length === 0 ? (
                <div className="p-6 text-center text-slate-400 font-medium">No active bots found in this arena</div>
              ) : (
                arenaAgents.map((agent, idx) => {
                  const isChecked = selectedAgentIds.includes(agent.id);
                  const balloonColor = BALLOON_COLORS[idx % BALLOON_COLORS.length];
                  return (
                    <div
                      key={agent.id}
                      onClick={() => handleToggleAgent(agent.id)}
                      className={`flex items-center justify-between px-3 py-2 rounded-lg cursor-pointer transition ${
                        isChecked ? 'bg-blue-100/70 text-blue-900' : 'hover:bg-slate-100 text-slate-700'
                      }`}
                    >
                      <div className="flex items-center gap-2.5">
                        <input
                          type="checkbox"
                          checked={isChecked}
                          onChange={() => {}}
                          className="rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                        />
                        <span
                          className={`w-5 h-5 rounded-full text-[10px] font-black flex items-center justify-center shadow-xs shrink-0 ${balloonColor}`}
                        >
                          {agent.name.charAt(0).toUpperCase()}
                        </span>
                        <span className="font-bold text-slate-900">{agent.name}</span>
                        <span className="text-slate-500 text-[10px] font-mono">({agent.team_name || 'Team'})</span>
                      </div>
                      <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded bg-white border border-slate-200 text-slate-600 font-semibold">
                        {agent.runtime}
                      </span>
                    </div>
                  );
                })
              )}
            </div>
          </div>

          {/* Footer Actions */}
          <div className="flex items-center justify-end gap-2.5 pt-3 border-t border-slate-100">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 rounded-lg bg-slate-100 hover:bg-slate-200 text-slate-700 font-semibold transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="inline-flex items-center gap-1.5 px-5 py-2 rounded-lg bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-700 hover:to-teal-700 disabled:opacity-50 text-white font-bold shadow-xs hover:shadow active:scale-[0.98] transition"
            >
              {loading ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  <span>Launching Simulation...</span>
                </>
              ) : (
                <>
                  <Play className="w-4 h-4 fill-current" />
                  <span>Launch Match</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
