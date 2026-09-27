import React, { useState } from 'react';
import {
  Bot,
  UploadCloud,
  ShieldAlert,
  CheckCircle2,
  RefreshCw,
  FileCode,
  Hash,
  Terminal,
  Copy,
  Check,
  Cpu,
  Layers
} from 'lucide-react';
import { AgentVersion, Arena, User } from '../types';
import { StatusBadge } from '../components/StatusBadge';
import { api } from '../api/client';

interface BotsPageProps {
  agents: AgentVersion[];
  arenas: Arena[];
  user: User | null;
  loading: boolean;
  onRefresh: () => void;
  onOpenUpload: () => void;
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

export const BotsPage: React.FC<BotsPageProps> = ({
  agents,
  arenas,
  user,
  loading,
  onRefresh,
  onOpenUpload,
}) => {
  const [actionLoadingId, setActionLoadingId] = useState<number | null>(null);
  const [copiedHash, setCopiedHash] = useState<string | null>(null);

  const handleDisqualify = async (agentId: number) => {
    const reason = prompt('Enter reason for disqualification:', 'Protocol violation or runtime crash');
    if (!reason) return;

    setActionLoadingId(agentId);
    try {
      await api.disqualifyAgent(agentId, reason);
      onRefresh();
    } catch (err: any) {
      alert(err.message || 'Error disqualifying agent');
    } finally {
      setActionLoadingId(null);
    }
  };

  const handleEnable = async (agentId: number) => {
    setActionLoadingId(agentId);
    try {
      await api.enableAgent(agentId);
      onRefresh();
    } catch (err: any) {
      alert(err.message || 'Error enabling agent');
    } finally {
      setActionLoadingId(null);
    }
  };

  const handleCopySha = (sha: string) => {
    navigator.clipboard.writeText(sha);
    setCopiedHash(sha);
    setTimeout(() => setCopiedHash(null), 1800);
  };

  const getRuntimeBadge = (runtime: string) => {
    switch (runtime.toLowerCase()) {
      case 'python-standard':
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono font-semibold bg-amber-50 text-amber-800 border border-amber-200">
            <span className="w-1.5 h-1.5 rounded-full bg-amber-500"></span>
            Python Standard
          </span>
        );
      case 'python-onnx':
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono font-semibold bg-emerald-50 text-emerald-800 border border-emerald-200">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
            Python ONNX
          </span>
        );
      case 'binary':
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono font-semibold bg-indigo-50 text-indigo-800 border border-indigo-200">
            <span className="w-1.5 h-1.5 rounded-full bg-indigo-500"></span>
            ELF Binary
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-mono font-semibold bg-slate-100 text-slate-700 border border-slate-200">
            {runtime}
          </span>
        );
    }
  };

  const activeCount = agents.filter((a) => a.status === 'active').length;
  const dqCount = agents.filter((a) => a.status === 'disqualified').length;

  return (
    <div className="space-y-4 animate-fade-in">
      {/* Top Banner Bar */}
      <div className="bg-white border border-slate-200 rounded-xl p-4 sm:p-5 shadow-sm flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-blue-50 border border-blue-100 flex items-center justify-center text-blue-600 shadow-xs">
            <Bot className="w-5 h-5" />
          </div>
          <div>
            <h1 className="text-base font-black text-slate-900 tracking-tight uppercase">
              Autonomous Agents & Bot Submissions
            </h1>
            <p className="text-xs text-slate-500 font-medium">
              Registered algorithmic agents, sandboxed runtime environments, and validation states
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2.5">
          <button
            onClick={onRefresh}
            disabled={loading}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-white hover:bg-slate-50 text-slate-700 border border-slate-300 shadow-xs hover:shadow transition disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 text-slate-500 ${loading ? 'animate-spin' : ''}`} />
            <span>Refresh</span>
          </button>
          <button
            onClick={onOpenUpload}
            className="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg text-xs font-bold bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-700 hover:to-indigo-700 text-white shadow-xs hover:shadow active:scale-[0.98] transition"
          >
            <UploadCloud className="w-4 h-4" />
            <span>Upload New Bot</span>
          </button>
        </div>
      </div>

      {/* KPI Stats Strip */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <div className="bg-white border border-slate-200 rounded-xl p-3 shadow-xs">
          <div className="text-[10px] uppercase font-bold text-slate-400">Total Submissions</div>
          <div className="text-lg font-black text-slate-900 font-mono mt-0.5">{agents.length}</div>
        </div>
        <div className="bg-white border border-slate-200 rounded-xl p-3 shadow-xs">
          <div className="text-[10px] uppercase font-bold text-emerald-600">Active Roster</div>
          <div className="text-lg font-black text-emerald-700 font-mono mt-0.5">{activeCount}</div>
        </div>
        <div className="bg-white border border-slate-200 rounded-xl p-3 shadow-xs">
          <div className="text-[10px] uppercase font-bold text-rose-500">Disqualified</div>
          <div className="text-lg font-black text-rose-600 font-mono mt-0.5">{dqCount}</div>
        </div>
        <div className="bg-white border border-slate-200 rounded-xl p-3 shadow-xs">
          <div className="text-[10px] uppercase font-bold text-blue-600">Active Arenas</div>
          <div className="text-lg font-black text-blue-700 font-mono mt-0.5">{arenas.length}</div>
        </div>
      </div>

      {/* Bots High-Density Table */}
      <div className="bg-white border border-slate-200 rounded-xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="dj-table">
            <thead>
              <tr>
                <th className="w-16 text-center">ID</th>
                <th>Bot Name</th>
                <th>Team</th>
                <th>Runtime Spec</th>
                <th>Entrypoint</th>
                <th>SHA-256 Checksum</th>
                <th className="text-center">Status</th>
                <th className="text-right pr-4">Actions</th>
              </tr>
            </thead>
            <tbody>
              {agents.length === 0 ? (
                <tr>
                  <td colSpan={8} className="text-center py-12 text-slate-400 font-medium">
                    No agents uploaded yet. Click &ldquo;Upload New Bot&rdquo; to submit an agent package.
                  </td>
                </tr>
              ) : (
                agents.map((agent, idx) => {
                  const balloonColor = BALLOON_COLORS[idx % BALLOON_COLORS.length];
                  return (
                    <tr key={agent.id} className="hover:bg-blue-50/70 transition-colors">
                      <td className="text-center font-mono font-bold text-slate-500 text-xs">
                        #{agent.id}
                      </td>
                      <td>
                        <div className="flex items-center gap-2">
                          <span
                            className={`w-6 h-6 rounded-full text-xs font-black flex items-center justify-center shadow-xs shrink-0 ${balloonColor}`}
                          >
                            {agent.name.charAt(0).toUpperCase()}
                          </span>
                          <div>
                            <div className="flex items-center gap-1.5">
                              <span className="font-bold text-slate-900 text-xs">{agent.name}</span>
                              <span className="text-[10px] text-slate-500 font-mono bg-slate-100 px-1.5 py-0.2 rounded border border-slate-200">
                                v{agent.version}
                              </span>
                            </div>
                          </div>
                        </div>
                      </td>
                      <td className="text-slate-600 text-xs font-medium">
                        {agent.team_name || `Team #${agent.team_id}`}
                      </td>
                      <td>{getRuntimeBadge(agent.runtime)}</td>
                      <td>
                        <code className="text-[11px] font-mono bg-slate-100 text-slate-700 px-2 py-0.5 rounded border border-slate-200">
                          {agent.entrypoint}
                        </code>
                      </td>
                      <td>
                        {agent.sha256 ? (
                          <button
                            onClick={() => handleCopySha(agent.sha256)}
                            title="Click to copy full SHA-256"
                            className="group inline-flex items-center gap-1 font-mono text-[11px] text-slate-500 hover:text-blue-600 transition"
                          >
                            <span>{agent.sha256.substring(0, 10)}…</span>
                            {copiedHash === agent.sha256 ? (
                              <Check className="w-3 h-3 text-emerald-600" />
                            ) : (
                              <Copy className="w-3 h-3 opacity-0 group-hover:opacity-100 transition" />
                            )}
                          </button>
                        ) : (
                          <span className="text-slate-400 font-mono text-xs">-</span>
                        )}
                      </td>
                      <td className="text-center">
                        <StatusBadge status={agent.status} />
                      </td>
                      <td className="text-right pr-4">
                        {user?.role === 'admin' ? (
                          <div className="flex items-center justify-end gap-2">
                            {agent.status === 'active' ? (
                              <button
                                onClick={() => handleDisqualify(agent.id)}
                                disabled={actionLoadingId === agent.id}
                                className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] font-bold bg-rose-50 hover:bg-rose-100 text-rose-700 border border-rose-200 shadow-xs hover:shadow transition disabled:opacity-50"
                              >
                                <ShieldAlert className="w-3 h-3 text-rose-600" />
                                <span>Disqualify</span>
                              </button>
                            ) : (
                              <button
                                onClick={() => handleEnable(agent.id)}
                                disabled={actionLoadingId === agent.id}
                                className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] font-bold bg-emerald-50 hover:bg-emerald-100 text-emerald-700 border border-emerald-200 shadow-xs hover:shadow transition disabled:opacity-50"
                              >
                                <CheckCircle2 className="w-3 h-3 text-emerald-600" />
                                <span>Re-enable</span>
                              </button>
                            )}
                          </div>
                        ) : (
                          <span className="text-[11px] text-slate-400 italic">Protected</span>
                        )}
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
