import React, { useState } from 'react';
import { X, Play, Shuffle, CheckCircle2, AlertCircle, Loader2 } from 'lucide-react';
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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-lg shadow-xl w-full max-w-lg overflow-hidden animate-in fade-in zoom-in-95 duration-150">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-slate-800 bg-slate-950/60">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded bg-emerald-500/10 border border-emerald-500/30 flex items-center justify-center text-emerald-400">
              <Play className="w-4 h-4" />
            </div>
            <h3 className="text-sm font-semibold text-slate-100">Schedule & Launch Arena Match</h3>
          </div>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-200 p-1 rounded hover:bg-slate-800 transition"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="p-5 space-y-4 text-xs">
          {error && (
            <div className="p-3 rounded bg-red-950/80 border border-red-800/60 text-red-300 flex items-start gap-2">
              <AlertCircle className="w-4 h-4 mt-0.5 shrink-0 text-red-400" />
              <span>{error}</span>
            </div>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-slate-400 font-medium mb-1">Target Arena</label>
              <select
                value={arenaId}
                onChange={(e) => {
                  setArenaId(Number(e.target.value));
                  setSelectedAgentIds([]);
                }}
                className="w-full bg-slate-950 border border-slate-800 rounded px-2.5 py-2 text-slate-200 focus:outline-none focus:border-sky-500"
              >
                {arenas.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-slate-400 font-medium mb-1">Match Random Seed</label>
              <div className="flex items-center gap-1.5">
                <input
                  type="number"
                  value={seed}
                  onChange={(e) => setSeed(Number(e.target.value))}
                  className="w-full font-mono bg-slate-950 border border-slate-800 rounded px-3 py-2 text-slate-100 focus:outline-none focus:border-sky-500"
                />
                <button
                  type="button"
                  onClick={handleRandomSeed}
                  title="Generate New Seed"
                  className="p-2 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition"
                >
                  <Shuffle className="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>

          {/* Participant Bots Selection */}
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="text-slate-400 font-medium">
                Select 5 Opponents ({selectedAgentIds.length}/5 selected)
              </label>
              <button
                type="button"
                onClick={handleAutoFill}
                className="text-sky-400 hover:text-sky-300 font-medium transition"
              >
                Auto-fill random 5
              </button>
            </div>

            <div className="border border-slate-800 rounded-lg bg-slate-950/60 max-h-48 overflow-y-auto divide-y divide-slate-800/60 p-1">
              {arenaAgents.length === 0 ? (
                <div className="p-4 text-center text-slate-500">No active bots found in this arena</div>
              ) : (
                arenaAgents.map((agent) => {
                  const isChecked = selectedAgentIds.includes(agent.id);
                  return (
                    <div
                      key={agent.id}
                      onClick={() => handleToggleAgent(agent.id)}
                      className={`flex items-center justify-between px-3 py-2 rounded cursor-pointer transition ${
                        isChecked ? 'bg-sky-950/60 text-sky-200' : 'hover:bg-slate-900 text-slate-300'
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <input
                          type="checkbox"
                          checked={isChecked}
                          onChange={() => {}}
                          className="rounded border-slate-700 text-sky-500 focus:ring-0 focus:outline-none"
                        />
                        <span className="font-semibold">{agent.name}</span>
                        <span className="text-slate-500 text-[10px] font-mono">({agent.team_name || 'Team'})</span>
                      </div>
                      <span className="text-[10px] font-mono uppercase px-1.5 py-0.5 rounded bg-slate-800 text-slate-400">
                        {agent.runtime}
                      </span>
                    </div>
                  );
                })
              )}
            </div>
          </div>

          {/* Footer Actions */}
          <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-800">
            <button
              type="button"
              onClick={onClose}
              className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="inline-flex items-center gap-1.5 px-4 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white font-medium transition"
            >
              {loading ? (
                <>
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  <span>Launching Simulation...</span>
                </>
              ) : (
                <>
                  <Play className="w-3.5 h-3.5" />
                  <span>Start Match</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
